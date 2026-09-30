package models

import (
	"time"

	"gorm.io/gorm"
)

// Warehouse is a dark-store / fulfillment location the admin manages.
// Orders are routed to the nearest active warehouse, which then owns its
// own inventory and packs/ships from there.
type Warehouse struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	Name              string         `gorm:"not null" json:"name"`
	City              string         `gorm:"not null;index" json:"city"`
	Address           string         `json:"address"`
	Lat               float64        `json:"lat"`
	Lng               float64        `json:"lng"`
	ServiceRadiusKm   float64        `gorm:"default:5" json:"service_radius_km"`
	ServiceArea       string         `gorm:"-" json:"service_area,omitempty"`
	IsActive          bool           `gorm:"default:true" json:"is_active"`
	WarehouseType     string         `gorm:"default:dark_store" json:"warehouse_type"`
	Status            string         `gorm:"default:open" json:"status"`
	Capacity          int            `gorm:"default:0" json:"capacity"`
	OpeningTime       *string        `json:"opening_time,omitempty"`
	ClosingTime       *string        `json:"closing_time,omitempty"`
	LowStockThreshold *int           `json:"low_stock_threshold,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// WarehouseRequest is the body for POST/PUT /admin/warehouses
type WarehouseRequest struct {
	Name            string  `json:"name" binding:"required"`
	City            string  `json:"city" binding:"required"`
	Address         string  `json:"address"`
	Lat             float64 `json:"lat" binding:"required"`
	Lng             float64 `json:"lng" binding:"required"`
	ServiceRadiusKm float64 `json:"service_radius_km"`
	IsActive        *bool   `json:"is_active"`
	WarehouseType   string  `json:"warehouse_type"`
	Status          string  `json:"status"`
	Capacity        int     `json:"capacity"`
	OpeningTime     *string `json:"opening_time"`
	ClosingTime     *string `json:"closing_time"`
}
