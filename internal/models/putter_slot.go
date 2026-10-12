package models

import "time"

// PutterSlot is a bookable time window for putters at a warehouse (HH:MM, IST).
type PutterSlot struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WarehouseID uint      `gorm:"not null;index" json:"warehouse_id"`
	StartTime   string    `gorm:"not null" json:"start_time"`
	EndTime     string    `gorm:"not null" json:"end_time"`
	Capacity    int       `gorm:"default:10" json:"capacity"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// PutterSlotBooking is a putter's booking of a slot on a date (YYYY-MM-DD, IST). One per day.
type PutterSlotBooking struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	StaffID     uint      `gorm:"not null;uniqueIndex:idx_putter_staff_date" json:"staff_id"`
	BookingDate string    `gorm:"column:booking_date;not null;uniqueIndex:idx_putter_staff_date;index" json:"date"`
	SlotID      uint      `gorm:"not null;index" json:"slot_id"`
	WarehouseID uint      `gorm:"not null;index" json:"warehouse_id"`
	Status      string    `gorm:"default:booked" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
