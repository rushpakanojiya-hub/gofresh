package models

import "time"

// PickerPhoto is a picker's profile picture URL.
type PickerPhoto struct {
	StaffID   uint      `gorm:"primaryKey" json:"staff_id"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at"`
}
