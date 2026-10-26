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
	"gorm.io/gorm/clause"
)

// ---- Vendors ----

// ListVendors godoc
// GET /api/v1/admin/finance/vendors?is_active=&page=&limit=
func ListVendors(c *gin.Context) {
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

	db := database.DB.Model(&models.Vendor{})
	if isActive := c.Query("is_active"); isActive != "" {
		db = db.Where("is_active = ?", isActive == "true")
	}

	var total int64
	db.Count(&total)

	var vendors []models.Vendor
	offset := (page - 1) * limit
	if err := db.Order("name ASC").Offset(offset).Limit(limit).Find(&vendors).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch vendors"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"vendors":     vendors,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int((total + int64(limit) - 1) / int64(limit)),
	})
}

// CreateVendor godoc
// POST /api/v1/admin/finance/vendors
func CreateVendor(c *gin.Context) {
	var req models.VendorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	vendor := models.Vendor{
		Name:        req.Name,
		ContactName: req.ContactName,
		Phone:       req.Phone,
		Email:       req.Email,
		GSTIN:       req.GSTIN,
		Address:     req.Address,
		IsActive:    isActive,
	}
	if err := database.DB.Create(&vendor).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vendor"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_vendor", "vendor", strconv.Itoa(int(vendor.ID)), "created")

	c.JSON(http.StatusCreated, vendor)
}

// UpdateVendor godoc
// PUT /api/v1/admin/finance/vendors/:id
func UpdateVendor(c *gin.Context) {
	id := c.Param("id")
	var vendor models.Vendor
	if err := database.DB.First(&vendor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}

	var req models.VendorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vendor.Name = req.Name
	vendor.ContactName = req.ContactName
	vendor.Phone = req.Phone
	vendor.Email = req.Email
	vendor.GSTIN = req.GSTIN
	vendor.Address = req.Address
	if req.IsActive != nil {
		vendor.IsActive = *req.IsActive
	}

	if err := database.DB.Save(&vendor).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vendor"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "update_vendor", "vendor", id, "updated")

	c.JSON(http.StatusOK, vendor)
}

// DeleteVendor godoc
// DELETE /api/v1/admin/finance/vendors/:id
func DeleteVendor(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&models.Vendor{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete vendor"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "delete_vendor", "vendor", id, "deleted")

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---- Vendor Bills (Accounts Payable) ----

type vendorBillResponse struct {
	models.VendorBill
	Status string `json:"status"`
}

func toVendorBillResponse(b models.VendorBill) vendorBillResponse {
	return vendorBillResponse{VendorBill: b, Status: models.VendorBillStatus(b)}
}

// ListVendorBills godoc
// GET /api/v1/admin/finance/vendor-bills?vendor_id=&status=&page=&limit=
func ListVendorBills(c *gin.Context) {
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

	db := database.DB.Model(&models.VendorBill{}).Preload("Vendor")
	if vendorID := c.Query("vendor_id"); vendorID != "" {
		db = db.Where("vendor_id = ?", vendorID)
	}

	var all []models.VendorBill
	if err := db.Order("bill_date DESC").Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch vendor bills"})
		return
	}

	// Status filter applied in-memory since it's a derived field, not a
	// stored column - the dataset here is small enough (finance/admin
	// tooling, not customer-facing) that this doesn't need to be pushed to SQL.
	statusFilter := c.Query("status")
	var filtered []models.VendorBill
	for _, b := range all {
		if statusFilter == "" || models.VendorBillStatus(b) == statusFilter {
			filtered = append(filtered, b)
		}
	}

	total := len(filtered)
	start := (page - 1) * limit
	end := start + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	page_slice := filtered[start:end]

	responses := make([]vendorBillResponse, len(page_slice))
	for i, b := range page_slice {
		responses[i] = toVendorBillResponse(b)
	}

	var totalOutstanding float64
	for _, b := range all {
		// BUG-08: voided bills are not outstanding.
		if b.VoidedAt != nil {
			continue
		}
		totalOutstanding += b.Amount - b.AmountPaid
	}

	c.JSON(http.StatusOK, gin.H{
		"bills":             responses,
		"page":              page,
		"limit":             limit,
		"total":             total,
		"total_pages":       (total + limit - 1) / limit,
		"total_outstanding": totalOutstanding,
	})
}

// CreateVendorBill godoc
// POST /api/v1/admin/finance/vendor-bills
func CreateVendorBill(c *gin.Context) {
	var req models.VendorBillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var vendor models.Vendor
	if err := database.DB.First(&vendor, req.VendorID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vendor not found"})
		return
	}

	billDate, err := time.Parse("2006-01-02", req.BillDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bill_date, use YYYY-MM-DD"})
		return
	}

	var dueDate *time.Time
	if req.DueDate != "" {
		d, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid due_date, use YYYY-MM-DD"})
			return
		}
		dueDate = &d
	}

	adminID := c.MustGet("user_id").(uint)
	bill := models.VendorBill{
		VendorID:    req.VendorID,
		BillNumber:  req.BillNumber,
		Amount:      req.Amount,
		GSTAmount:   req.GSTAmount,
		BillDate:    billDate,
		DueDate:     dueDate,
		Note:        req.Note,
		CreatedByID: adminID,
	}
	if err := database.DB.Create(&bill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vendor bill"})
		return
	}
	if err := services.PostVendorBillLedgerEntry(bill.ID); err != nil {
		log.Printf("failed to post vendor bill ledger entry for bill %d: %v", bill.ID, err)
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_vendor_bill", "vendor_bill", strconv.Itoa(int(bill.ID)), "created")

	database.DB.Preload("Vendor").First(&bill, bill.ID)
	c.JSON(http.StatusCreated, toVendorBillResponse(bill))
}

// PayVendorBill godoc
// POST /api/v1/admin/finance/vendor-bills/:id/pay
// Records a (possibly partial) payment against a bill. Rejects overpayment
// rather than silently clamping, since a payment amount typed wrong should
// surface as an error, not get quietly capped and hide the mistake.
func PayVendorBill(c *gin.Context) {
	id := c.Param("id")

	var req models.VendorBillPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bill models.VendorBill
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the row for the duration of this transaction so two concurrent
		// payments on the same bill can't both read the same AmountPaid and
		// both pass the remaining-balance check.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&bill, id).Error; err != nil {
			return err
		}

		// BUG-08: no payments against voided or on-hold bills.
		if bill.VoidedAt != nil || bill.HoldStatus != "" {
			return gorm.ErrInvalidField
		}

		// BUG-09: debit notes reduce what is still payable on the bill.
                var debited float64
                if err := tx.Model(&models.DebitNote{}).Where("vendor_bill_id = ?", bill.ID).Select("COALESCE(SUM(amount), 0)").Scan(&debited).Error; err != nil {
                        return err
                }
                remaining := bill.Amount - debited - bill.AmountPaid
		if req.Amount > remaining {
			return gorm.ErrInvalidData
		}

		bill.AmountPaid += req.Amount
		return tx.Save(&bill).Error
	})

	if txErr != nil {
		if txErr == gorm.ErrInvalidData {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Payment exceeds remaining balance of the bill"})
			return
		}
		if txErr == gorm.ErrInvalidField {
			c.JSON(http.StatusConflict, gin.H{"error": "Cannot pay a voided or on-hold bill"})
			return
		}
		if txErr == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Vendor bill not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record payment"})
		return
	}

	// Debit Vendor Payable, Credit Bank for this payment.
	if err := services.PostVendorPaymentLedgerEntry(bill.ID, req.Amount); err != nil {
		log.Printf("failed to post vendor payment ledger entry for bill %s: %v", id, err)
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "pay_vendor_bill", "vendor_bill", id, "paid")

	database.DB.Preload("Vendor").First(&bill, bill.ID)
	c.JSON(http.StatusOK, toVendorBillResponse(bill))
}

// DeleteVendorBill godoc
// DELETE /api/v1/admin/finance/vendor-bills/:id
// VoidVendorBill godoc
// POST /api/v1/admin/finance/vendor-bills/:id/void
// Voids a bill instead of hard-deleting it, preserving the audit trail.
// Blocked if any payment has already been recorded against the bill - per
// SRS 12.x, a paid/partially-paid bill must have its payment(s) reversed
// first (via a separate payment-reversal flow), not silently voided out
// from under recorded cash movement.
func VoidVendorBill(c *gin.Context) {
	id := c.Param("id")
	var bill models.VendorBill
	if err := database.DB.First(&bill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor bill not found"})
		return
	}

	if bill.VoidedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bill is already voided"})
		return
	}

	if bill.AmountPaid > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot void a bill with recorded payments - reverse the payment(s) first"})
		return
	}

	var req models.VendorBillVoidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	now := time.Now()
	bill.VoidedAt = &now
	bill.VoidReason = req.Reason
	bill.VoidedByID = &adminID

	if err := database.DB.Save(&bill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to void vendor bill"})
		return
	}

	// Reverse the ledger entries posted when this bill was created (Debit
	// Inventory/GST-ITC, Credit Vendor Payable) so voiding it doesn't leave
	// stale entries corrupting the general ledger.
	if err := services.ReverseVendorBillLedgerEntry(bill.ID); err != nil {
		log.Printf("failed to reverse vendor bill ledger entry for bill %s: %v", id, err)
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "void_vendor_bill", "vendor_bill", id, req.Reason)

	database.DB.Preload("Vendor").First(&bill, bill.ID)
	c.JSON(http.StatusOK, toVendorBillResponse(bill))
}

// HoldVendorBill godoc
// POST /api/v1/admin/finance/vendor-bills/:id/hold
// Puts the bill on hold, e.g. pending internal review before payment.
func HoldVendorBill(c *gin.Context) {
	setVendorBillHoldStatus(c, "on_hold", "hold_vendor_bill")
}

// DisputeVendorBill godoc
// POST /api/v1/admin/finance/vendor-bills/:id/dispute
// Marks the bill as disputed, e.g. amount or delivery is contested with
// the vendor. Distinct from hold so finance can see why payment paused.
func DisputeVendorBill(c *gin.Context) {
	setVendorBillHoldStatus(c, "disputed", "dispute_vendor_bill")
}

// ReleaseHoldVendorBill godoc
// POST /api/v1/admin/finance/vendor-bills/:id/release-hold
// Clears an on_hold or disputed status, returning the bill to its normal
// paid/partially_paid/unpaid status derived from AmountPaid.
func ReleaseHoldVendorBill(c *gin.Context) {
	id := c.Param("id")
	var bill models.VendorBill
	if err := database.DB.First(&bill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor bill not found"})
		return
	}

	if bill.HoldStatus == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bill is not currently on hold or disputed"})
		return
	}

	var req models.VendorBillHoldRequest
	_ = c.ShouldBindJSON(&req) // reason optional on release

	bill.HoldStatus = ""
	bill.HoldReason = ""

	if err := database.DB.Save(&bill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to release hold on vendor bill"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "release_hold_vendor_bill", "vendor_bill", id, "released")

	database.DB.Preload("Vendor").First(&bill, bill.ID)
	c.JSON(http.StatusOK, toVendorBillResponse(bill))
}

// setVendorBillHoldStatus is the shared implementation for HoldVendorBill
// and DisputeVendorBill - same request shape, same validation, only the
// resulting HoldStatus value and audit action differ.
func setVendorBillHoldStatus(c *gin.Context, holdStatus string, auditAction string) {
	id := c.Param("id")
	var bill models.VendorBill
	if err := database.DB.First(&bill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor bill not found"})
		return
	}

	if bill.VoidedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change hold status on a voided bill"})
		return
	}

	var req models.VendorBillHoldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bill.HoldStatus = holdStatus
	bill.HoldReason = req.Reason

	if err := database.DB.Save(&bill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vendor bill hold status"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, auditAction, "vendor_bill", id, req.Reason)

	database.DB.Preload("Vendor").First(&bill, bill.ID)
	c.JSON(http.StatusOK, toVendorBillResponse(bill))
}
