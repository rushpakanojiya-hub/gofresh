package handlers

import (
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
)

// transferReservedQty (BUG-61) returns units at the warehouse currently held
// by active customer cart reservations, so a stock transfer cannot take them.
func transferReservedQty(tx *gorm.DB, productID, warehouseID uint) int {
	var reserved int
	tx.Model(&models.CartReservation{}).
		Where("product_id = ? AND warehouse_id = ? AND expires_at > ?", productID, warehouseID, time.Now()).
		Select("COALESCE(SUM(quantity), 0)").Scan(&reserved)
	return reserved
}
