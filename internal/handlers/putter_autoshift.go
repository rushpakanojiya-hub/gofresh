package handlers

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// putterSyncShift keeps a putter's shift in step with the booked slot:
// it ends running shifts whose slot is over (ended_at = slot end) and starts
// a shift automatically while today's booked slot is running.
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
	var b models.PutterSlotBooking
	r := database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, today, "booked").
		Order("id DESC").Limit(1).Find(&b)
	if r.Error != nil || r.RowsAffected == 0 {
		return
	}
	var slot models.PutterSlot
	if err := database.DB.First(&slot, b.SlotID).Error; err != nil {
		return
	}
	startAt, ok1 := putterSlotAt(today, slot.StartTime)
	endAt, ok2 := putterSlotAt(today, slot.EndTime)
	if !ok1 || !ok2 || now.Before(startAt) || !now.Before(endAt) {
		return
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
