package handlers

import (
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

type pickerWin struct{ s, e time.Time }

// pickerTodayWindows returns the time windows of all of today's booked slots.
func pickerTodayWindows(staffID uint) []pickerWin {
	var bs []models.PickerSlotBooking
	database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, pickerToday(), "booked").Order("id").Find(&bs)
	out := []pickerWin{}
	for _, b := range bs {
		var slot models.PickerSlot
		if err := database.DB.First(&slot, b.SlotID).Error; err != nil {
			continue
		}
		s, ok1 := putterSlotAt(b.BookingDate, slot.StartTime)
		e, ok2 := putterSlotAt(b.BookingDate, slot.EndTime)
		if ok1 && ok2 {
			out = append(out, pickerWin{s, e})
		}
	}
	return out
}

// pickerSlotGate returns "" when any of today's booked slots is running now, else a message.
func pickerSlotGate(b models.PickerSlotBooking) string {
	ws := pickerTodayWindows(b.StaffID)
	if len(ws) == 0 {
		return "Slot timing is not set correctly. Contact your manager."
	}
	now := time.Now()
	var next *time.Time
	for _, w := range ws {
		if !now.Before(w.s) && now.Before(w.e) {
			return ""
		}
		if now.Before(w.s) && (next == nil || w.s.Before(*next)) {
			t := w.s
			next = &t
		}
	}
	if next != nil {
		return "Your next slot starts at " + next.In(putterSlotLoc()).Format("3:04 PM")
	}
	return "Your slot for today has ended"
}

// pickerSyncPresence takes an online picker offline when no booked slot is running.
func pickerSyncPresence(staffID uint) {
	var p models.PickerPresence
	if err := database.DB.Where("staff_id = ? AND is_online = ?", staffID, true).First(&p).Error; err != nil {
		return
	}
	now := time.Now()
	for _, w := range pickerTodayWindows(staffID) {
		if !now.Before(w.s) && now.Before(w.e) {
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
