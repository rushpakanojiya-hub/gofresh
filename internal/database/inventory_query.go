package database

import (
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetTotalStock returns the combined available stock for a product across
// all warehouses. Only warehouse-inventory rows marked in_stock count toward
// the total — a warehouse that has zeroed out its stock or flagged a product
// unavailable does not contribute.
//
// NOTE: this is a Phase 3 simplification (single combined availability
// number). Routing an order to a specific nearest warehouse comes in a
// later phase; for now, checkout/cart treat "in stock somewhere" as
// purchasable.
func GetTotalStock(productID uint) (int, error) {
return GetTotalStockForUser(productID, 0) // BUG-88: delegates to the reservation-aware version
}

// GetTotalStockForUser returns combined purchasable stock for a product
// across open, active, non-mother warehouses, minus other shoppers' active
// cart reservations (BUG-88). The given user's own reservations are not
// subtracted, so they are never blocked by their own hold. Pass userID 0
// to subtract every reservation.
func GetTotalStockForUser(productID uint, userID uint) (int, error) {
    var stock int
    err := DB.Model(&models.Inventory{}).
        Joins("JOIN warehouses ON warehouses.id = inventories.warehouse_id").
        Where("inventories.product_id = ? AND inventories.in_stock = ?", productID, true).
        Where("warehouses.is_active = ? AND warehouses.status = ? AND (warehouses.warehouse_type IS NULL OR warehouses.warehouse_type <> ?)", true, "open", "mother_warehouse").
        Select("COALESCE(SUM(inventories.stock), 0)").
        Scan(&stock).Error
    if err != nil {
        return 0, err
    }

    var reserved int
    rq := DB.Model(&models.CartReservation{}).
        Joins("JOIN warehouses ON warehouses.id = cart_reservations.warehouse_id").
        Where("cart_reservations.product_id = ? AND cart_reservations.expires_at > ?", productID, time.Now()).
        Where("warehouses.is_active = ? AND warehouses.status = ? AND (warehouses.warehouse_type IS NULL OR warehouses.warehouse_type <> ?)", true, "open", "mother_warehouse")
    if userID > 0 {
        rq = rq.Where("cart_reservations.user_id <> ?", userID)
    }
    if err := rq.Select("COALESCE(SUM(cart_reservations.quantity), 0)").Scan(&reserved).Error; err != nil {
        return 0, err
    }

    total := stock - reserved
    if total < 0 {
        total = 0
    }
    return total, nil
}