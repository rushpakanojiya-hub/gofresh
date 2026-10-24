package services

import "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"

// orderWarehouseScope (BUG-45) returns the warehouse a rider must be checked
// in at to be eligible for this order. Legacy orders without a WarehouseID
// keep the old behaviour (any checked-in rider): anyWarehouse is true.
func orderWarehouseScope(order *models.Order) (anyWarehouse bool, warehouseID uint) {
	if order == nil || order.WarehouseID == nil {
		return true, 0
	}
	return false, *order.WarehouseID
}
