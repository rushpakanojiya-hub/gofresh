package handlers

import (
	"sync"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

type pickerWin struct {
	id uint
	wh uint
	s  time.Time
	e  time.Time
}

// pickerAutoDone remembers bookings that were already auto-started, so a picker
// who goes offline by hand during a slot is not put back online again.
var pickerAutoDone sync.Map

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
			out = append(out, pickerWin{id: b.ID, wh: b.WarehouseID, s: s, e: e})
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

// pickerSyncPresence keeps a picker's online state in step with the booked slots:
// offline when no booked slot is running, and online automatically when one starts.
func pickerSyncPresence(staffID uint) {
	now := time.Now()
	ws := pickerTodayWindows(staffID)
	var running *pickerWin
	for i := range ws {
		if !now.Before(ws[i].s) && now.Before(ws[i].e) {
			running = &ws[i]
			break
		}
	}

	var p models.PickerPresence
	found := database.DB.Where("staff_id = ?", staffID).First(&p).Error == nil
	if found && p.IsOnline {
		if running == nil {
			database.DB.Model(&models.PickingTask{}).
				Where("picker_id = ? AND status = ?", staffID, "pending").Update("picker_id", nil)
			pickerForceOffline(staffID)
		}
		return
	}
	if running == nil {
		return
	}
	if _, done := pickerAutoDone.LoadOrStore(running.id, true); done {
		return
	}
	p.StaffID = staffID
	if p.Workflow == "" {
		p.Workflow = "picker"
	}
	p.WarehouseID = running.wh
	p.IsOnline = true
	if err := database.DB.Save(&p).Error; err != nil {
		pickerAutoDone.Delete(running.id)
		return
	}
	pickerGoOnline(staffID, now)
}

func init() {
	go func() {
		t := time.NewTicker(time.Minute)
		for range t.C {
			if database.DB == nil {
				continue
			}
			seen := map[uint]bool{}
			var online []uint
			database.DB.Model(&models.PickerPresence{}).Where("is_online = ?", true).Pluck("staff_id", &online)
			var booked []uint
			database.DB.Model(&models.PickerSlotBooking{}).
				Where("booking_date = ? AND status = ?", pickerToday(), "booked").
				Distinct("staff_id").Pluck("staff_id", &booked)
			for _, id := range append(online, booked...) {
				if seen[id] {
					continue
				}
				seen[id] = true
				pickerSyncPresence(id)
			}
		}
	}()
}
