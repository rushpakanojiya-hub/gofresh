package models

import "time"

// WarehouseZone is the top level of the physical location hierarchy inside
// one warehouse (e.g. "Zone A" - frozen, ambient, fragile, etc).
type WarehouseZone struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WarehouseID uint      `gorm:"not null;uniqueIndex:idx_zone_warehouse_name" json:"warehouse_id"`
	Warehouse   Warehouse `gorm:"foreignKey:WarehouseID" json:"warehouse,omitempty"`
	Name        string    `gorm:"not null;uniqueIndex:idx_zone_warehouse_name" json:"name"`
	StorageType string    `gorm:"size:10;not null;default:dry" json:"storage_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WarehouseRack belongs to one zone (e.g. "Rack A03").
type WarehouseRack struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	ZoneID    uint          `gorm:"not null;uniqueIndex:idx_rack_zone_name" json:"zone_id"`
	Zone      WarehouseZone `gorm:"foreignKey:ZoneID" json:"zone,omitempty"`
	Name      string        `gorm:"not null;uniqueIndex:idx_rack_zone_name" json:"name"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// WarehouseBin is the smallest physical location unit, belongs to one rack
// (e.g. "Bin A03-12"). A product's inventory row points at a bin.
type WarehouseBin struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	RackID    uint          `gorm:"not null;uniqueIndex:idx_bin_rack_name" json:"rack_id"`
	Rack      WarehouseRack `gorm:"foreignKey:RackID" json:"rack,omitempty"`
	Name      string        `gorm:"not null;uniqueIndex:idx_bin_rack_name" json:"name"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// WarehouseZoneRequest is the body for POST /admin/warehouses/:id/zones and
// POST /warehouse/zones (warehouse manager use).
type WarehouseZoneRequest struct {
	Name        string `json:"name" binding:"required"`
	StorageType string `json:"storage_type"`
}

// WarehouseRackRequest is the body for POST /warehouse/zones/:zone_id/racks
type WarehouseRackRequest struct {
	Name string `json:"name" binding:"required"`
}

// WarehouseBinRequest is the body for POST /warehouse/racks/:rack_id/bins
type WarehouseBinRequest struct {
	Name string `json:"name" binding:"required"`
}

// AssignBinRequest is the body for PUT /warehouse/inventory/:product_id/bin
type AssignBinRequest struct {
	BinID *uint `json:"bin_id"` // null clears the assignment
}
