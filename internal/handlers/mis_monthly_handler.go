package handlers

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/xuri/excelize/v2"
)

type monthlyMisRow struct {
	Label    string  `json:"label"`
	Current  float64 `json:"current"`
	Previous float64 `json:"previous"`
	YTD      float64 `json:"ytd"`
	Budget   float64 `json:"budget"`
	Variance float64 `json:"variance"`
}

func monthBounds(dateStr string) (time.Time, time.Time, time.Time, time.Time, time.Time) {
	var t time.Time
	if dateStr != "" {
		if parsed, err := time.Parse("2006-01", dateStr); err == nil {
			t = parsed
		}
	}
	if t.IsZero() {
		t = time.Now()
	}
	monthStart := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)
	prevStart := monthStart.AddDate(0, -1, 0)
	prevEnd := monthStart.Add(-time.Second)
	ytdStart := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	return monthStart, monthEnd, prevStart, prevEnd, ytdStart
}

func getMonthlyBudget(monthKey, rowKey string) float64 {
	for _, e := range getManualEntries("monthly_budget", monthKey) {
		if e.RowKey == rowKey {
			if v, ok := e.Data["budget"].(float64); ok {
				return v
			}
		}
	}
	return 0
}

func mkRow(label string, cur, prev, ytd, budget float64) monthlyMisRow {
	return monthlyMisRow{
		Label: label, Current: round2(cur), Previous: round2(prev), YTD: round2(ytd),
		Budget: round2(budget), Variance: round2(cur - budget),
	}
}

type monthlyAgg struct {
	orders, cancelledOrders, customers, newCustomers float64
	gmv, productSales, delivery, discounts, refunds  float64
	platformFee float64
	cancellationsAmt, cogs, deliveryCost, wastage    float64
}

func calcMonthlyAgg(s, e time.Time) monthlyAgg {
	var a monthlyAgg
	database.DB.Raw(`SELECT COUNT(*) FROM orders WHERE created_at BETWEEN ? AND ?`, s, e).Row().Scan(&a.orders)
	database.DB.Raw(`SELECT COUNT(*) FROM orders WHERE created_at BETWEEN ? AND ? AND status = 'cancelled'`, s, e).Row().Scan(&a.cancelledOrders)
	database.DB.Raw(`SELECT COUNT(DISTINCT user_id) FROM orders WHERE created_at BETWEEN ? AND ?`, s, e).Row().Scan(&a.customers)
	database.DB.Raw(`SELECT COUNT(*) FROM users WHERE role = 'customer' AND created_at BETWEEN ? AND ?`, s, e).Row().Scan(&a.newCustomers)
	database.DB.Raw(`SELECT COALESCE(SUM(items_amount),0) FROM orders WHERE created_at BETWEEN ? AND ?`, s, e).Row().Scan(&a.gmv)
	database.DB.Raw(`SELECT COALESCE(SUM(items_amount),0) FROM orders WHERE created_at BETWEEN ? AND ? AND status <> 'cancelled'`, s, e).Row().Scan(&a.productSales)
	database.DB.Raw(`SELECT COALESCE(SUM(delivery_charge),0) FROM orders WHERE created_at BETWEEN ? AND ? AND status <> 'cancelled'`, s, e).Row().Scan(&a.delivery)
	database.DB.Raw(`SELECT COALESCE(SUM(platform_fee),0) FROM orders WHERE created_at BETWEEN ? AND ? AND status <> 'cancelled'`, s, e).Row().Scan(&a.platformFee)
	database.DB.Raw(`SELECT COALESCE(SUM(oc.discount_amount),0) FROM order_coupons oc JOIN orders o ON o.id = oc.order_id WHERE o.created_at BETWEEN ? AND ?`, s, e).Row().Scan(&a.discounts)
	database.DB.Raw(`SELECT COALESCE(SUM(total_amount),0) FROM credit_notes WHERE issued_at BETWEEN ? AND ?`, s, e).Row().Scan(&a.refunds)
	database.DB.Raw(`SELECT COALESCE(SUM(items_amount),0) FROM orders WHERE created_at BETWEEN ? AND ? AND status = 'cancelled'`, s, e).Row().Scan(&a.cancellationsAmt)
	database.DB.Raw(`SELECT COALESCE(SUM(oi.quantity * p.cost_price),0) FROM order_items oi JOIN orders o ON o.id = oi.order_id JOIN products p ON p.id = oi.product_id WHERE o.created_at BETWEEN ? AND ? AND o.status <> 'cancelled'`, s, e).Row().Scan(&a.cogs)
	database.DB.Raw(`SELECT COALESCE(SUM(amount),0) FROM expenses WHERE expense_date BETWEEN ? AND ? AND approval_status <> 'rejected' AND LOWER(TRIM(category)) = 'delivery'`, s, e).Row().Scan(&a.deliveryCost)
	database.DB.Raw(`SELECT COALESCE(SUM(amount),0) FROM expenses WHERE expense_date BETWEEN ? AND ? AND approval_status <> 'rejected' AND LOWER(TRIM(category)) IN ('wastage','expiry')`, s, e).Row().Scan(&a.wastage)
	return a
}

type categorySales struct {
	Name  string
	Sales float64
}

func topCategorySales(s, e time.Time) []categorySales {
	rows := []categorySales{}
	database.DB.Raw(`
SELECT c.name as name, COALESCE(SUM(oi.quantity * oi.price),0) as sales
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN products p ON p.id = oi.product_id
JOIN categories c ON c.id = p.category_id
WHERE o.created_at BETWEEN ? AND ? AND o.status <> 'cancelled'
GROUP BY c.name ORDER BY sales DESC
`, s, e).Scan(&rows)
	return rows
}

func computeGroceryMonthlyMIS(monthKey string, start, end, prevStart, prevEnd, ytdStart time.Time) []monthlyMisRow {
	cur := calcMonthlyAgg(start, end)
	prev := calcMonthlyAgg(prevStart, prevEnd)
	ytd := calcMonthlyAgg(ytdStart, end)

	netRev := func(a monthlyAgg) float64 {
		return a.productSales + a.delivery + a.platformFee - a.discounts - a.refunds
	}
	netRevCur, netRevPrev, netRevYTD := netRev(cur), netRev(prev), netRev(ytd)
	gpCur, gpPrev, gpYTD := netRevCur-cur.cogs, netRevPrev-prev.cogs, netRevYTD-ytd.cogs

	marginPct := func(gp, nr float64) float64 {
		if nr == 0 {
			return 0
		}
		return gp / nr * 100
	}
	avgBasket := func(a monthlyAgg) float64 {
		if a.orders == 0 {
			return 0
		}
		return a.gmv / a.orders
	}
	fillRate := func(a monthlyAgg) float64 {
		if a.orders == 0 {
			return 0
		}
		return (a.orders - a.cancelledOrders) / a.orders * 100
	}

	var invValue float64
	database.DB.Raw(`SELECT COALESCE(SUM(i.stock * p.cost_price),0) FROM inventories i JOIN products p ON p.id = i.product_id`).Row().Scan(&invValue)
	stockTurnover := 0.0
	if invValue != 0 {
		stockTurnover = cur.cogs / invValue
	}

	out := []monthlyMisRow{}
	add := func(label string, c, p, y float64) {
		out = append(out, mkRow(label, c, p, y, getMonthlyBudget(monthKey, label)))
	}
	add("Orders", cur.orders, prev.orders, ytd.orders)
	add("Customers", cur.customers, prev.customers, ytd.customers)
	add("New Customers", cur.newCustomers, prev.newCustomers, ytd.newCustomers)
	add("Average Basket Value", avgBasket(cur), avgBasket(prev), avgBasket(ytd))
	add("GMV", cur.gmv, prev.gmv, ytd.gmv)
	add("Grocery Sales", cur.productSales, prev.productSales, ytd.productSales)

	topCats := topCategorySales(start, end)
	otherCur := cur.productSales
	for i, cat := range topCats {
		if i >= 3 {
			break
		}
		add(cat.Name+" Sales", cat.Sales, 0, 0)
		otherCur -= cat.Sales
	}
	add("Other Category Sales", otherCur, 0, 0)

	add("Delivery Income", cur.delivery, prev.delivery, ytd.delivery)
	add("Platform / Convenience Fee", cur.platformFee, prev.platformFee, ytd.platformFee)
	add("Discounts", cur.discounts, prev.discounts, ytd.discounts)
	add("Refunds / Returns", cur.refunds, prev.refunds, ytd.refunds)
	add("Net Revenue", netRevCur, netRevPrev, netRevYTD)
	add("Cost of Goods Sold", cur.cogs, prev.cogs, ytd.cogs)
	add("Gross Profit", gpCur, gpPrev, gpYTD)
	add("Gross Margin %", marginPct(gpCur, netRevCur), marginPct(gpPrev, netRevPrev), marginPct(gpYTD, netRevYTD))
	add("Delivery Cost", cur.deliveryCost, prev.deliveryCost, ytd.deliveryCost)
	add("Wastage / Spoilage", cur.wastage, prev.wastage, ytd.wastage)
	// BUG-16: only the live snapshot is known; no historical cost snapshot exists, so prior columns are 0, not a copy.
	add("Inventory Value", invValue, 0, 0)
	add("Stock Turnover", stockTurnover, 0, 0)
	add("On-time Delivery %", getMonthlyBudget(monthKey, "On-time Delivery % (manual)"), 0, 0)
	add("Order Fill Rate %", fillRate(cur), fillRate(prev), fillRate(ytd))
	return out
}

func computeDashboardMonthlyMIS(monthKey string, start, end, prevStart, prevEnd, ytdStart time.Time) []monthlyMisRow {
	cur := calcMonthlyAgg(start, end)
	prev := calcMonthlyAgg(prevStart, prevEnd)
	ytd := calcMonthlyAgg(ytdStart, end)

	netRev := func(a monthlyAgg) float64 {
		return a.productSales + a.delivery + a.platformFee - a.discounts - a.refunds
	}
	netRevCur, netRevPrev, netRevYTD := netRev(cur), netRev(prev), netRev(ytd)
	gpCur, gpPrev, gpYTD := netRevCur-cur.cogs, netRevPrev-prev.cogs, netRevYTD-ytd.cogs

	totalExpense := func(s, e time.Time) float64 {
		var v float64
		database.DB.Raw(`SELECT COALESCE(SUM(amount),0) FROM expenses WHERE expense_date BETWEEN ? AND ? AND approval_status <> 'rejected'`, s, e).Row().Scan(&v)
		return v
	}
	opexCur, opexPrev, opexYTD := totalExpense(start, end), totalExpense(prevStart, prevEnd), totalExpense(ytdStart, end)
	ebitdaCur, ebitdaPrev, ebitdaYTD := gpCur-opexCur, gpPrev-opexPrev, gpYTD-opexYTD
	ebitdaPct := func(eb, nr float64) float64 {
		if nr == 0 {
			return 0
		}
		return eb / nr * 100
	}

	var totalCustomers float64
	database.DB.Raw(`SELECT COUNT(*) FROM users WHERE role = 'customer' AND created_at <= ?`, end).Row().Scan(&totalCustomers)

	out := []monthlyMisRow{}
	add := func(label string, c, p, y float64) {
		out = append(out, mkRow(label, c, p, y, getMonthlyBudget(monthKey, label)))
	}
	add("Total Customers / Users", totalCustomers, 0, 0)
	add("New Customers / Users", cur.newCustomers, prev.newCustomers, ytd.newCustomers)
	add("Active Users", cur.customers, prev.customers, ytd.customers)
	add("Total Orders / Transactions", cur.orders, prev.orders, ytd.orders)
	if cur.orders != 0 {
		add("Gross Merchandise / Transaction", cur.gmv/cur.orders, 0, 0)
	} else {
		add("Gross Merchandise / Transaction", 0, 0, 0)
	}
	add("Total Revenue", cur.gmv, prev.gmv, ytd.gmv)
	add("Refunds / Cancellations", cur.refunds+cur.cancellationsAmt, prev.refunds+prev.cancellationsAmt, ytd.refunds+ytd.cancellationsAmt)
	add("Net Revenue", netRevCur, netRevPrev, netRevYTD)
	add("Gross Profit", gpCur, gpPrev, gpYTD)
	add("Operating Expenses", opexCur, opexPrev, opexYTD)
	add("EBITDA", ebitdaCur, ebitdaPrev, ebitdaYTD)
	add("EBITDA %", ebitdaPct(ebitdaCur, netRevCur), ebitdaPct(ebitdaPrev, netRevPrev), ebitdaPct(ebitdaYTD, netRevYTD))
	add("Cash & Bank Balance", getMonthlyBudget(monthKey, "Cash & Bank Balance (manual)"), 0, 0)
	add("Tax / Statutory Dues", getMonthlyBudget(monthKey, "Tax / Statutory Dues (manual)"), 0, 0)
	return out
}

func GetMonthlyMIS(c *gin.Context) {
	monthStart, monthEnd, prevStart, prevEnd, ytdStart := monthBounds(c.Query("month"))
	monthKey := monthStart.Format("2006-01")
	c.JSON(http.StatusOK, gin.H{
		"month":     monthKey,
		"dashboard": computeDashboardMonthlyMIS(monthKey, monthStart, monthEnd, prevStart, prevEnd, ytdStart),
		"grocery":   computeGroceryMonthlyMIS(monthKey, monthStart, monthEnd, prevStart, prevEnd, ytdStart),
	})
}

func writeMonthlySheet(xf *excelize.File, sheet string, rows []monthlyMisRow) {
	xf.SetSheetRow(sheet, "A1", &[]interface{}{"Particulars", "Current Month", "Previous Month", "YTD", "Budget", "Variance"})
	for i, r := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		xf.SetSheetRow(sheet, cell, &[]interface{}{r.Label, r.Current, r.Previous, r.YTD, r.Budget, r.Variance})
	}
	for _, col := range []string{"A", "B", "C", "D", "E", "F"} {
		xf.SetColWidth(sheet, col, col, 26)
	}
}

func ExportMonthlyMIS(c *gin.Context) {
	monthStart, monthEnd, prevStart, prevEnd, ytdStart := monthBounds(c.Query("month"))
	monthKey := monthStart.Format("2006-01")

	xf := excelize.NewFile()
	defer xf.Close()

	dashSheet := "Dashboard"
	xf.SetSheetName("Sheet1", dashSheet)
	writeMonthlySheet(xf, dashSheet, computeDashboardMonthlyMIS(monthKey, monthStart, monthEnd, prevStart, prevEnd, ytdStart))

	grocerySheet := "Grocery"
	xf.NewSheet(grocerySheet)
	writeMonthlySheet(xf, grocerySheet, computeGroceryMonthlyMIS(monthKey, monthStart, monthEnd, prevStart, prevEnd, ytdStart))

	xf.SetActiveSheet(0)
	filename := fmt.Sprintf("monthly-mis-%s.xlsx", monthKey)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := xf.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Excel file"})
	}
}

var _ = math.Round

func monthBoundsYearStart(t time.Time) time.Time {
	return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
}
