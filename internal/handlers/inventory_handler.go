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
for _, inv := range invs {
var reserved int
database.DB.Model(&models.CartReservation{}).
Where("product_id = ? AND warehouse_id = ? AND expires_at > ?", inv.ProductID, inv.WarehouseID, time.Now()).
Select("COALESCE(SUM(quantity), 0)").Scan(&reserved)

rows = append(rows, models.InventoryRow{
ProductID:     inv.ProductID,
ProductName:   inv.Product.Name,
CategoryName:  inv.Product.Category.Name,
WarehouseID:   inv.WarehouseID,
WarehouseName: inv.Warehouse.Name,
Stock:         inv.Stock,
Reserved:      reserved,
Available:     max(0, inv.Stock - reserved),
InStock:       inv.InStock,
})
}

// Summary stats computed across ALL inventory (not just this page)
summaryDB := database.DB.Model(&models.Inventory{})
var totalSKUs int64
summaryDB.Count(&totalSKUs)

var totalAvailable int64
database.DB.Model(&models.Inventory{}).Select("COALESCE(SUM(stock), 0)").Scan(&totalAvailable)

var totalReserved int64
database.DB.Model(&models.CartReservation{}).Where("expires_at > ?", time.Now()).
Select("COALESCE(SUM(quantity), 0)").Scan(&totalReserved)

var lowStockCount int64
database.DB.Model(&models.Inventory{}).Where("stock > 0 AND stock < ?", lowStockThreshold).Count(&lowStockCount)

var outOfStockCount int64
database.DB.Model(&models.Inventory{}).Where("stock <= 0").Count(&outOfStockCount)

damagedCutoff := time.Now().AddDate(0, 0, -30)
var damagedCount int64
database.DB.Model(&models.StockMovement{}).
Where("reason = ? AND created_at >= ?", models.AdjustReasonDamaged, damagedCutoff).
Distinct("product_id").Count(&damagedCount)

var expiredCount int64
database.DB.Model(&models.Batch{}).
Where("expiry_date < ? AND quantity > 0", time.Now()).
Distinct("product_id").Count(&expiredCount)

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
