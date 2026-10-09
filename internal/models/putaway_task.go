package models

import "time"

const (
	PutawayPending    = "pending"
	PutawayInProgress = "in_progress"
	PutawayCompleted  = "completed"
)

// PutawayTask is one put-away job created when a receiving passes QC.
type PutawayTask struct {
	ID             uint          `gorm:"primaryKey" json:"id"`
	ReceivingID    uint          `gorm:"not null;uniqueIndex" json:"receiving_id"`
	Receiving      Receiving     `gorm:"foreignKey:ReceivingID" json:"receiving,omitempty"`
	WarehouseID    uint          `gorm:"not null;index" json:"warehouse_id"`
	ProductID      uint          `gorm:"not null;index" json:"product_id"`
	Product        Product       `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Quantity       int           `json:"quantity"`
	SuggestedBinID *uint         `json:"suggested_bin_id,omitempty"`
	SuggestedBin   *WarehouseBin `gorm:"foreignKey:SuggestedBinID" json:"suggested_bin,omitempty"`
	ActualBinID    *uint         `json:"actual_bin_id,omitempty"`
	ActualBin      *WarehouseBin `gorm:"foreignKey:ActualBinID" json:"actual_bin,omitempty"`
	PutterID       *uint         `gorm:"index" json:"putter_id,omitempty"` // WarehouseStaff.ID
	Status         string        `gorm:"default:pending;index" json:"status"`
	WrongLocation  bool          `json:"wrong_location"`
	Note           string        `json:"note,omitempty"`
	AssignedAt     *time.Time    `json:"assigned_at,omitempty"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	CompletedAt    *time.Time    `json:"completed_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}
