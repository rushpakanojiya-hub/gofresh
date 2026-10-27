package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

const lowStockThreshold = 10

// GetInventoryOverview godoc
// GET /api/v1/admin/inventory (admin only)
func GetInventoryOverview(c *gin.Context) {
	var query models.InventoryOverviewQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}

	db := database.DB.Model(&models.Inventory{}).
		Joins("JOIN products ON products.id = inventories.product_id").
		Joins("JOIN warehouses ON warehouses.id = inventories.warehouse_id")

	if query.WarehouseID > 0 {
		db = db.Where("inventories.warehouse_id = ?", query.WarehouseID)
	}
	if query.CategoryID > 0 {
		db = db.Where("products.category_id = ?", query.CategoryID)
	}
	if query.StockStatus == "out" {
		db = db.Where("inventories.stock <= 0")
	} else if query.StockStatus == "low" {
		db = db.Where("inventories.stock > 0 AND inventories.stock < ?", lowStockThreshold)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count inventory"})
		return
	}

	offset := (query.Page - 1) * query.Limit
	var invs []models.Inventory
	if err := db.Preload("Product").Preload("Product.Category").Preload("Warehouse").
		Order("inventories.stock ASC").Offset(offset).Limit(query.Limit).Find(&invs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventory"})
		return
	}

	rows := make([]models.InventoryRow, 0, len(invs))
	// BUG-19: one grouped query for the whole page instead of one query per row.
	type reservedRow struct {
		ProductID   uint
		WarehouseID uint
		Qty         int
	}
	var reservedRows []reservedRow
	if len(invs) > 0 {
		pids := make([]uint, 0, len(invs))
		for _, inv := range invs {
			pids = append(pids, uint(inv.ProductID))
		}
		database.DB.Model(&models.CartReservation{}).
			Select("product_id, warehouse_id, COALESCE(SUM(quantity), 0) AS qty").
			Where("product_id IN ? AND expires_at > ?", pids, time.Now()).
			Group("product_id, warehouse_id").
			Scan(&reservedRows)
	}
	reservedByKey := make(map[[2]uint]int, len(reservedRows))
	for _, r := range reservedRows {
		reservedByKey[[2]uint{r.ProductID, r.WarehouseID}] = r.Qty
	}

	for _, inv := range invs {
		reserved := reservedByKey[[2]uint{uint(inv.ProductID), uint(inv.WarehouseID)}]

		rows = append(rows, models.InventoryRow{
			ProductID:     inv.ProductID,
			ProductName:   inv.Product.Name,
			CategoryName:  inv.Product.Category.Name,
			WarehouseID:   inv.WarehouseID,
			WarehouseName: inv.Warehouse.Name,
			Stock:         inv.Stock,
			Reserved:      reserved,
			Available:     max(0, inv.Stock-reserved),
			InStock:       inv.InStock,
		})
	}

	// Summary stats computed across ALL inventory (not just this page)
	sumWH := int64(query.WarehouseID)
    sumCat := int64(query.CategoryID)
    // BUG-64: summary cards follow the same warehouse/category filters as the table,
    // and "available" is stock minus active cart reservations (never below 0 per row).
    var totalSKUs int64
    inventorySummaryBase(sumWH, sumCat).Count(&totalSKUs)

    var totalAvailable int64
    inventorySummaryBase(sumWH, sumCat).
        Joins("LEFT JOIN (SELECT product_id, warehouse_id, SUM(quantity) AS qty FROM cart_reservations WHERE expires_at > ? GROUP BY product_id, warehouse_id) r ON r.product_id = inventories.product_id AND r.warehouse_id = inventories.warehouse_id", time.Now()).
        Select("COALESCE(SUM(GREATEST(inventories.stock - COALESCE(r.qty, 0), 0)), 0)").Scan(&totalAvailable)

    var totalReserved int64
    resQ := database.DB.Model(&models.CartReservation{}).
        Joins("JOIN products ON products.id = cart_reservations.product_id").
        Where("cart_reservations.expires_at > ?", time.Now())
    if sumWH > 0 {
        resQ = resQ.Where("cart_reservations.warehouse_id = ?", sumWH)
    }
    if sumCat > 0 {
        resQ = resQ.Where("products.category_id = ?", sumCat)
    }
    resQ.Select("COALESCE(SUM(cart_reservations.quantity), 0)").Scan(&totalReserved)

    var lowStockCount int64
    inventorySummaryBase(sumWH, sumCat).Where("inventories.stock > 0 AND inventories.stock < ?", lowStockThreshold).Count(&lowStockCount)

    var outOfStockCount int64
    inventorySummaryBase(sumWH, sumCat).Where("inventories.stock <= 0").Count(&outOfStockCount)

	damagedCutoff := time.Now().AddDate(0, 0, -30)
	// BUG-15: damaged/expired cards follow the same warehouse/category filters as the other summary cards.
	var damagedCount int64
	damagedQ := database.DB.Model(&models.StockMovement{}).
	    Where("stock_movements.reason = ? AND stock_movements.created_at >= ?", models.AdjustReasonDamaged, damagedCutoff)
	if sumWH > 0 {
	    damagedQ = damagedQ.Where("stock_movements.warehouse_id = ?", sumWH)
	}
	if sumCat > 0 {
	    damagedQ = damagedQ.Joins("JOIN products ON products.id = stock_movements.product_id").Where("products.category_id = ?", sumCat)
	}
	damagedQ.Distinct("product_id").Count(&damagedCount)
	
	var expiredCount int64
	expiredQ := database.DB.Model(&models.Batch{}).
	    Where("batches.expiry_date < ? AND batches.quantity > 0", time.Now())
	if sumWH > 0 {
	    expiredQ = expiredQ.Where("batches.warehouse_id = ?", sumWH)
	}
	if sumCat > 0 {
	    expiredQ = expiredQ.Joins("JOIN products ON products.id = batches.product_id").Where("products.category_id = ?", sumCat)
	}
	expiredQ.Distinct("product_id").Count(&expiredCount)

	c.JSON(http.StatusOK, models.InventoryOverviewResponse{
		TotalSKUs:       totalSKUs,
		TotalAvailable:  totalAvailable,
		TotalReserved:   totalReserved,
		LowStockCount:   lowStockCount,
		OutOfStockCount: outOfStockCount,
		DamagedStock:    damagedCount,
		ExpiredStock:    expiredCount,
		Rows:            rows,
		Page:            query.Page,
		Limit:           query.Limit,
		Total:           total,
		TotalPages:      int((total + int64(query.Limit) - 1) / int64(query.Limit)),
	})
}
