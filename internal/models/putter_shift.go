package models

import "time"

// PutterShift is one start/end shift of a putter.
type PutterShift struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	StaffID     uint       `gorm:"not null;index" json:"staff_id"`
	WarehouseID uint       `gorm:"not null;index" json:"warehouse_id"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}
