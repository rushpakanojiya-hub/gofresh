package handlers

import (
"fmt"
"net/http"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ---- Rider COD Deposits (SRS 12.9) ----

// CreateRiderCODDeposit godoc
// POST /api/v1/admin/finance/rider-cod-deposits
func CreateRiderCODDeposit(c *gin.Context) {
var req models.RiderCODDepositRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}
depositDate, err := time.Parse("2006-01-02", req.DepositDate)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deposit_date, use YYYY-MM-DD"})
return
}
adminID := c.MustGet("user_id").(uint)
deposit := models.RiderCODDeposit{
DeliveryPartnerID: req.DeliveryPartnerID,
Amount:            req.Amount,
DepositDate:       depositDate,
Status:            "pending",
Note:              req.Note,
CreatedByID:       adminID,
}
if err := database.DB.Create(&deposit).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record COD deposit"})
return
}
services.CreateDeliveryNotification(
deposit.DeliveryPartnerID,
"Cash deposit recorded",
fmt.Sprintf("Your cash deposit of Rs.%.2f has been recorded and is pending verification.", deposit.Amount),
"cod_deposit_recorded",
nil,
)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "create_rider_cod_deposit", "rider_cod_deposit", "", "pending")
c.JSON(http.StatusCreated, deposit)
}

// ListRiderCODDeposits godoc
// GET /api/v1/admin/finance/rider-cod-deposits
func ListRiderCODDeposits(c *gin.Context) {
var deposits []models.RiderCODDeposit
db := database.DB.Order("created_at DESC")
if status := c.Query("status"); status != "" {
db = db.Where("status = ?", status)
}
if partnerID := c.Query("delivery_partner_id"); partnerID != "" {
db = db.Where("delivery_partner_id = ?", partnerID)
}
db.Find(&deposits)
c.JSON(http.StatusOK, gin.H{"rider_cod_deposits": deposits})
}

// VerifyRiderCODDeposit godoc
// POST /api/v1/admin/finance/rider-cod-deposits/:id/verify
func VerifyRiderCODDeposit(c *gin.Context) {
id := c.Param("id")
var deposit models.RiderCODDeposit
if err := database.DB.First(&deposit, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "COD deposit not found"})
return
}
if deposit.Status != "pending" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending deposits can be verified"})
return
}
adminID := c.MustGet("user_id").(uint)
now := time.Now()
deposit.Status = "verified"
deposit.VerifiedByID = &adminID
deposit.VerifiedAt = &now
if err := database.DB.Save(&deposit).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify deposit"})
return
}
if err := services.PostRiderCODDepositLedgerEntry(deposit.ID); err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Deposit verified but ledger posting failed: " + err.Error()})
return
}
services.CreateDeliveryNotification(
deposit.DeliveryPartnerID,
"COD settlement verified",
fmt.Sprintf("Your cash deposit of Rs.%.2f has been verified.", deposit.Amount),
"cod_settlement_verified",
nil,
)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "verify_rider_cod_deposit", "rider_cod_deposit", id, "verified")
c.JSON(http.StatusOK, deposit)
}

// ---- Rider Payouts (SRS 12.11) ----

const perDeliveryEarningRate = 30.0

// CreateRiderPayout godoc
// POST /api/v1/admin/finance/rider-payouts
// Computes the payout amount from delivered-order count in the period,
// same rate used by the Rider Payable report.
func CreateRiderPayout(c *gin.Context) {
var req models.RiderPayoutRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}
periodFrom, err := time.Parse("2006-01-02", req.PeriodFrom)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid period_from, use YYYY-MM-DD"})
return
}
periodTo, err := time.Parse("2006-01-02", req.PeriodTo)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid period_to, use YYYY-MM-DD"})
return
}
periodToExclusive := periodTo.AddDate(0, 0, 1)

// Block duplicate payouts: if any payout already exists for this rider
// whose period overlaps the requested range, refuse rather than create a
// second one that would double-count and double-accrue the same deliveries.
var overlapCount int64
database.DB.Model(&models.RiderPayout{}).
Where("delivery_partner_id = ? AND period_from < ? AND period_to >= ?",
req.DeliveryPartnerID, periodToExclusive, periodFrom).
Count(&overlapCount)
if overlapCount > 0 {
c.JSON(http.StatusConflict, gin.H{"error": "A payout already exists for this rider covering an overlapping period"})
return
}

var deliveredCount int64
database.DB.Model(&models.Order{}).
Where("delivery_partner_id = ? AND status = ? AND COALESCE(delivered_at, updated_at) >= ? AND COALESCE(delivered_at, updated_at) < ?",
req.DeliveryPartnerID, "delivered", periodFrom, periodToExclusive).
Count(&deliveredCount)

adminID := c.MustGet("user_id").(uint)
payout := models.RiderPayout{
DeliveryPartnerID: req.DeliveryPartnerID,
PeriodFrom:        periodFrom,
PeriodTo:          periodTo,
DeliveredCount:    int(deliveredCount),
Amount:            float64(deliveredCount) * perDeliveryEarningRate,
Status:            "pending",
CreatedByID:       adminID,
}
if err := database.DB.Create(&payout).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rider payout"})
return
}
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "create_rider_payout", "rider_payout", "", "pending")
c.JSON(http.StatusCreated, payout)
}

// ListRiderPayouts godoc
// GET /api/v1/admin/finance/rider-payouts
func ListRiderPayouts(c *gin.Context) {
var payouts []models.RiderPayout
db := database.DB.Order("created_at DESC")
if status := c.Query("status"); status != "" {
db = db.Where("status = ?", status)
}
if partnerID := c.Query("delivery_partner_id"); partnerID != "" {
db = db.Where("delivery_partner_id = ?", partnerID)
}
db.Find(&payouts)
c.JSON(http.StatusOK, gin.H{"rider_payouts": payouts})
}

// ApproveRiderPayout godoc
// POST /api/v1/admin/finance/rider-payouts/:id/approve
// Accrues the payout to the ledger (Debit Rider Delivery Expense, Credit
// Rider Payable) - the amount is now formally recognized as owed.
func ApproveRiderPayout(c *gin.Context) {
id := c.Param("id")
var payout models.RiderPayout
if err := database.DB.First(&payout, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Rider payout not found"})
return
}
if payout.Status != "pending" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending payouts can be approved"})
return
}
adminID := c.MustGet("user_id").(uint)
now := time.Now()
payout.Status = "approved"
payout.ApprovedByID = &adminID
payout.ApprovedAt = &now
if err := database.DB.Save(&payout).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve rider payout"})
return
}
if err := services.PostRiderPayoutAccrualLedgerEntry(payout.ID); err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Payout approved but ledger posting failed: " + err.Error()})
return
}
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "approve_rider_payout", "rider_payout", id, "pending->approved")
c.JSON(http.StatusOK, payout)
}

// PayRiderPayout godoc
// POST /api/v1/admin/finance/rider-payouts/:id/pay
// Settles an approved payout (Debit Rider Payable, Credit Bank) - the
// actual money-out step.
func PayRiderPayout(c *gin.Context) {
id := c.Param("id")
var payout models.RiderPayout
if err := database.DB.First(&payout, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Rider payout not found"})
return
}
if payout.Status != "approved" {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only approved payouts can be paid"})
return
}
if err := services.PostRiderPayoutSettlementLedgerEntry(payout.ID); err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post ledger entry: " + err.Error()})
return
}
now := time.Now()
payout.Status = "paid"
payout.PaidAt = &now
if err := database.DB.Save(&payout).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark payout paid"})
return
}
if false {
// ledger posting already performed before save
return
}
adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "pay_rider_payout", "rider_payout", id, "approved->paid")
c.JSON(http.StatusOK, payout)
}
