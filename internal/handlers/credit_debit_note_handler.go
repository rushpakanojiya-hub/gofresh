package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ListCreditNotes godoc
// GET /api/v1/admin/finance/credit-notes
func ListCreditNotes(c *gin.Context) {
	var notes []models.CreditNote
	db := database.DB.Order("issued_at DESC")
	if orderID := c.Query("order_id"); orderID != "" {
		db = db.Where("order_id = ?", orderID)
	}
	if err := db.Find(&notes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch credit notes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"credit_notes": notes})
}

// GetCreditNote godoc
// GET /api/v1/admin/finance/credit-notes/:id
func GetCreditNote(c *gin.Context) {
	id := c.Param("id")
	var note models.CreditNote
	if err := database.DB.Preload("Items").First(&note, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Credit note not found"})
		return
	}
	c.JSON(http.StatusOK, note)
}

// ListDebitNotes godoc
// GET /api/v1/admin/finance/debit-notes
func ListDebitNotes(c *gin.Context) {
	var notes []models.DebitNote
	db := database.DB.Order("issued_at DESC")
	if vendorID := c.Query("vendor_id"); vendorID != "" {
		db = db.Where("vendor_id = ?", vendorID)
	}
	if billID := c.Query("vendor_bill_id"); billID != "" {
		db = db.Where("vendor_bill_id = ?", billID)
	}
	if err := db.Find(&notes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch debit notes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"debit_notes": notes})
}

// GetDebitNote godoc
// GET /api/v1/admin/finance/debit-notes/:id
func GetDebitNote(c *gin.Context) {
	id := c.Param("id")
	var note models.DebitNote
	if err := database.DB.First(&note, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Debit note not found"})
		return
	}
	c.JSON(http.StatusOK, note)
}

// CreateDebitNote godoc
// POST /api/v1/admin/finance/vendor-bills/:id/debit-note
// Issues a Debit Note against a vendor bill for a purchase return, short
// supply, rate difference, or quality issue (SRS 12.18). Always an explicit
// admin action - unlike Credit Notes, there is no automatic trigger.
func CreateDebitNote(c *gin.Context) {
	billID := c.Param("id")

	var req models.DebitNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bill models.VendorBill
	if err := database.DB.First(&bill, billID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor bill not found"})
		return
	}
	if bill.VoidedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot issue a debit note against a voided bill"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	note, err := services.GenerateDebitNote(bill.ID, req.Amount, req.GSTAmount, req.Reason, adminID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create debit note: " + err.Error()})
		return
	}

	if err := services.PostDebitNoteLedgerEntry(note.ID); err != nil {
		log.Printf("failed to post debit note ledger entry for debit note %d: %v", note.ID, err)
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_debit_note", "debit_note", billID, req.Reason)

	c.JSON(http.StatusCreated, note)
}
