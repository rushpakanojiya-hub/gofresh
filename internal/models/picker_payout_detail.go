package models

import "time"

// PickerPayoutDetail is where a picker's payout goes (UPI and/or bank).
type PickerPayoutDetail struct {
	StaffID       uint      `gorm:"primaryKey" json:"staff_id"`
	UPIID         string    `gorm:"column:upi_id" json:"upi_id"`
	AccountHolder string    `json:"account_holder"`
	AccountNumber string    `json:"-"`
	IFSC          string    `json:"ifsc"`
	UpdatedAt     time.Time `json:"updated_at"`
}
