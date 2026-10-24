package models

import "time"

// GiftCard is a prepaid code that credits the customer's wallet once.
type GiftCard struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	Code             string     `gorm:"uniqueIndex;not null" json:"code"`
	Amount           float64    `gorm:"not null" json:"amount"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RedeemedByUserID *uint      `gorm:"index" json:"redeemed_by_user_id,omitempty"`
	RedeemedAt       *time.Time `json:"redeemed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}
