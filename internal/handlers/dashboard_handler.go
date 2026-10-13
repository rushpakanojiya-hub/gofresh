package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// dashboardTrendDays is how many days of history the trend charts cover.
const dashboardTrendDays = 14

// DashboardStats is the stat-card block of GET /admin/analytics/dashboard.
type DashboardStats struct {
	TotalUsers             int64   `json:"total_users"`
	NewUsersToday          int64   `json:"new_users_today"`
	TotalOrders            int64   `json:"total_orders"`
	OrdersToday            int64   `json:"orders_today"`
	TotalSales             float64 `json:"total_sales"`
	RevenueToday           float64 `json:"revenue_today"`
	AvgOrderValue          float64 `json:"avg_order_value"`
	PendingOrders          int64   `json:"pending_orders"`
	ConfirmedOrders        int64   `json:"confirmed_orders"`
	ShippedOrders          int64   `json:"shipped_orders"`
	DeliveredOrders        int64   `json:"delivered_orders"`
	CancelledOrders        int64   `json:"cancelled_orders"`
	ReturnedOrders         int64   `json:"returned_orders"`
	TotalProducts          int64   `json:"total_products"`
	LowStockProducts       int64   `json:"low_stock_products"`
	OutOfStockProducts     int64   `json:"out_of_stock_products"`
	ActiveDeliveryPartners int64   `json:"active_delivery_partners"`
	TotalWarehouses        int64   `json:"total_warehouses"`
	OpenSupportTickets     int64   `json:"open_support_tickets"`
	PendingPaymentAmount   float64 `json:"pending_payment_amount"`
}

// DashboardTrendPoint is one day of the sales/orders trend chart.
type DashboardTrendPoint struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
	Orders  int64   `json:"orders"`
}

// DashboardUserPoint is one day of the user-growth trend chart.
type DashboardUserPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// DashboardStatusCount is a generic status -> count row (orders, tickets).
type DashboardStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// DashboardPaymentSplit is one payment method's collected revenue.
type DashboardPaymentSplit struct {
	Method  string  `json:"method"`
	Revenue float64 `json:"revenue"`
	Count   int64   `json:"count"`
}

// DashboardWarehouseRevenue is one warehouse's revenue contribution.
type DashboardWarehouseRevenue struct {
	WarehouseName string  `json:"warehouse_name"`
	Revenue       float64 `json:"revenue"`
}

// DashboardCharts bundles every chart dataset for the dashboard.
type DashboardCharts struct {
	SalesTrend         []DashboardTrendPoint       `json:"sales_trend"`
	UserGrowth         []DashboardUserPoint        `json:"user_growth"`
	OrdersByStatus     []DashboardStatusCount      `json:"orders_by_status"`
	PaymentSplit       []DashboardPaymentSplit     `json:"payment_split"`
	TopProducts        []ProductPerformance        `json:"top_products"`
	RevenueByWarehouse []DashboardWarehouseRevenue `json:"revenue_by_warehouse"`
	TicketsByStatus    []DashboardStatusCount      `json:"tickets_by_status"`
}

// DashboardOverviewResponse is the full response for GET /admin/analytics/dashboard.
type DashboardOverviewResponse struct {
	Stats  DashboardStats  `json:"stats"`
	Charts DashboardCharts `json:"charts"`
}

// GetDashboardOverview godoc
// GET /api/v1/admin/analytics/dashboard (admin only)
// Powers the admin Overview page: 19 headline stats plus 7 chart datasets
// (sales trend, user growth, order-status split, payment-method split,
// top products, revenue by warehouse, ticket-status split).
func GetDashboardOverview(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	trendStart := todayStart.AddDate(0, 0, -(dashboardTrendDays - 1))

	var stats DashboardStats

	database.DB.Model(&models.User{}).Count(&stats.TotalUsers)
	database.DB.Model(&models.User{}).Where("created_at >= ?", todayStart).Count(&stats.NewUsersToday)

	database.DB.Model(&models.Order{}).Count(&stats.TotalOrders)
	database.DB.Model(&models.Order{}).Where("created_at >= ?", todayStart).Count(&stats.OrdersToday)

	database.DB.Model(&models.Order{}).
		Where("status != ?", "cancelled").
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&stats.TotalSales)

	database.DB.Model(&models.Order{}).
		Where("status != ? AND created_at >= ?", "cancelled", todayStart).
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&stats.RevenueToday)

	if stats.TotalOrders > 0 {
		stats.AvgOrderValue = stats.TotalSales / float64(stats.TotalOrders)
	}

	database.DB.Model(&models.Order{}).Where("status = ?", "pending").Count(&stats.PendingOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "confirmed").Count(&stats.ConfirmedOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "shipped").Count(&stats.ShippedOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "delivered").Count(&stats.DeliveredOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "cancelled").Count(&stats.CancelledOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "returned").Count(&stats.ReturnedOrders)

	database.DB.Model(&models.Product{}).Count(&stats.TotalProducts)
	database.DB.Model(&models.Inventory{}).Where("stock > 0 AND stock < ?", lowStockThreshold).Count(&stats.LowStockProducts)
	database.DB.Model(&models.Inventory{}).Where("stock <= 0").Count(&stats.OutOfStockProducts)

	database.DB.Model(&models.DeliveryPartner{}).Where("is_active = ?", true).Count(&stats.ActiveDeliveryPartners)
	database.DB.Model(&models.Warehouse{}).Where("(warehouse_type IS NULL OR warehouse_type <> ?)", "mother_warehouse").Count(&stats.TotalWarehouses)
	database.DB.Model(&models.SupportTicket{}).Where("status IN ?", []string{"open", "in_progress"}).Count(&stats.OpenSupportTickets)

	database.DB.Model(&models.Order{}).
		Where("payment_status = ?", "pending").
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&stats.PendingPaymentAmount)

	// --- Charts ---
	charts := DashboardCharts{}

	// Sales + orders trend (last N days, gap-filled with zeros)
	type trendRow struct {
		Day     string
		Revenue float64
		Orders  int64
	}
	var trendRows []trendRow
	database.DB.Model(&models.Order{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as day, COALESCE(SUM(total_amount), 0) as revenue, COUNT(*) as orders").
		Where("created_at >= ? AND status != ?", trendStart, "cancelled").
		Group("day").
		Order("day").
		Scan(&trendRows)

	trendByDay := make(map[string]trendRow, len(trendRows))
	for _, r := range trendRows {
		trendByDay[r.Day] = r
	}
	charts.SalesTrend = make([]DashboardTrendPoint, 0, dashboardTrendDays)
	for i := 0; i < dashboardTrendDays; i++ {
		day := trendStart.AddDate(0, 0, i).Format("2006-01-02")
		if r, ok := trendByDay[day]; ok {
			charts.SalesTrend = append(charts.SalesTrend, DashboardTrendPoint{Date: day, Revenue: r.Revenue, Orders: r.Orders})
		} else {
			charts.SalesTrend = append(charts.SalesTrend, DashboardTrendPoint{Date: day, Revenue: 0, Orders: 0})
		}
	}

	// User growth (last N days, gap-filled with zeros)
	type userRow struct {
		Day   string
		Count int64
	}
	var userRows []userRow
	database.DB.Model(&models.User{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as day, COUNT(*) as count").
		Where("created_at >= ?", trendStart).
		Group("day").
		Order("day").
		Scan(&userRows)

	userByDay := make(map[string]int64, len(userRows))
	for _, r := range userRows {
		userByDay[r.Day] = r.Count
	}
	charts.UserGrowth = make([]DashboardUserPoint, 0, dashboardTrendDays)
	for i := 0; i < dashboardTrendDays; i++ {
		day := trendStart.AddDate(0, 0, i).Format("2006-01-02")
		charts.UserGrowth = append(charts.UserGrowth, DashboardUserPoint{Date: day, Count: userByDay[day]})
	}

	// Pre-initialize every chart slice to empty (not nil). GORM leaves a
	// slice nil when a query matches zero rows, and encoding/json renders a
	// nil slice as null instead of [] - which crashes the frontend's
	// .map() calls on a fresh/low-data database. Empty slices avoid that.
	charts.OrdersByStatus = []DashboardStatusCount{}
	charts.PaymentSplit = []DashboardPaymentSplit{}
	charts.TopProducts = []ProductPerformance{}
	charts.RevenueByWarehouse = []DashboardWarehouseRevenue{}
	charts.TicketsByStatus = []DashboardStatusCount{}

	// Orders by status
	database.DB.Model(&models.Order{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&charts.OrdersByStatus)

	// Payment method split (paid orders only)
	database.DB.Model(&models.Order{}).
		Select("payment_method as method, COALESCE(SUM(total_amount), 0) as revenue, COUNT(*) as count").
		Where("payment_status = ?", "paid").
		Group("payment_method").
		Scan(&charts.PaymentSplit)

	// Top 5 products by revenue
	database.DB.
		Table("order_items").
		Select("order_items.product_id, products.name as product_name, SUM(order_items.quantity) as units_sold, SUM(order_items.quantity * order_items.price) as total_revenue").
		Joins("JOIN products ON products.id = order_items.product_id").
		Group("order_items.product_id, products.name").
		Order("total_revenue DESC").
		Limit(5).
		Scan(&charts.TopProducts)

	// Revenue by warehouse (top 6)
	database.DB.
		Table("orders").
		Select("warehouses.name as warehouse_name, COALESCE(SUM(orders.total_amount), 0) as revenue").
		Joins("JOIN warehouses ON warehouses.id = orders.warehouse_id").
		Where("orders.status != ?", "cancelled").
		Group("warehouses.name").
		Order("revenue DESC").
		Limit(6).
		Scan(&charts.RevenueByWarehouse)

	// Support tickets by status
	database.DB.Model(&models.SupportTicket{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&charts.TicketsByStatus)

	c.JSON(http.StatusOK, DashboardOverviewResponse{Stats: stats, Charts: charts})
}
