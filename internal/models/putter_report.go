package models

import "time"

// PutterPutawayReport is a damaged / short-stock report raised by a putter on a putaway task.
type PutterPutawayReport struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TaskID      uint      `gorm:"not null;index" json:"task_id"`
	StaffID     uint      `gorm:"not null;index" json:"staff_id"`
	WarehouseID uint      `gorm:"not null;index" json:"warehouse_id"`
	Type        string    `gorm:"not null" json:"type"`
	Quantity    int       `json:"quantity"`
	Reason      string    `json:"reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
