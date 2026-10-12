package models

import "time"

// DeviceToken stores an FCM push notification token registered by a user's
// device (via the Flutter app), so the backend can send push notifications
// (e.g. daily engagement messages, order updates) to that device.
type DeviceToken struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	UserID            *uint     `json:"user_id,omitempty"`
	DeliveryPartnerID *uint     `json:"delivery_partner_id,omitempty"`
	StaffID           *uint     `json:"staff_id,omitempty" gorm:"index"`
	Token             string    `json:"token" gorm:"uniqueIndex;not null"`
	Platform          string    `json:"platform"` // "android" or "ios"
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
