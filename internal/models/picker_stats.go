package models

import "time"

// PickerStats tracks a picker's login/active time for the current IST day.
type PickerStats struct {
	StaffID       uint       `gorm:"primaryKey;autoIncrement:false" json:"staff_id"`
	StatsDate     string     `json:"stats_date"`
	FirstOnlineAt *time.Time `json:"first_online_at,omitempty"`
	OnlineSince   *time.Time `json:"online_since,omitempty"`
	ActiveSeconds int        `json:"active_seconds"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
