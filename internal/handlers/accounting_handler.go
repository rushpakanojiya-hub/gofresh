package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
)

// ---- Chart of Accounts ----

// ListAccounts godoc
// GET /api/v1/admin/finance/accounts?type=&is_active=
func ListAccounts(c *gin.Context) {
	db := database.DB.Model(&models.Account{})
	if t := c.Query("type"); t != "" {
		db = db.Where("type = ?", t)
	}
	if isActive := c.Query("is_active"); isActive != "" {
		db = db.Where("is_active = ?", isActive == "true")
	}

	var accounts []models.Account
	if err := db.Order("code ASC").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch accounts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

// CreateAccount godoc
// POST /api/v1/admin/finance/accounts
func CreateAccount(c *gin.Context) {
	var req models.AccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !models.ValidAccountTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid type - must be asset, liability, equity, revenue, or expense"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	account := models.Account{
		Code:     req.Code,
		Name:     req.Name,
		Type:     req.Type,
		IsActive: isActive,
	}
	if err := database.DB.Create(&account).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create account - code may already be in use"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_account", "account", fmt.Sprint(account.ID), "code: "+account.Code)

	c.JSON(http.StatusCreated, account)
}

// UpdateAccount godoc
// PUT /api/v1/admin/finance/accounts/:id
func UpdateAccount(c *gin.Context) {
	id := c.Param("id")
	var account models.Account
	if err := database.DB.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
		return
	}

	var req models.AccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !models.ValidAccountTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid type"})
		return
	}

	account.Code = req.Code
	account.Name = req.Name
	account.Type = req.Type
	if req.IsActive != nil {
		account.IsActive = *req.IsActive
	}

	if err := database.DB.Save(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update account"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "update_account", "account", id, "updated")

	c.JSON(http.StatusOK, account)
}

// ---- Ledger ----

// ListLedgerEntries godoc
// GET /api/v1/admin/finance/ledger?account_id=&from=&to=&page=&limit=
func ListLedgerEntries(c *gin.Context) {
	page := 1
	limit := 50
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	db := database.DB.Model(&models.LedgerEntry{}).Preload("Account")
	if accountID := c.Query("account_id"); accountID != "" {
		db = db.Where("account_id = ?", accountID)
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			db = db.Where("entry_date >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			db = db.Where("entry_date < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	db.Count(&total)

	var entries []models.LedgerEntry
	offset := (page - 1) * limit
	if err := db.Order("entry_date DESC, id DESC").Offset(offset).Limit(limit).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ledger entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries":     entries,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int((total + int64(limit) - 1) / int64(limit)),
	})
}

// CreateManualJournalEntry godoc
// POST /api/v1/admin/finance/ledger
// Records a balanced multi-line journal entry (double-entry bookkeeping):
// every line debits or credits one account, and total debits must equal
// total credits across all lines in the request - this is checked before
// anything is written, and the whole entry is created inside a single DB
// transaction so a partial/unbalanced entry can never land in the ledger.
// CreateManualJournalEntry godoc
// POST /api/v1/admin/finance/ledger
// Stages a balanced multi-line journal entry for approval (12.25
// maker-checker) rather than posting it directly - it only becomes real
// ledger rows once a different admin approves it via
// ApprovePendingJournalEntry.
func CreateManualJournalEntry(c *gin.Context) {
	var req models.ManualJournalEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := time.Parse("2006-01-02", req.EntryDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entry_date, use YYYY-MM-DD"})
		return
	}

	var totalDebit, totalCredit float64
	for _, line := range req.Lines {
		if !models.ValidLedgerEntryTypes[line.Type] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Each line's type must be debit or credit"})
			return
		}
		var account models.Account
		if err := database.DB.First(&account, line.AccountID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("account %d not found", line.AccountID)})
			return
		}
		if line.Type == "debit" {
			totalDebit += line.Amount
		} else {
			totalCredit += line.Amount
		}
	}
	if totalDebit != totalCredit {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "Entry does not balance: total debits must equal total credits",
			"total_debit":  totalDebit,
			"total_credit": totalCredit,
		})
		return
	}

	linesJSON, err := json.Marshal(req.Lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode journal entry lines"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	pending := models.PendingJournalEntry{
		EntryDate:     req.EntryDate,
		LinesJSON:     string(linesJSON),
		TotalAmount:   totalDebit,
		Status:        "pending",
		RequestedByID: adminID,
	}
	if err := database.DB.Create(&pending).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stage journal entry"})
		return
	}
	pending.Lines = req.Lines

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "submit_journal_entry", "pending_journal_entry", strconv.Itoa(int(pending.ID)), "pending")

	c.JSON(http.StatusCreated, pending)
}

// ListPendingJournalEntries godoc
// GET /api/v1/admin/finance/ledger/pending
func ListPendingJournalEntries(c *gin.Context) {
	var pendings []models.PendingJournalEntry
	db := database.DB.Order("created_at DESC")
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	if err := db.Find(&pendings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending journal entries"})
		return
	}
	for i := range pendings {
		var lines []models.LedgerEntryLine
		_ = json.Unmarshal([]byte(pendings[i].LinesJSON), &lines)
		pendings[i].Lines = lines
	}
	c.JSON(http.StatusOK, gin.H{"pending_journal_entries": pendings})
}

// ApprovePendingJournalEntry godoc
// POST /api/v1/admin/finance/ledger/pending/:id/approve
// Maker-checker (12.25): the approver must differ from the requester.
// Only on approval are the actual balanced LedgerEntry rows created.
func ApprovePendingJournalEntry(c *gin.Context) {
	id := c.Param("id")
	var pending models.PendingJournalEntry
	if err := database.DB.First(&pending, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pending journal entry not found"})
		return
	}
	if pending.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending entries can be approved"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	if adminID == pending.RequestedByID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Maker-checker: the requester cannot approve their own journal entry"})
		return
	}

	var lines []models.LedgerEntryLine
	if err := json.Unmarshal([]byte(pending.LinesJSON), &lines); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode staged journal entry lines"})
		return
	}

	entryDate, err := time.Parse("2006-01-02", pending.EntryDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid stored entry_date"})
		return
	}

	transactionRef := fmt.Sprintf("MJ-%d", time.Now().UnixNano())
	var created []models.LedgerEntry
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, line := range lines {
			var account models.Account
			if err := tx.First(&account, line.AccountID).Error; err != nil {
				return fmt.Errorf("account %d not found", line.AccountID)
			}
			entry := models.LedgerEntry{
				TransactionRef: transactionRef,
				AccountID:      line.AccountID,
				Type:           line.Type,
				Amount:         line.Amount,
				Description:    line.Description,
				ReferenceType:  "manual",
				EntryDate:      entryDate,
				CreatedByID:    &adminID,
			}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
			created = append(created, entry)
		}
		now := time.Now()
		pending.Status = "approved"
		pending.ApprovedByID = &adminID
		pending.ApprovedAt = &now
		return tx.Save(&pending).Error
	})
	if txErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": txErr.Error()})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "approve_journal_entry", "pending_journal_entry", id, "pending->approved, posted as "+transactionRef)

	var withAccounts []models.LedgerEntry
	database.DB.Preload("Account").Where("transaction_ref = ?", transactionRef).Order("id ASC").Find(&withAccounts)
	c.JSON(http.StatusCreated, gin.H{"transaction_ref": transactionRef, "entries": withAccounts})
}

// RejectPendingJournalEntry godoc
// POST /api/v1/admin/finance/ledger/pending/:id/reject
func RejectPendingJournalEntry(c *gin.Context) {
	id := c.Param("id")
	var pending models.PendingJournalEntry
	if err := database.DB.First(&pending, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pending journal entry not found"})
		return
	}
	if pending.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending entries can be rejected"})
		return
	}

	var req models.PendingJournalEntryRejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	if adminID == pending.RequestedByID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Maker-checker: the requester cannot reject their own journal entry"})
		return
	}

	pending.Status = "rejected"
	pending.RejectionReason = req.Reason
	if err := database.DB.Save(&pending).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject journal entry"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "reject_journal_entry", "pending_journal_entry", id, req.Reason)

	c.JSON(http.StatusOK, pending)
}

// GetTrialBalance godoc
// GET /api/v1/admin/finance/ledger/trial-balance?as_of=YYYY-MM-DD
// Sums all ledger entries up to (and including) as_of per account, giving
// the standard trial balance view: every account's net debit or credit
// position. Total debits and total credits across all accounts should
// always be equal - is_balanced surfaces that as an explicit sanity check
// rather than making the caller compute it.
func GetTrialBalance(c *gin.Context) {
	asOf := c.Query("as_of")
	cutoff := time.Now()
	if asOf != "" {
		if t, err := time.Parse("2006-01-02", asOf); err == nil {
			cutoff = t.AddDate(0, 0, 1)
		}
	}

	type row struct {
		AccountID   uint    `json:"account_id"`
		AccountCode string  `json:"account_code"`
		AccountName string  `json:"account_name"`
		AccountType string  `json:"account_type"`
		TotalDebit  float64 `json:"total_debit"`
		TotalCredit float64 `json:"total_credit"`
	}
	var rows []row
	database.DB.Table("ledger_entries").
		Joins("JOIN accounts ON accounts.id = ledger_entries.account_id").
		Where("ledger_entries.entry_date < ?", cutoff).
		Select(`accounts.id as account_id, accounts.code as account_code, accounts.name as account_name, accounts.type as account_type,
COALESCE(SUM(CASE WHEN ledger_entries.type = 'debit' THEN ledger_entries.amount ELSE 0 END),0) as total_debit,
COALESCE(SUM(CASE WHEN ledger_entries.type = 'credit' THEN ledger_entries.amount ELSE 0 END),0) as total_credit`).
		Group("accounts.id, accounts.code, accounts.name, accounts.type").
		Order("accounts.code ASC").
		Scan(&rows)

	var grandDebit, grandCredit float64
	for _, r := range rows {
		grandDebit += r.TotalDebit
		grandCredit += r.TotalCredit
	}

	c.JSON(http.StatusOK, gin.H{
		"as_of":        cutoff.AddDate(0, 0, -1).Format("2006-01-02"),
		"accounts":     rows,
		"total_debit":  grandDebit,
		"total_credit": grandCredit,
		"is_balanced":  grandDebit == grandCredit,
	})
}
