package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// RequestVendorBankChange godoc
// POST /api/v1/admin/finance/vendors/:id/bank-change-request
func RequestVendorBankChange(c *gin.Context) {
	vendorID := c.Param("id")
	var vendor models.Vendor
	if err := database.DB.First(&vendor, vendorID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}

	var req models.VendorBankChangeRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	change := models.VendorBankChangeRequest{
		VendorID:         vendor.ID,
		NewAccountHolder: req.AccountHolder,
		NewAccountNumber: req.AccountNumber,
		NewIFSC:          req.IFSC,
		Status:           "pending",
		RequestedByID:    adminID,
	}
	if err := database.DB.Create(&change).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bank change request"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "request_vendor_bank_change", "vendor_bank_change_request", vendorID, "pending")

	c.JSON(http.StatusCreated, change)
}

// ListVendorBankChangeRequests godoc
// GET /api/v1/admin/finance/vendor-bank-change-requests
func ListVendorBankChangeRequests(c *gin.Context) {
	var changes []models.VendorBankChangeRequest
	db := database.DB.Order("created_at DESC")
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	if vendorID := c.Query("vendor_id"); vendorID != "" {
		db = db.Where("vendor_id = ?", vendorID)
	}
	if err := db.Find(&changes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bank change requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bank_change_requests": changes})
}

// ApproveVendorBankChange godoc
// POST /api/v1/admin/finance/vendor-bank-change-requests/:id/approve
// Maker-checker (12.28.5): the approver must differ from the requester.
// Only on approval does the vendor's bank details actually change.
func ApproveVendorBankChange(c *gin.Context) {
	id := c.Param("id")
	var change models.VendorBankChangeRequest
	if err := database.DB.First(&change, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bank change request not found"})
		return
	}
	if change.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending requests can be approved"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	if adminID == change.RequestedByID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Maker-checker: the requester cannot approve their own bank change request"})
		return
	}

	var vendor models.Vendor
	if err := database.DB.First(&vendor, change.VendorID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}

	vendor.BankAccountHolder = change.NewAccountHolder
	vendor.BankAccountNumber = change.NewAccountNumber
	vendor.BankIFSC = change.NewIFSC
	if err := database.DB.Save(&vendor).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vendor bank details"})
		return
	}

	now := time.Now()
	change.Status = "approved"
	change.ApprovedByID = &adminID
	change.ApprovedAt = &now
	if err := database.DB.Save(&change).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update bank change request"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "approve_vendor_bank_change", "vendor_bank_change_request", id, "pending->approved")

	c.JSON(http.StatusOK, gin.H{"vendor": vendor, "bank_change_request": change})
}

// RejectVendorBankChange godoc
// POST /api/v1/admin/finance/vendor-bank-change-requests/:id/reject
func RejectVendorBankChange(c *gin.Context) {
	id := c.Param("id")
	var change models.VendorBankChangeRequest
	if err := database.DB.First(&change, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bank change request not found"})
		return
	}
	if change.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending requests can be rejected"})
		return
	}

	var req models.VendorBankChangeRejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	if adminID == change.RequestedByID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Maker-checker: the requester cannot reject their own bank change request"})
		return
	}

	change.Status = "rejected"
	change.RejectionReason = req.Reason
	if err := database.DB.Save(&change).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject bank change request"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "reject_vendor_bank_change", "vendor_bank_change_request", id, req.Reason)

	c.JSON(http.StatusOK, change)
}
