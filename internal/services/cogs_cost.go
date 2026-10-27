package services

import (
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "gorm.io/gorm"
)

// BUG-09: unit cost for COGS on one order line. Uses the order_items.cost_price
// snapshot taken at checkout/substitution; rows created before the snapshot
// existed (cost_price = 0) fall back to the product's current cost (including
// soft-deleted products).
func orderLineUnitCost(db *gorm.DB, orderID, productID uint) float64 {
    var oi models.OrderItem
    if err := db.Where("order_id = ? AND product_id = ?", orderID, productID).Order("id").First(&oi).Error; err == nil && oi.CostPrice > 0 {
        return oi.CostPrice
    }
    var prod models.Product
    if err := db.Unscoped().Select("id", "cost_price").First(&prod, productID).Error; err == nil && prod.CostPrice > 0 {
        return prod.CostPrice
    }
    return 0
}