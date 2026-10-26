package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// AnalyticsSummary is the response shape for GET /admin/analytics/summary
type AnalyticsSummary struct {
	TotalUsers      int64   `json:"total_users"`
	TotalOrders     int64   `json:"total_orders"`
	TotalSales      float64 `json:"total_sales"`
	PendingOrders   int64   `json:"pending_orders"`
	DeliveredOrders int64   `json:"delivered_orders"`
	CancelledOrders int64   `json:"cancelled_orders"`
}

// GetAnalyticsSummary godoc
// GET /api/v1/admin/analytics/summary (admin only)
// Returns high-level store metrics: user count, order count, total revenue
// (from paid/confirmed orders only, not cancelled), and an order status
// breakdown.
func GetAnalyticsSummary(c *gin.Context) {
	var summary AnalyticsSummary

	database.DB.Model(&models.User{}).Count(&summary.TotalUsers)
	database.DB.Model(&models.Order{}).Count(&summary.TotalOrders)

	database.DB.Model(&models.Order{}).
		Where("status != ?", "cancelled").
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&summary.TotalSales)

	database.DB.Model(&models.Order{}).Where("status = ?", "pending").Count(&summary.PendingOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "delivered").Count(&summary.DeliveredOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "cancelled").Count(&summary.CancelledOrders)

	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// ProductPerformance is one row of GET /admin/analytics/products
type ProductPerformance struct {
	ProductID    uint    `json:"product_id"`
	ProductName  string  `json:"product_name"`
	UnitsSold    int64   `json:"units_sold"`
	TotalRevenue float64 `json:"total_revenue"`
}

// GetProductPerformance godoc
// GET /api/v1/admin/analytics/products (admin only)
// Returns units sold and revenue per product, based on order_items,
// sorted by revenue descending. Useful for spotting best-sellers.
func GetProductPerformance(c *gin.Context) {
	var results []ProductPerformance

	err := database.DB.
		Table("order_items").
		Select("order_items.product_id, products.name as product_name, SUM(order_items.quantity) as units_sold, SUM(order_items.quantity * order_items.price) as total_revenue").
		Joins("JOIN products ON products.id = order_items.product_id").
		Group("order_items.product_id, products.name").
		Order("total_revenue DESC").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load product performance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"products": results})
}
