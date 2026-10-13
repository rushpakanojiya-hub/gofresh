package handlers

import (
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var putterIdxOnce sync.Once

// putterDropOldIndex removes the old one-booking-per-day unique index (once per process).
func putterDropOldIndex() {
	putterIdxOnce.Do(func() {
		database.DB.Exec("DROP INDEX IF EXISTS idx_putter_staff_date")
	})
}

// putterMineSlots returns the slot ids the putter has booked on a date.
func putterMineSlots(staffID uint, date string) map[uint]bool {
	m := map[uint]bool{}
	var ids []uint
	database.DB.Model(&models.PutterSlotBooking{}).
		Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, date, "booked").Pluck("slot_id", &ids)
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// putterTodayBooking picks today's running booking, else the next upcoming one, else the last finished one.
func putterTodayBooking(staffID uint) (models.PutterSlotBooking, models.PutterSlot, bool) {
	var bs []models.PutterSlotBooking
	res := database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, putterSlotToday(), "booked").Order("id").Find(&bs)
	if res.Error != nil || len(bs) == 0 {
		return models.PutterSlotBooking{}, models.PutterSlot{}, false
	}
	now := time.Now()
	slots := make([]models.PutterSlot, len(bs))
	run, nxt, lst := -1, -1, -1
	var nxtStart, lstEnd time.Time
	for i, b := range bs {
		if err := database.DB.First(&slots[i], b.SlotID).Error; err != nil {
			continue
		}
		s, ok1 := putterSlotAt(b.BookingDate, slots[i].StartTime)
		e, ok2 := putterSlotAt(b.BookingDate, slots[i].EndTime)
		if !ok1 || !ok2 {
			continue
		}
		switch {
		case !now.Before(s) && now.Before(e):
			if run < 0 {
				run = i
			}
		case now.Before(s):
			if nxt < 0 || s.Before(nxtStart) {
				nxt, nxtStart = i, s
			}
		default:
			if lst < 0 || e.After(lstEnd) {
				lst, lstEnd = i, e
			}
		}
	}
	idx := run
	if idx < 0 {
		idx = nxt
	}
	if idx < 0 {
		idx = lst
	}
	if idx < 0 {
		return bs[0], slots[0], true
	}
	return bs[idx], slots[idx], true
}

// putterSyncShift keeps a putter's shift in step with the booked slots:
// it ends running shifts whose slot is over (ended_at = slot end) and starts
// a shift automatically while any of today's booked slots is running.
func putterSyncShift(staffID uint) {
	now := time.Now()

	var open []models.PutterShift
	database.DB.Where("staff_id = ? AND ended_at IS NULL", staffID).Find(&open)
	for _, s := range open {
		if s.SlotID == nil {
			continue
		}
		var slot models.PutterSlot
		if err := database.DB.First(&slot, *s.SlotID).Error; err != nil {
			continue
		}
		date := s.StartedAt.In(putterSlotLoc()).Format("2006-01-02")
		endAt, ok := putterSlotAt(date, slot.EndTime)
		if ok && now.After(endAt) {
			database.DB.Model(&models.PutterShift{}).Where("id = ? AND ended_at IS NULL", s.ID).Update("ended_at", endAt)
		}
	}

	today := putterSlotToday()
	var bs []models.PutterSlotBooking
	database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, today, "booked").Order("id").Find(&bs)
	for _, b := range bs {
		var slot models.PutterSlot
		if err := database.DB.First(&slot, b.SlotID).Error; err != nil {
			continue
		}
		startAt, ok1 := putterSlotAt(today, slot.StartTime)
		endAt, ok2 := putterSlotAt(today, slot.EndTime)
		if !ok1 || !ok2 || now.Before(startAt) || !now.Before(endAt) {
			continue
		}
		begin := startAt
		if b.CreatedAt.After(begin) {
			begin = b.CreatedAt
		}
		_ = database.DB.Transaction(func(tx *gorm.DB) error {
			var st models.WarehouseStaff
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&st, staffID).Error; err != nil {
				return err
			}
			var running, already int64
			tx.Model(&models.PutterShift{}).Where("staff_id = ? AND ended_at IS NULL", staffID).Count(&running)
			tx.Model(&models.PutterShift{}).Where("staff_id = ? AND slot_id = ? AND started_at >= ?", staffID, slot.ID, startAt.Add(-time.Hour)).Count(&already)
			if running > 0 || already > 0 {
				return nil
			}
			sid := slot.ID
			return tx.Create(&models.PutterShift{StaffID: staffID, WarehouseID: st.WarehouseID, SlotID: &sid, StartedAt: begin}).Error
		})
		return
	}
}

// putterAutoHint is the dashboard hint shown when no shift is running.
func putterAutoHint(date string, slot models.PutterSlot) string {
	startAt, ok1 := putterSlotAt(date, slot.StartTime)
	endAt, ok2 := putterSlotAt(date, slot.EndTime)
	if !ok1 || !ok2 {
		return "Slot timing is not set correctly. Contact your manager."
	}
	now := time.Now()
	if now.Before(startAt) {
		return "Your shift will start automatically at " + putterClock(startAt)
	}
	if !now.Before(endAt) {
		return "Your slot for today has ended"
	}
	return ""
}

// putterShiftEndsAt returns when the running shift's slot ends (nil if unknown).
func putterShiftEndsAt(s models.PutterShift) interface{} {
	if s.SlotID == nil {
		return nil
	}
	var slot models.PutterSlot
	if err := database.DB.First(&slot, *s.SlotID).Error; err != nil {
		return nil
	}
	endAt, ok := putterSlotAt(s.StartedAt.In(putterSlotLoc()).Format("2006-01-02"), slot.EndTime)
	if !ok {
		return nil
	}
	return endAt
}
