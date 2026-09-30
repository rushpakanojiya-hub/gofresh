package models

import "time"

// Inventory tracks stock for a product AT a specific warehouse.
// Each (ProductID, WarehouseID) pair has exactly one row — a product can
// have different stock counts across different warehouses.
type Inventory struct {
	ID                uint          `gorm:"primaryKey" json:"id"`
	ProductID         uint          `gorm:"not null;uniqueIndex:idx_product_warehouse" json:"product_id"`
	Product           Product       `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	WarehouseID       uint          `gorm:"not null;uniqueIndex:idx_product_warehouse" json:"warehouse_id"`
	Warehouse         Warehouse     `gorm:"foreignKey:WarehouseID" json:"warehouse,omitempty"`
	BinID             *uint         `gorm:"index" json:"bin_id,omitempty"`
	Bin               *WarehouseBin `gorm:"foreignKey:BinID" json:"bin,omitempty"`
	Stock             int           `gorm:"default:0" json:"stock"`
	InStock           bool          `gorm:"default:true" json:"in_stock"`
	LowStockThreshold *int          `json:"low_stock_threshold,omitempty"`
	StockAlertState   string        `gorm:"default:normal" json:"stock_alert_state"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// InventoryUpdateRequest is the body for PUT /admin/products/:id/inventory (admin only).
// Stock is an absolute value (not a delta) — it replaces the current stock count
// for the given warehouse.
type InventoryUpdateRequest struct {
	WarehouseID uint `json:"warehouse_id" binding:"required"`
	Stock       int  `json:"stock" binding:"gte=0"`
}
