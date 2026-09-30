package handlers

import (
"log"
"net/http"
"strconv"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
"gorm.io/gorm"
)

// ListExpenses godoc
// GET /api/v1/admin/finance/expenses?category=&warehouse_id=&from=&to=&page=&limit=
func ListExpenses(c *gin.Context) {
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

db := database.DB.Model(&models.Expense{}).Preload("Warehouse")
if category := c.Query("category"); category != "" {
db = db.Where("category = ?", category)
}
if warehouseID := c.Query("warehouse_id"); warehouseID != "" {
db = db.Where("warehouse_id = ?", warehouseID)
}
if from := c.Query("from"); from != "" {
if t, err := time.Parse("2006-01-02", from); err == nil {
db = db.Where("expense_date >= ?", t)
}
}
if to := c.Query("to"); to != "" {
if t, err := time.Parse("2006-01-02", to); err == nil {
db = db.Where("expense_date < ?", t.AddDate(0, 0, 1))
}
}

var total int64
db.Count(&total)

var expenses []models.Expense
offset := (page - 1) * limit
if err := db.Order("expense_date DESC").Offset(offset).Limit(limit).Find(&expenses).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch expenses"})
return
}

var totalAmount float64
db.Session(&gorm.Session{}).Select("COALESCE(SUM(amount),0)").Scan(&totalAmount)

c.JSON(http.StatusOK, gin.H{
"expenses":     expenses,
"page":         page,
"limit":        limit,
"total":        total,
"total_pages":  int((total + int64(limit) - 1) / int64(limit)),
"total_amount": totalAmount,
})
}

// CreateExpense godoc
// POST /api/v1/admin/finance/expenses
func CreateExpense(c *gin.Context) {
var req models.ExpenseRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}
if !models.ValidExpenseCategories[req.Category] {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category"})
return
}
expenseDate, err := time.Parse("2006-01-02", req.ExpenseDate)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expense_date, use YYYY-MM-DD"})
return
}

adminID := c.MustGet("user_id").(uint)
expense := models.Expense{
Amount:      req.Amount,
Category:    req.Category,
ExpenseDate: expenseDate,
WarehouseID: req.WarehouseID,
Note:        req.Note,
ReceiptURL:  req.ReceiptURL,
AddedByID:   adminID,
ApprovalStatus: "draft",
}
if err := database.DB.Create(&expense).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create expense"})
return
}

adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "create_expense", "expense", strconv.Itoa(int(expense.ID)), "created")
c.JSON(http.StatusCreated, expense)
}

// UpdateExpense godoc
// PUT /api/v1/admin/finance/expenses/:id
func UpdateExpense(c *gin.Context) {
id := c.Param("id")
var expense models.Expense
if err := database.DB.First(&expense, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
return
}

var req models.ExpenseRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}
if !models.ValidExpenseCategories[req.Category] {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category"})
return
}
expenseDate, err := time.Parse("2006-01-02", req.ExpenseDate)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expense_date, use YYYY-MM-DD"})
return
}

previousAmount := expense.Amount
previousStatus := expense.ApprovalStatus

expense.Amount = req.Amount
expense.Category = req.Category
expense.ExpenseDate = expenseDate
expense.WarehouseID = req.WarehouseID
expense.Note = req.Note
expense.ReceiptURL = req.ReceiptURL

if err := database.DB.Save(&expense).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update expense"})
return
}

// If this expense was already paid (and therefore already posted to
// the general ledger), post an adjusting entry for the amount delta so
// the ledger doesn't go stale relative to the updated Expense record.
if previousStatus == "paid" && req.Amount != previousAmount {
if err := services.PostExpenseAdjustmentLedgerEntry(expense.ID, req.Amount-previousAmount); err != nil {
log.Printf("failed to post expense adjustment ledger entry for expense %d: %v", expense.ID, err)
}
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "update_expense", "expense", id, "updated")

c.JSON(http.StatusOK, expense)
}

// DeleteExpense godoc
// DELETE /api/v1/admin/finance/expenses/:id
func DeleteExpense(c *gin.Context) {
id := c.Param("id")
var expense models.Expense
if err := database.DB.First(&expense, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
return
}
if expense.ApprovalStatus == "paid" || expense.ApprovalStatus == "approved" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Paid or approved expenses cannot be deleted"})
return
}
if err := database.DB.Delete(&expense).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete expense"})
return
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "delete_expense", "expense", id, "deleted")

c.JSON(http.StatusOK, gin.H{"success": true})
}

// SubmitExpense godoc
// POST /api/v1/admin/finance/expenses/:id/submit
func SubmitExpense(c *gin.Context) {
id := c.Param("id")
var expense models.Expense
if err := database.DB.First(&expense, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
return
}
if expense.ApprovalStatus != "draft" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only draft expenses can be submitted"})
return
}
expense.ApprovalStatus = "submitted"
if err := database.DB.Save(&expense).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit expense"})
return
}
adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "submit_expense", "expense", id, "draft->submitted")
c.JSON(http.StatusOK, expense)
}

// ApproveExpense godoc
// POST /api/v1/admin/finance/expenses/:id/approve
// Maker-checker (12.13, 12.25): the approver must be a different admin
// than the one who created the expense.
func ApproveExpense(c *gin.Context) {
id := c.Param("id")
var expense models.Expense
if err := database.DB.First(&expense, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
return
}
if expense.ApprovalStatus != "submitted" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only submitted expenses can be approved"})
return
}
adminID := c.MustGet("user_id").(uint)
if adminID == expense.AddedByID {
c.JSON(http.StatusForbidden, gin.H{"error": "Maker-checker: the expense creator cannot approve their own expense"})
return
}
now := time.Now()
expense.ApprovalStatus = "approved"
expense.ApprovedByID = &adminID
expense.ApprovedAt = &now
if err := database.DB.Save(&expense).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve expense"})
return
}
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "approve_expense", "expense", id, "submitted->approved")
c.JSON(http.StatusOK, expense)
}

// RejectExpense godoc
// POST /api/v1/admin/finance/expenses/:id/reject
func RejectExpense(c *gin.Context) {
id := c.Param("id")
var expense models.Expense
if err := database.DB.First(&expense, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
return
}
if expense.ApprovalStatus != "submitted" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only submitted expenses can be rejected"})
return
}
var req models.ExpenseRejectRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}
adminID := c.MustGet("user_id").(uint)
if adminID == expense.AddedByID {
c.JSON(http.StatusForbidden, gin.H{"error": "Maker-checker: the expense creator cannot reject their own expense"})
return
}
expense.ApprovalStatus = "rejected"
expense.RejectionReason = req.Reason
if err := database.DB.Save(&expense).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject expense"})
return
}
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "reject_expense", "expense", id, req.Reason)
c.JSON(http.StatusOK, expense)
}

// PayExpense godoc
// POST /api/v1/admin/finance/expenses/:id/pay
// Only an approved expense can be paid. This is the point where the
// expense is actually recorded to the ledger (moved out of CreateExpense,
// which now only creates a draft - see 12.13/12.25 maker-checker workflow).
func PayExpense(c *gin.Context) {
id := c.Param("id")
var expense models.Expense
if err := database.DB.First(&expense, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
return
}
if expense.ApprovalStatus != "approved" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only approved expenses can be paid"})
return
}
now := time.Now()
expense.ApprovalStatus = "paid"
expense.PaidAt = &now
if err := database.DB.Save(&expense).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark expense paid"})
return
}
if err := services.PostExpenseLedgerEntry(expense.ID); err != nil {
log.Printf("failed to post expense ledger entry for expense %d: %v", expense.ID, err)
}
adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "pay_expense", "expense", id, "approved->paid")
c.JSON(http.StatusOK, expense)
}
