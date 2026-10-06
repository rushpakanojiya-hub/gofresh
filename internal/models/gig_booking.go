package models

import "time"

// GigBooking is one fixed daily gig slot (e.g. key "06-07") booked by a
// delivery partner for a date (YYYY-MM-DD, IST). Cancelling deletes the row.
type GigBooking struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	DeliveryPartnerID uint      `gorm:"not null;uniqueIndex:idx_gig_partner_slot" json:"delivery_partner_id"`
	Date              string    `gorm:"size:10;not null;uniqueIndex:idx_gig_partner_slot" json:"date"`
	SlotKey           string    `gorm:"size:10;not null;uniqueIndex:idx_gig_partner_slot" json:"slot_key"`
	CreatedAt         time.Time `json:"created_at"`
}
