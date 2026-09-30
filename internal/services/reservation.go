package services

import (
	"errors"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReservationTTL is how long a cart hold lasts before it silently expires
// and the stock becomes available to other shoppers again.
const ReservationTTL = 10 * time.Minute

var ErrInsufficientStock = errors.New("insufficient stock for this product")

// ErrNotAvailableAtWarehouse means the product has no inventory row at all
// for the user's resolved warehouse (as opposed to a row with 0 stock),
// which is a normal "not stocked here yet" case, not a server error.
var ErrNotAvailableAtWarehouse = errors.New("this product is not available at your nearest warehouse")

// expireStaleReservations deletes reservation rows (for one product+warehouse)
// whose hold has already run out. Called right before every availability
// check/reserve so expired holds never block a genuinely free purchase -
// this is the "lazy cleanup" half of the TTL (see also the periodic sweep
// registered in main.go for reservations nobody happens to re-check).
func expireStaleReservations(tx *gorm.DB, productID, warehouseID uint) error {
	return tx.Where("product_id = ? AND warehouse_id = ? AND expires_at < ?",
		productID, warehouseID, time.Now()).
		Delete(&models.CartReservation{}).Error
}

// ReserveStock places (or refreshes) a 10-minute hold for userID on
// quantity units of productID at warehouseID. It fails with
// ErrInsufficientStock if, after subtracting everyone else's active holds,
// there isn't enough real stock left to grant this reservation.
//
// Must be called with the Inventory row already locked (SELECT ... FOR
// UPDATE) by the caller's transaction, so two concurrent reserves on the
// same product/warehouse can't both read the same "available" number and
// both succeed beyond real stock.
func ReserveStock(tx *gorm.DB, userID, productID, warehouseID uint, quantity int) error {
	if err := expireStaleReservations(tx, productID, warehouseID); err != nil {
		return err
	}

	var inventory models.Inventory
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ? AND warehouse_id = ?", productID, warehouseID).
		First(&inventory).Error; err != nil {
		return ErrNotAvailableAtWarehouse
	}
	if !inventory.InStock {
		return ErrInsufficientStock
	}

	var reservedByOthers int64
	if err := tx.Model(&models.CartReservation{}).
		Where("product_id = ? AND warehouse_id = ? AND user_id != ? AND expires_at >= ?",
			productID, warehouseID, userID, time.Now()).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&reservedByOthers).Error; err != nil {
		return err
	}

	available := inventory.Stock - int(reservedByOthers)
	if quantity > available {
		return ErrInsufficientStock
	}

	// Upsert: one row per (user, product, warehouse) - update in place.
	reservation := models.CartReservation{
		UserID:      userID,
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    quantity,
		ExpiresAt:   time.Now().Add(ReservationTTL),
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "product_id"}, {Name: "warehouse_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"quantity", "expires_at", "updated_at"}),
	}).Create(&reservation).Error
}

// ReleaseReservation removes a single (user, product, warehouse) hold, e.g.
// when the item is removed from the cart entirely.
func ReleaseReservation(userID, productID, warehouseID uint) error {
	return database.DB.
		Where("user_id = ? AND product_id = ? AND warehouse_id = ?", userID, productID, warehouseID).
		Delete(&models.CartReservation{}).Error
}

// ReleaseAllUserReservations clears every hold a user has, regardless of
// warehouse. Called after a successful checkout (the real stock deduction
// already happened under its own lock, so the holds have served their
// purpose) and after the cart is cleared, so nothing lingers until its TTL.
func ReleaseAllUserReservations(tx *gorm.DB, userID uint) error {
	return tx.Where("user_id = ?", userID).Delete(&models.CartReservation{}).Error
}

// CleanupExpiredReservations deletes every reservation row (any product,
// any warehouse) whose hold has run out. Registered as a periodic cron job
// in main.go so rows don't pile up indefinitely for products nobody
// re-checks after the original shopper wanders off.
func CleanupExpiredReservations() {
	database.DB.Where("expires_at < ?", time.Now()).Delete(&models.CartReservation{})
}
