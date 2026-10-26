package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ListBankTransactions godoc
// GET /api/v1/admin/finance/bank-transactions?status=&from=&to=&page=&limit=
func ListBankTransactions(c *gin.Context) {
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	db := database.DB.Model(&models.BankTransaction{})
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			db = db.Where("transaction_date >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			db = db.Where("transaction_date < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	db.Count(&total)

	var unmatchedCount int64
	database.DB.Model(&models.BankTransaction{}).Where("status = ?", "unmatched").Count(&unmatchedCount)

	var transactions []models.BankTransaction
	offset := (page - 1) * limit
	if err := db.Order("transaction_date DESC").Offset(offset).Limit(limit).Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bank transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions":    transactions,
		"page":            page,
		"limit":           limit,
		"total":           total,
		"total_pages":     int((total + int64(limit) - 1) / int64(limit)),
		"unmatched_count": unmatchedCount,
	})
}

// CreateBankTransaction godoc
// POST /api/v1/admin/finance/bank-transactions
// Manually records one line from a bank statement. No CSV/auto-import yet -
// finance staff enter each line by hand, which is fine at this business's
// transaction volume and avoids building a bank-format parser prematurely.
func CreateBankTransaction(c *gin.Context) {
	var req models.BankTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txDate, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction_date, use YYYY-MM-DD"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	txn := models.BankTransaction{
		TransactionDate: txDate,
		Description:     req.Description,
		Amount:          req.Amount,
		ReferenceNumber: req.ReferenceNumber,
		Status:          "unmatched",
		CreatedByID:     adminID,
	}
	if err := database.DB.Create(&txn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bank transaction"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_bank_transaction", "bank_transaction", strconv.Itoa(int(txn.ID)), "created")

	c.JSON(http.StatusCreated, txn)
}

// MatchBankTransaction godoc
// POST /api/v1/admin/finance/bank-transactions/:id/match
// Marks a bank line as reconciled against an internal record. MatchedID is
// a loose reference (no FK) since matched_type can point at several
// different tables (vendor bill payments, payouts, etc) - validating it
// belongs to caller discipline rather than the DB schema here.
func MatchBankTransaction(c *gin.Context) {
	id := c.Param("id")
	var txn models.BankTransaction
	if err := database.DB.First(&txn, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bank transaction not found"})
		return
	}

	var req models.BankTransactionMatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	adminID := c.MustGet("user_id").(uint)

	txn.Status = "matched"
	txn.MatchedType = req.MatchedType
	txn.MatchedID = req.MatchedID
	txn.MatchedAt = &now
	txn.MatchedByID = &adminID
	txn.Note = req.Note

	if err := database.DB.Save(&txn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to match bank transaction"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "match_bank_transaction", "bank_transaction", id, "matched to "+req.MatchedType)

	c.JSON(http.StatusOK, txn)
}

// IgnoreBankTransaction godoc
// POST /api/v1/admin/finance/bank-transactions/:id/ignore
// For lines that will never have an internal match (bank fees, interest
// credited by the bank itself) - keeps them out of the "unmatched" count
// without pretending they're linked to some order/bill they aren't.
func IgnoreBankTransaction(c *gin.Context) {
	id := c.Param("id")
	var txn models.BankTransaction
	if err := database.DB.First(&txn, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bank transaction not found"})
		return
	}

	txn.Status = "ignored"
	if err := database.DB.Save(&txn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update bank transaction"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "ignore_bank_transaction", "bank_transaction", id, "ignored")

	c.JSON(http.StatusOK, txn)
}

// DeleteBankTransaction godoc
// DELETE /api/v1/admin/finance/bank-transactions/:id
// VoidBankTransaction godoc
// POST /api/v1/admin/finance/bank-transactions/:id/void
// Voids a bank transaction line instead of hard-deleting it, preserving
// the audit trail. If the line was matched to an internal record, voiding
// does not automatically unwind that match - unmatch first via a separate
// action if the match itself was wrong.
func VoidBankTransaction(c *gin.Context) {
	id := c.Param("id")
	var txn models.BankTransaction
	if err := database.DB.First(&txn, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bank transaction not found"})
		return
	}

	if txn.VoidedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bank transaction is already voided"})
		return
	}

	var req models.BankTransactionVoidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	now := time.Now()
	txn.VoidedAt = &now
	txn.VoidReason = req.Reason
	txn.VoidedByID = &adminID

	if err := database.DB.Save(&txn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to void bank transaction"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "void_bank_transaction", "bank_transaction", id, req.Reason)

	c.JSON(http.StatusOK, txn)
}
