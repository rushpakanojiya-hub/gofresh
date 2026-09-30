package models

import "time"

// DeliveryNotification is an in-app notification for a delivery partner -
// distinct from the phone-based SMS-style Notification log used for
// customer-facing messages. This one tracks read/unread state per
// notification, which the rider app needs and the SMS log was never
// designed for.
type DeliveryNotification struct {
ID                uint      `json:"id" gorm:"primaryKey"`
DeliveryPartnerID uint      `json:"delivery_partner_id" gorm:"not null;index"`
Title             string    `json:"title"`
Message           string    `json:"message"`
// Type mirrors the categories already used by SendNotification
// (e.g. "new_assignment", "reassignment", "cod_settlement",
// "failed_delivery"), kept as a free string for the same reason the
// existing Notification model does - new categories shouldn't need a
// migration.
Type      string     `json:"type"`
OrderID   *uint      `json:"order_id,omitempty"`
IsRead    bool       `json:"is_read" gorm:"not null;default:false;index"`
ReadAt    *time.Time `json:"read_at,omitempty"`
CreatedAt time.Time  `json:"created_at"`
}
