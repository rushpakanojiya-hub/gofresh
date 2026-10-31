package handlers

import (
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
)

// GetGSTSummary godoc
// GET /api/v1/admin/finance/gst?from=YYYY-MM-DD&to=YYYY-MM-DD
// Aggregates GST already computed and stored on invoices (generated at
// order-confirmation/payment time) - this is output GST (tax collected on
// sales) only. There is no purchase-side GST tracked (no vendor bills /
// input tax credit records exist in the system), so "purchase_gst" and
// "input_tax_credit" are not included - a UI showing zeros for those would
// misrepresent the business as having no input credit, which isn't true,
// it's just not tracked yet.
func GetGSTSummary(c *gin.Context) {
	from, to := revenueDateRange(c)

	type totals struct {
		TaxableAmount float64
		CGSTAmount    float64
		SGSTAmount    float64
		IGSTAmount    float64
		InvoiceCount  int64
	}
	var t totals
	database.DB.Table("invoices").
		Where("generated_at >= ? AND generated_at < ? AND order_id IN (SELECT id FROM orders WHERE status NOT IN ('cancelled','returned'))", from, to).
		Select("COALESCE(SUM(taxable_amount),0) as taxable_amount, COALESCE(SUM(cgst_amount),0) as cgst_amount, COALESCE(SUM(sgst_amount),0) as sgst_amount, COALESCE(SUM(igst_amount),0) as igst_amount, COUNT(*) as invoice_count").
		Scan(&t)

	totalGST := t.CGSTAmount + t.SGSTAmount + t.IGSTAmount

	type hsnRow struct {
		HSNCode       string  `json:"hsn_code"`
		TaxableAmount float64 `json:"taxable_amount"`
		GSTAmount     float64 `json:"gst_amount"`
		Quantity      int64   `json:"quantity"`
	}
	byHSN := []hsnRow{}
	database.DB.Table("invoice_items").
		Joins("JOIN invoices ON invoices.id = invoice_items.invoice_id").
		Where("invoices.generated_at >= ? AND invoices.generated_at < ? AND invoices.order_id IN (SELECT id FROM orders WHERE status NOT IN ('cancelled','returned'))", from, to).
		Select("COALESCE(NULLIF(invoice_items.hsn_code, ''), 'Not set') as hsn_code, COALESCE(SUM(invoice_items.price * invoice_items.quantity * (CASE WHEN invoices.items_amount > 0 THEN GREATEST(invoices.items_amount - invoices.discount_amount, 0) / invoices.items_amount ELSE 1 END) - invoice_items.gst_amount),0) as taxable_amount, COALESCE(SUM(invoice_items.gst_amount),0) as gst_amount, COALESCE(SUM(invoice_items.quantity),0) as quantity").
		Group("COALESCE(NULLIF(invoice_items.hsn_code, ''), 'Not set')").
		Order("gst_amount DESC").
		Scan(&byHSN)

	type rateRow struct {
		GSTPercent    float64 `json:"gst_percent"`
		TaxableAmount float64 `json:"taxable_amount"`
		GSTAmount     float64 `json:"gst_amount"`
	}
	byRate := []rateRow{}
	database.DB.Table("invoice_items").
		Joins("JOIN invoices ON invoices.id = invoice_items.invoice_id").
		Where("invoices.generated_at >= ? AND invoices.generated_at < ? AND invoices.order_id IN (SELECT id FROM orders WHERE status NOT IN ('cancelled','returned'))", from, to).
		Select("invoice_items.gst_percent, COALESCE(SUM(invoice_items.price * invoice_items.quantity * (CASE WHEN invoices.items_amount > 0 THEN GREATEST(invoices.items_amount - invoices.discount_amount, 0) / invoices.items_amount ELSE 1 END) - invoice_items.gst_amount),0) as taxable_amount, COALESCE(SUM(invoice_items.gst_amount),0) as gst_amount").
		Group("invoice_items.gst_percent").
		Order("invoice_items.gst_percent ASC").
		Scan(&byRate)

	// BUG-05: delivery charge and platform fee GST sit in the invoice header but in no item row; report them as the reconciling remainder.
    var itemsTaxable, itemsGST float64
    for _, r := range byRate {
        itemsTaxable += r.TaxableAmount
        itemsGST += r.GSTAmount
    }
    otherCharges := gin.H{
        "taxable_amount": math.Round((t.TaxableAmount-itemsTaxable)*100) / 100,
        "gst_amount":     math.Round((totalGST-itemsGST)*100) / 100,
    }

	c.JSON(http.StatusOK, gin.H{
		"from":           from.Format("2006-01-02"),
		"to":             to.AddDate(0, 0, -1).Format("2006-01-02"),
		"taxable_amount": t.TaxableAmount,
		"cgst_amount":    t.CGSTAmount,
		"sgst_amount":    t.SGSTAmount,
		"igst_amount":    t.IGSTAmount,
		"total_gst":      totalGST,
		"invoice_count":  t.InvoiceCount,
		"by_hsn":         byHSN,
		"by_rate":        byRate,
		"other_charges":  otherCharges,
	})
}
