package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
)

// profitLossOrdersFilter is the same "did this order actually collect
// money" condition used by GetRevenueSummary, but pre-qualified with
// "orders." so it's safe to reuse inside joined queries (order_items JOIN
// orders JOIN products) without ambiguous-column errors.
const profitLossOrdersFilter = "(orders.payment_method = 'online' AND orders.payment_status = 'paid' AND orders.status NOT IN ('cancelled','returned')) OR (orders.payment_method = 'cod' AND orders.status = 'delivered')"

// GetProfitLoss godoc
// GET /api/v1/admin/finance/profit-loss?from=YYYY-MM-DD&to=YYYY-MM-DD
// Builds a P&L statement: Revenue - COGS = Gross Profit; Gross Profit -
// Operating Expenses = EBITDA (used here as a proxy for Net Profit, since
// this business has no depreciation/interest/tax line items tracked yet).
//
// COGS uses each order item's cost_price snapshot (taken at checkout or
// substitution), so later product cost edits do not rewrite history. Order
// items created before the snapshot existed (cost_price = 0) fall back to the
// product's current cost. cost_price_coverage flags how much of the sold
// quantity had any cost at all, so finance staff know if the number is trustworthy.
func GetProfitLoss(c *gin.Context) {
	from, to := revenueDateRange(c)

	var grossRevenue float64
	database.DB.Table("orders").
		Where("created_at >= ? AND created_at < ?", from, to).
		Where("(payment_method = 'online' AND payment_status = 'paid' AND status NOT IN ('cancelled','returned')) OR (payment_method = 'cod' AND status = 'delivered')").
		Select("COALESCE(SUM(items_amount),0)").
		Scan(&grossRevenue)

	// COGS: sum of (order_item.quantity * unit cost) for items on
// revenue-qualifying orders in range, where unit cost is the order item's
// snapshot (falling back to the product's cost for pre-snapshot rows). Items
// with no cost at all are skipped, so a partially-costed catalog doesn't
// silently understate COGS to zero.
	var cogs float64
if err := database.DB.Table("order_items").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("JOIN products ON products.id = order_items.product_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where(profitLossOrdersFilter).
		Where("COALESCE(NULLIF(order_items.cost_price,0), products.cost_price, 0) > 0").
		Select("COALESCE(SUM(order_items.quantity * COALESCE(NULLIF(order_items.cost_price,0), products.cost_price, 0)),0)").
		Scan(&cogs).Error; err != nil {
_ = c.Error(err)
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute COGS"})
return
}

	// Coverage: what fraction of sold *items* (by quantity) had a cost_price
	// set, so the UI can warn "COGS is incomplete" rather than imply it's exact.
	var itemsWithCost, itemsTotal int64
if err := database.DB.Table("order_items").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("JOIN products ON products.id = order_items.product_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where(profitLossOrdersFilter).
		Where("COALESCE(NULLIF(order_items.cost_price,0), products.cost_price, 0) > 0").
		Select("COALESCE(SUM(order_items.quantity),0)").
		Scan(&itemsWithCost).Error; err != nil {
_ = c.Error(err)
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute COGS coverage"})
return
}

	database.DB.Table("order_items").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where(profitLossOrdersFilter).
		Select("COALESCE(SUM(order_items.quantity),0)").
		Scan(&itemsTotal)

	costPriceCoverage := 0.0
	if itemsTotal > 0 {
		costPriceCoverage = float64(itemsWithCost) / float64(itemsTotal) * 100
	}

	// BUG-13: revenue and COGS come from the general ledger so the P&L matches the GL.
    var glRevenue, glCOGS float64
    database.DB.Table("ledger_entries").
        Joins("JOIN accounts ON accounts.id = ledger_entries.account_id").
        Where("ledger_entries.entry_date >= ? AND ledger_entries.entry_date < ? AND accounts.code = ?", from, to, "4001").
        Select("COALESCE(SUM(CASE WHEN ledger_entries.type = 'credit' THEN ledger_entries.amount ELSE -ledger_entries.amount END),0)").
        Scan(&glRevenue)
    database.DB.Table("ledger_entries").
        Joins("JOIN accounts ON accounts.id = ledger_entries.account_id").
        Where("ledger_entries.entry_date >= ? AND ledger_entries.entry_date < ? AND accounts.code = ?", from, to, "5001").
        Select("COALESCE(SUM(CASE WHEN ledger_entries.type = 'debit' THEN ledger_entries.amount ELSE -ledger_entries.amount END),0)").
        Scan(&glCOGS)
    grossRevenue = glRevenue
    cogs = glCOGS
    grossProfit := grossRevenue - cogs

	var operatingExpenses float64
	database.DB.Table("expenses").
		Where("expense_date >= ? AND expense_date < ?", from, to).
		Where("approval_status IN ?", []string{"approved", "paid"}).
		Select("COALESCE(SUM(amount),0)").
		Scan(&operatingExpenses)

	// BUG-13: operating expenses = ledger accounts 5002-5006 (discounts, opex incl. payroll, rider payouts, gateway fees, promo).
    database.DB.Table("ledger_entries").
        Joins("JOIN accounts ON accounts.id = ledger_entries.account_id").
        Where("ledger_entries.entry_date >= ? AND ledger_entries.entry_date < ? AND accounts.code IN ?", from, to, []string{"5002", "5003", "5004", "5005", "5006"}).
        Select("COALESCE(SUM(CASE WHEN ledger_entries.type = 'debit' THEN ledger_entries.amount ELSE -ledger_entries.amount END),0)").
        Scan(&operatingExpenses)
    ebitda := grossProfit - operatingExpenses

	c.JSON(http.StatusOK, gin.H{
		"from":                from.Format("2006-01-02"),
		"to":                  to.AddDate(0, 0, -1).Format("2006-01-02"),
		"revenue":             grossRevenue,
		"cogs":                cogs,
		"cost_price_coverage": costPriceCoverage,
		"gross_profit":        grossProfit,
		"operating_expenses":  operatingExpenses,
		"ebitda":              ebitda,
		"net_profit":          ebitda,
	})
}
