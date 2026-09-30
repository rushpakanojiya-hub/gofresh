package models

import "time"

// OTP stores a one-time code sent to a phone number for login.
// A new row is created every time /auth/send-otp is called; old/expired
// rows for the same phone are cleaned up when a new one is issued.
type OTP struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Phone     string    `gorm:"index;not null" json:"-"`
	Code      string    `gorm:"not null" json:"-"`
	ExpiresAt time.Time `json:"-"`
	Verified  bool      `gorm:"default:false" json:"-"`
	Attempts  int       `gorm:"not null;default:0" json:"-"`
	CreatedAt time.Time `json:"-"`
}
