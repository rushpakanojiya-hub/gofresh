package handlers

import (
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

func pickerSlotWindow(b models.PickerSlotBooking) (time.Time, time.Time, bool) {
	var slot models.PickerSlot
	if err := database.DB.First(&slot, b.SlotID).Error; err != nil {
		return time.Time{}, time.Time{}, false
	}
	s, ok1 := putterSlotAt(b.BookingDate, slot.StartTime)
	e, ok2 := putterSlotAt(b.BookingDate, slot.EndTime)
	return s, e, ok1 && ok2
}

// pickerSlotGate returns "" when the booked slot is running now, else a message.
func pickerSlotGate(b models.PickerSlotBooking) string {
	s, e, ok := pickerSlotWindow(b)
	if !ok {
		return "Slot timing is not set correctly. Contact your manager."
	}
	now := time.Now()
	if now.Before(s) {
		return "Your slot starts at " + s.In(putterSlotLoc()).Format("3:04 PM")
	}
	if !now.Before(e) {
		return "Your slot for today has ended"
	}
	return ""
}

// pickerSyncPresence takes an online picker offline once the booked slot is over.
func pickerSyncPresence(staffID uint) {
	var p models.PickerPresence
	if err := database.DB.Where("staff_id = ? AND is_online = ?", staffID, true).First(&p).Error; err != nil {
		return
	}
	var b models.PickerSlotBooking
	err := database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, pickerToday(), "booked").
		Order("id DESC").First(&b).Error
	if err == nil {
		_, e, ok := pickerSlotWindow(b)
		if !ok || time.Now().Before(e) {
			return
		}
	}
	database.DB.Model(&models.PickingTask{}).
		Where("picker_id = ? AND status = ?", staffID, "pending").Update("picker_id", nil)
	pickerForceOffline(staffID)
}

func init() {
	go func() {
		t := time.NewTicker(time.Minute)
		for range t.C {
			if database.DB == nil {
				continue
			}
			var ids []uint
			database.DB.Model(&models.PickerPresence{}).Where("is_online = ?", true).Pluck("staff_id", &ids)
			for _, id := range ids {
				pickerSyncPresence(id)
			}
		}
	}()
}
