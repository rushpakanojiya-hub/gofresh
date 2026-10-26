package handlers

import (
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "gorm.io/gorm"
)

// inventorySummaryBase returns an Inventory query carrying the same
// warehouse/category filters as the inventory table (BUG-64).
func inventorySummaryBase(warehouseID, categoryID int64) *gorm.DB {
    q := database.DB.Model(&models.Inventory{}).
        Joins("JOIN products ON products.id = inventories.product_id")
    if warehouseID > 0 {
        q = q.Where("inventories.warehouse_id = ?", warehouseID)
    }
    if categoryID > 0 {
        q = q.Where("products.category_id = ?", categoryID)
    }
    return q
}