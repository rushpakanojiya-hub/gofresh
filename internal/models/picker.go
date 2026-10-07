package models

import "time"

// PickerSlot is a bookable work shift at a warehouse (dark store).
type PickerSlot struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WarehouseID uint      `gorm:"not null;index" json:"warehouse_id"`
	StartTime   string    `gorm:"not null" json:"start_time"`
	EndTime     string    `gorm:"not null" json:"end_time"`
	Capacity    int       `gorm:"default:10" json:"capacity"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// PickerSlotBooking is a staff member's booking of a slot on a date (YYYY-MM-DD, IST).
type PickerSlotBooking struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	StaffID     uint      `gorm:"not null;uniqueIndex:idx_picker_staff_slot_date" json:"staff_id"`
	SlotID      uint      `gorm:"not null;uniqueIndex:idx_picker_staff_slot_date;index" json:"slot_id"`
	BookingDate string    `gorm:"column:booking_date;not null;uniqueIndex:idx_picker_staff_slot_date;index" json:"date"`
	WarehouseID uint      `gorm:"not null;index" json:"warehouse_id"`
	Status      string    `gorm:"default:booked" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// PickerPresence is the online/offline state of a staff member and the store they work at while online.
type PickerPresence struct {
	StaffID     uint      `gorm:"primaryKey;autoIncrement:false" json:"staff_id"`
	WarehouseID uint      `json:"warehouse_id"`
	IsOnline    bool      `json:"is_online"`
	Workflow    string    `json:"workflow"`
	UpdatedAt   time.Time `json:"updated_at"`
}
