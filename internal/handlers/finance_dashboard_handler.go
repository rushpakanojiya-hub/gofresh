package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// FinanceDashboard godoc
// GET /api/v1/admin/finance/dashboard
// Single unified view (SRS 12.23): revenue, AP, AR, gateway pending, COD
// pending, vendor payable, expenses, GST, bank balance (from ledger trial
// balance), and a P&L snapshot for the current month - each figure pulled
// directly rather than round-tripping through the other finance handlers.
func FinanceDashboard(c *gin.Context) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var totalRevenue, cogs, totalExpenses float64
	revenueFilter := "(payment_method = 'online' AND payment_status = 'paid' AND status NOT IN ('cancelled','returned')) OR (payment_method = 'cod' AND status = 'delivered')"
	database.DB.Table("orders").
		Where("created_at >= ?", monthStart).
		Where(revenueFilter).
		Select("COALESCE(SUM(items_amount),0)").Scan(&totalRevenue)

	database.DB.Table("order_items").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("LEFT JOIN products ON products.id = order_items.product_id").
		Where("orders.created_at >= ?", monthStart).
		Where(revenueFilter).
		Select("COALESCE(SUM(order_items.quantity * products.cost_price),0)").Scan(&cogs)

	database.DB.Model(&models.Expense{}).
		Where("expense_date >= ? AND approval_status = ?", monthStart, "paid").
		Select("COALESCE(SUM(amount),0)").Scan(&totalExpenses)

	// Accounts Payable: outstanding vendor bills (not voided, not fully paid).
	var vendorPayable float64
	database.DB.Model(&models.VendorBill{}).
		Where("voided_at IS NULL").
		Select("COALESCE(SUM(amount + gst_amount - amount_paid),0)").Scan(&vendorPayable)

	// Accounts Receivable / gateway pending: online payments not yet paid.
	var gatewayPending float64
	database.DB.Table("payments").
		Joins("JOIN orders ON orders.id = payments.order_id").
		Where("orders.payment_method = ? AND payments.status IN ?", "online", []string{"created", "failed"}).
		Select("COALESCE(SUM(payments.amount),0)").Scan(&gatewayPending)

	// COD pending: delivered COD orders not yet marked paid.
	var codPending float64
	database.DB.Table("orders").
		Where("payment_method = ? AND status = ? AND payment_status != ?", "cod", "delivered", "paid").
		Select("COALESCE(SUM(total_amount),0)").Scan(&codPending)

	// GST this month (output GST from invoices).
	var outputGST float64
	database.DB.Table("invoices").
		Where("generated_at >= ?", monthStart).
		Select("COALESCE(SUM(cgst_amount + sgst_amount + igst_amount),0)").Scan(&outputGST)

	var vendorGST float64
	database.DB.Model(&models.VendorBill{}).
		Where("bill_date >= ? AND voided_at IS NULL", monthStart).
		Select("COALESCE(SUM(gst_amount),0)").Scan(&vendorGST)

	// Bank balance: net of all ledger entries against Bank (1002) and Cash (1001).
	var bankBalance float64
	database.DB.Table("ledger_entries").
		Joins("JOIN accounts ON accounts.id = ledger_entries.account_id").
		Where("accounts.code IN ?", []string{"1001", "1002"}).
		Select("COALESCE(SUM(CASE WHEN ledger_entries.type = 'debit' THEN ledger_entries.amount ELSE -ledger_entries.amount END),0)").
		Scan(&bankBalance)

	// Pending approvals count (maker-checker queue size, so the dashboard
	// surfaces work waiting on a second admin).
	var pendingExpenses, pendingJournalEntries, pendingBankChanges int64
	database.DB.Model(&models.Expense{}).Where("approval_status = ?", "submitted").Count(&pendingExpenses)
	database.DB.Model(&models.PendingJournalEntry{}).Where("status = ?", "pending").Count(&pendingJournalEntries)
	database.DB.Model(&models.VendorBankChangeRequest{}).Where("status = ?", "pending").Count(&pendingBankChanges)

	grossProfit := totalRevenue - cogs
	netProfit := grossProfit - totalExpenses

	c.JSON(http.StatusOK, gin.H{
		"period_start": monthStart,
		"revenue": gin.H{
			"total_revenue": totalRevenue,
			"cogs":          cogs,
			"gross_profit":  grossProfit,
			"expenses":      totalExpenses,
			"net_profit":    netProfit,
		},
		"accounts_payable": gin.H{
			"vendor_payable": vendorPayable,
		},
		"accounts_receivable": gin.H{
			"gateway_pending": gatewayPending,
			"cod_pending":     codPending,
		},
		"gst": gin.H{
			"output_gst":      outputGST,
			"vendor_gst":      vendorGST,
			"net_gst_payable": outputGST - vendorGST,
		},
		"bank_balance": bankBalance,
		"pending_approvals": gin.H{
			"expenses":        pendingExpenses,
			"journal_entries": pendingJournalEntries,
			"bank_changes":    pendingBankChanges,
		},
	})
}
