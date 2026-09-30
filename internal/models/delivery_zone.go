package models

import "time"

// DeliveryZone represents a serviceable area defined by a set of pincodes
// (or a city name as fallback). Used to check delivery availability, delivery
// charge, COD availability, and estimated delivery days at checkout.
type DeliveryZone struct {
ID              uint      `gorm:"primaryKey" json:"id"`
Name            string    `gorm:"not null" json:"name"` // e.g. "Ahmedabad Local", "Surat Extended"
City            string    `json:"city"`
Pincodes        string    `json:"pincodes"` // comma-separated, e.g. "380001,380002,380015"
DeliveryCharge  float64   `gorm:"default:0" json:"delivery_charge"`
IsCODAvailable  bool      `json:"is_cod_available"`
EstimatedDays   int       `gorm:"default:3" json:"estimated_days"`
IsActive        bool      `gorm:"default:true" json:"is_active"`
CreatedAt       time.Time `json:"created_at"`
UpdatedAt       time.Time `json:"updated_at"`
}

// CreateDeliveryZoneRequest is the admin request body for POST /admin/delivery-zones.
type CreateDeliveryZoneRequest struct {
Name           string  `json:"name" binding:"required"`
City           string  `json:"city"`
Pincodes       string  `json:"pincodes" binding:"required"`
DeliveryCharge float64 `json:"delivery_charge"`
IsCODAvailable *bool   `json:"is_cod_available"`
EstimatedDays  int     `json:"estimated_days"`
}
