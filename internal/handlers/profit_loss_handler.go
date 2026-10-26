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
// COGS is computed from each order item's product cost_price at query time
// (not a stored snapshot), so it reflects the product's current cost even
// for old orders - acceptable for a small business dashboard, but flagged
// via cost_price_coverage so finance staff know if the number is trustworthy.
func GetProfitLoss(c *gin.Context) {
	from, to := revenueDateRange(c)

	var grossRevenue float64
	database.DB.Table("orders").
		Where("created_at >= ? AND created_at < ?", from, to).
		Where("(payment_method = 'online' AND payment_status = 'paid' AND status NOT IN ('cancelled','returned')) OR (payment_method = 'cod' AND status = 'delivered')").
		Select("COALESCE(SUM(items_amount),0)").
		Scan(&grossRevenue)

	// COGS: sum of (order_item.quantity * product.cost_price) for items on
	// revenue-qualifying orders in range. Only counts items whose product
	// currently has cost_price > 0, so a partially-costed catalog doesn't
	// silently understate COGS to zero.
	var cogs float64
	database.DB.Table("order_items").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("JOIN products ON products.id = order_items.product_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where(profitLossOrdersFilter).
		Where("products.cost_price > 0").
		Select("COALESCE(SUM(order_items.quantity * products.cost_price),0)").
		Scan(&cogs)

	// Coverage: what fraction of sold *items* (by quantity) had a cost_price
	// set, so the UI can warn "COGS is incomplete" rather than imply it's exact.
	var itemsWithCost, itemsTotal int64
	database.DB.Table("order_items").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("JOIN products ON products.id = order_items.product_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where(profitLossOrdersFilter).
		Where("products.cost_price > 0").
		Select("COALESCE(SUM(order_items.quantity),0)").
		Scan(&itemsWithCost)

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

	grossProfit := grossRevenue - cogs

	var operatingExpenses float64
	database.DB.Table("expenses").
		Where("expense_date >= ? AND expense_date < ?", from, to).
		Where("approval_status IN ?", []string{"approved", "paid"}).
		Select("COALESCE(SUM(amount),0)").
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
