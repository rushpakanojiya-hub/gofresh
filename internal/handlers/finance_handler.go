package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
)

// revenueDateRange parses optional ?from=YYYY-MM-DD&to=YYYY-MM-DD query
// params, defaulting to the last 30 days when absent so the endpoint is
// usable with no params for a quick "recent revenue" view.
func revenueDateRange(c *gin.Context) (time.Time, time.Time) {
	to := time.Now()
	from := to.AddDate(0, 0, -30)

	if v := c.Query("from"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			from = parsed
		}
	}
	if v := c.Query("to"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			// Include the whole "to" day.
			to = parsed.Add(24 * time.Hour)
		}
	}
	return from, to
}

// GetRevenueSummary godoc
// GET /api/v1/admin/finance/revenue?from=YYYY-MM-DD&to=YYYY-MM-DD
// Only counts orders that actually collected money (paid online orders, or
// COD orders that reached delivered) - a cancelled/pending COD order isn't
// real revenue yet, so it's excluded rather than counted at face value.
func GetRevenueSummary(c *gin.Context) {
	from, to := revenueDateRange(c)

	type totals struct {
		GrossSales     float64
		DeliveryCharge float64
		PlatformFee    float64
		OrderCount     int64
	}
	var t totals

	base := database.DB.Table("orders").
		Where("created_at >= ? AND created_at < ?", from, to).
		Where("(payment_method = 'online' AND payment_status = 'paid' AND status NOT IN ('cancelled','returned')) OR (payment_method = 'cod' AND status = 'delivered')")

	base.
		Select("COALESCE(SUM(items_amount),0) as gross_sales, COALESCE(SUM(delivery_charge),0) as delivery_charge, COALESCE(SUM(platform_fee),0) as platform_fee, COUNT(*) as order_count").
		Scan(&t)

	// Discounts: coupon-driven markdowns aren't stored directly on orders,
	// so approximate via order_coupons if present; otherwise 0.
	var discountTotal float64
	database.DB.Table("order_coupons").
		Joins("JOIN orders ON orders.id = order_coupons.order_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where("(orders.payment_method = 'online' AND orders.payment_status = 'paid' AND orders.status NOT IN ('cancelled','returned')) OR (orders.payment_method = 'cod' AND orders.status = 'delivered')").
		Select("COALESCE(SUM(order_coupons.discount_amount),0)").
		Scan(&discountTotal)

	netSales := t.GrossSales - discountTotal

	// By warehouse
	type warehouseRow struct {
		WarehouseID *uint   `json:"warehouse_id"`
		Name        string  `json:"warehouse_name"`
		Revenue     float64 `json:"revenue"`
		OrderCount  int64   `json:"order_count"`
	}
	byWarehouse := []warehouseRow{}
	database.DB.Table("orders").
		Select("orders.warehouse_id, COALESCE(warehouses.name, 'Unassigned') as name, COALESCE(SUM(orders.items_amount),0) as revenue, COUNT(*) as order_count").
		Joins("LEFT JOIN warehouses ON warehouses.id = orders.warehouse_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where("(orders.payment_method = 'online' AND orders.payment_status = 'paid' AND orders.status NOT IN ('cancelled','returned')) OR (orders.payment_method = 'cod' AND orders.status = 'delivered')").
		Group("orders.warehouse_id, warehouses.name").
		Order("revenue DESC").
		Scan(&byWarehouse)

	// By product (top 20 by revenue)
	type productRow struct {
		ProductID   uint    `json:"product_id"`
		ProductName string  `json:"product_name"`
		Revenue     float64 `json:"revenue"`
		Quantity    int64   `json:"quantity"`
	}
	byProduct := []productRow{}
	database.DB.Table("order_items").
		Select("order_items.product_id, products.name as product_name, COALESCE(SUM(order_items.price * order_items.quantity),0) as revenue, COALESCE(SUM(order_items.quantity),0) as quantity").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("LEFT JOIN products ON products.id = order_items.product_id").
		Where("orders.created_at >= ? AND orders.created_at < ?", from, to).
		Where("(orders.payment_method = 'online' AND orders.payment_status = 'paid' AND orders.status NOT IN ('cancelled','returned')) OR (orders.payment_method = 'cod' AND orders.status = 'delivered')").
		Group("order_items.product_id, products.name").
		Order("revenue DESC").
		Limit(20).
		Scan(&byProduct)

	c.JSON(http.StatusOK, gin.H{
		"from":            from.Format("2006-01-02"),
		"to":              to.AddDate(0, 0, -1).Format("2006-01-02"),
		"gross_sales":     t.GrossSales,
		"net_sales":       netSales,
		"discounts":       discountTotal,
		"delivery_charge": t.DeliveryCharge,
		"platform_fee":    t.PlatformFee,
		"order_count":     t.OrderCount,
		"by_warehouse":    byWarehouse,
		"by_product":      byProduct,
	})
}
