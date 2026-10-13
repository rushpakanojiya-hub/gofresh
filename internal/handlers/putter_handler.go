package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

const putterEarlyStart = 15 * time.Minute

var (
	putterTablesOnce sync.Once
	putterSeedMu     sync.Mutex
)

func ensurePutterTables() {
	putterTablesOnce.Do(func() {
		_ = database.DB.AutoMigrate(&models.PutterShift{}, &models.PutterSlot{}, &models.PutterSlotBooking{})
	})
}

// PutterRoleOnly lets only active warehouse staff with role "putter" use the
// putter app routes. Managers/pickers get 403.
func PutterRoleOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID, ok := pickerStaffID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		var st models.WarehouseStaff
		if err := database.DB.Select("id", "role", "is_active").First(&st, staffID).Error; err != nil || st.Role != "putter" || !st.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Only putter accounts can use the putter app"})
			return
		}
		c.Next()
	}
}

// ---------------- time helpers (all IST) ----------------

func putterSlotLoc() *time.Location { return time.FixedZone("IST", 5*3600+1800) }

func putterSlotToday() string { return time.Now().In(putterSlotLoc()).Format("2006-01-02") }

func putterClock(t time.Time) string { return t.In(putterSlotLoc()).Format("3:04 PM") }

func putterSlotDateOK(s string) bool {
	loc := putterSlotLoc()
	d, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return false
	}
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return !d.Before(start) && !d.After(start.AddDate(0, 0, 7))
}

func putterSlotAt(date, hhmm string) (time.Time, bool) {
	p := strings.Split(hhmm, ":")
	if len(p) < 2 {
		return time.Time{}, false
	}
	h, e1 := strconv.Atoi(strings.TrimSpace(p[0]))
	m, e2 := strconv.Atoi(strings.TrimSpace(p[1]))
	if e1 != nil || e2 != nil {
		return time.Time{}, false
	}
	loc := putterSlotLoc()
	d, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, loc), true
}

// putterShiftGate says whether a shift may start now for a booked slot.
func putterShiftGate(date string, slot models.PutterSlot) (bool, string) {
	startAt, ok1 := putterSlotAt(date, slot.StartTime)
	endAt, ok2 := putterSlotAt(date, slot.EndTime)
	if !ok1 || !ok2 {
		return false, "Slot timing is not set correctly. Contact your manager."
	}
	now := time.Now()
	opens := startAt.Add(-putterEarlyStart)
	if now.Before(opens) {
		return false, "Your shift opens at " + putterClock(opens)
	}
	if now.After(endAt) {
		return false, "Your slot for today has ended"
	}
	return true, ""
}

// ensurePutterSlots creates slots for a warehouse the first time they are needed,
// copying the picker slot timings (or sensible defaults).
func ensurePutterSlots(wid uint) {
	putterSeedMu.Lock()
	defer putterSeedMu.Unlock()
	var n int64
	database.DB.Model(&models.PutterSlot{}).Where("warehouse_id = ?", wid).Count(&n)
	if n > 0 {
		return
	}
	database.EnsurePickerTables()
	var ps []models.PickerSlot
	database.DB.Where("warehouse_id = ? AND is_active = ?", wid, true).Order("start_time").Find(&ps)
	if len(ps) > 0 {
		for _, s := range ps {
			capacity := s.Capacity
			if capacity <= 0 {
				capacity = 10
			}
			database.DB.Create(&models.PutterSlot{WarehouseID: wid, StartTime: s.StartTime, EndTime: s.EndTime, Capacity: capacity, IsActive: true})
		}
		return
	}
	for _, t := range [][2]string{{"06:00", "10:00"}, {"10:00", "14:00"}, {"14:00", "18:00"}, {"18:00", "22:00"}} {
		database.DB.Create(&models.PutterSlot{WarehouseID: wid, StartTime: t[0], EndTime: t[1], Capacity: 10, IsActive: true})
	}
}

func putterLoadStaff(c *gin.Context) (models.WarehouseStaff, bool) {
	var st models.WarehouseStaff
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return st, false
	}
	if err := database.DB.Preload("Warehouse").First(&st, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Staff not found"})
		return st, false
	}
	return st, true
}

func putterActiveShift(staffID uint) (models.PutterShift, bool) {
	putterSyncShift(staffID)
	var s models.PutterShift
	res := database.DB.Where("staff_id = ? AND ended_at IS NULL", staffID).Order("id DESC").Limit(1).Find(&s)
	return s, res.Error == nil && res.RowsAffected > 0
}

func putterTodayBooking(staffID uint) (models.PutterSlotBooking, models.PutterSlot, bool) {
	var b models.PutterSlotBooking
	res := database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, putterSlotToday(), "booked").
		Order("id DESC").Limit(1).Find(&b)
	if res.Error != nil || res.RowsAffected == 0 {
		return b, models.PutterSlot{}, false
	}
	var s models.PutterSlot
	if err := database.DB.First(&s, b.SlotID).Error; err != nil {
		return b, models.PutterSlot{}, false
	}
	return b, s, true
}

// ---------------- dashboard + shift ----------------

// GetPutterDashboard GET /warehouse/putter/dashboard
func GetPutterDashboard(c *gin.Context) {
	ensurePutterTables()
	EnsurePutawayTables()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	staffID := staff.ID
	putterSyncShift(staff.ID)
	wid := staff.WarehouseID
	loc := putterSlotLoc()
	now := time.Now().In(loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	mine := func() *gorm.DB {
		return database.DB.Where("warehouse_id = ? AND putter_id = ?", wid, staffID)
	}
	var pending, inProgress, done []models.PutawayTask
	opsPutawayPreload(mine().Where("status = ?", models.PutawayPending)).Order("created_at ASC").Limit(100).Find(&pending)
	opsPutawayPreload(mine().Where("status = ?", models.PutawayInProgress)).Order("started_at ASC").Limit(100).Find(&inProgress)
	opsPutawayPreload(mine().Where("status = ? AND completed_at >= ?", models.PutawayCompleted, dayStart)).Order("completed_at DESC").Limit(100).Find(&done)

	var shifts []models.PutterShift
	database.DB.Where("staff_id = ? AND started_at >= ?", staffID, dayStart).Find(&shifts)
	total := 0
	for _, s := range shifts {
		end := time.Now()
		if s.EndedAt != nil {
			end = *s.EndedAt
		}
		total += int(end.Sub(s.StartedAt).Seconds())
	}

	active, hasActive := putterActiveShift(staffID)
	b, bslot, hasB := putterTodayBooking(staffID)
	canStart, hint, slotOver := false, "Book a slot for today. Your shift will start and end automatically with the slot", false
	var booking interface{}
	if hasB {
		booking = gin.H{"id": b.ID, "slot_id": b.SlotID, "date": b.BookingDate, "start_time": bslot.StartTime, "end_time": bslot.EndTime}
		canStart, hint = false, putterAutoHint(b.BookingDate, bslot)
		if endAt, okEnd := putterSlotAt(b.BookingDate, bslot.EndTime); okEnd && time.Now().After(endAt) {
			slotOver = true
		}
	}
	if hasActive {
		canStart, hint = false, ""
	}
	shift := gin.H{"active": hasActive, "today_seconds": total, "started_at": nil, "can_start": canStart, "hint": hint, "slot_over": slotOver}
	if hasActive {
		shift["started_at"] = active.StartedAt
		shift["ends_at"] = putterShiftEndsAt(active)
	}

	var movesToday int64
	database.DB.Model(&models.StockMovement{}).
		Where("warehouse_id = ? AND staff_id = ? AND movement_type = ? AND reason = ? AND created_at >= ?",
			wid, staffID, models.MovementTransfer, "bin_move", dayStart).Count(&movesToday)
	var reportsToday int64
	database.DB.Model(&models.PutterPutawayReport{}).
		Where("warehouse_id = ? AND staff_id = ? AND created_at >= ?", wid, staffID, dayStart).Count(&reportsToday)
	unitsPlaced := 0
	for _, t := range done {
		unitsPlaced += t.Quantity
	}
	productivity := gin.H{
		"putaway_done":  len(done),
		"units_placed":  unitsPlaced,
		"moves":         movesToday,
		"reports":       reportsToday,
		"shift_seconds": total,
	}

	c.JSON(http.StatusOK, gin.H{
		"staff": gin.H{
			"id":          staff.ID,
			"name":        staff.Name,
			"phone":       staff.Phone,
			"employee_id": fmt.Sprintf("PUT-%04d", staff.ID),
		},
		"store":         gin.H{"name": staff.Warehouse.Name, "city": staff.Warehouse.City},
		"shift":         shift,
		"today_booking": booking,
		"counts":        gin.H{"pending": len(pending), "in_progress": len(inProgress), "completed_today": len(done)},
		"productivity":  productivity,
		"pending":       opsPutawayRows(pending),
		"in_progress":   opsPutawayRows(inProgress),
		"completed":     opsPutawayRows(done),
	})
}

// StartPutterShift POST /warehouse/putter/shift/start (needs today's booked slot)
func StartPutterShift(c *gin.Context) {
	ensurePutterTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	code := http.StatusInternalServerError
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var st models.WarehouseStaff
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&st, staffID).Error; e != nil {
			code = http.StatusNotFound
			return errors.New("staff not found")
		}
		var n int64
		tx.Model(&models.PutterShift{}).Where("staff_id = ? AND ended_at IS NULL", staffID).Count(&n)
		if n > 0 {
			code = http.StatusConflict
			return errors.New("Your shift is already running")
		}
		var b models.PutterSlotBooking
		r := tx.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, putterSlotToday(), "booked").
			Order("id DESC").Limit(1).Find(&b)
		if r.Error != nil || r.RowsAffected == 0 {
			code = http.StatusBadRequest
			return errors.New("Book a slot for today first")
		}
		var slot models.PutterSlot
		if e := tx.First(&slot, b.SlotID).Error; e != nil {
			code = http.StatusNotFound
			return errors.New("Your booked slot no longer exists")
		}
		if allowed, msg := putterShiftGate(b.BookingDate, slot); !allowed {
			code = http.StatusConflict
			return errors.New(msg)
		}
		sid := slot.ID
		return tx.Create(&models.PutterShift{StaffID: staffID, WarehouseID: st.WarehouseID, SlotID: &sid, StartedAt: time.Now()}).Error
	})
	if err != nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// EndPutterShift POST /warehouse/putter/shift/end
func EndPutterShift(c *gin.Context) {
	ensurePutterTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	res := database.DB.Model(&models.PutterShift{}).
		Where("staff_id = ? AND ended_at IS NULL", staffID).
		Update("ended_at", time.Now())
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not end the shift"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "No running shift to end"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------------- slots ----------------

// GetPutterSlots GET /warehouse/putter/slots?date=YYYY-MM-DD
func GetPutterSlots(c *gin.Context) {
	ensurePutterTables()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	date := c.DefaultQuery("date", putterSlotToday())
	if !putterSlotDateOK(date) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date"})
		return
	}
	wid := staff.WarehouseID
	ensurePutterSlots(wid)

	var slots []models.PutterSlot
	database.DB.Where("warehouse_id = ? AND is_active = ?", wid, true).Order("start_time").Find(&slots)

	var rows []struct {
		SlotID uint
		N      int
	}
	database.DB.Model(&models.PutterSlotBooking{}).
		Select("slot_id, COUNT(*) AS n").
		Where("warehouse_id = ? AND booking_date = ? AND status = ?", wid, date, "booked").
		Group("slot_id").Scan(&rows)
	counts := map[uint]int{}
	for _, r := range rows {
		counts[r.SlotID] = r.N
	}
	mineSlot := map[uint]bool{}
	var my models.PutterSlotBooking
	res := database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staff.ID, date, "booked").Limit(1).Find(&my)
	if res.Error == nil && res.RowsAffected > 0 {
		mineSlot[my.SlotID] = true
	}

	now := time.Now()
	out := make([]gin.H, 0, len(slots))
	for _, s := range slots {
		endAt, okEnd := putterSlotAt(date, s.EndTime)
		out = append(out, gin.H{
			"id":           s.ID,
			"start_time":   s.StartTime,
			"end_time":     s.EndTime,
			"capacity":     s.Capacity,
			"booked":       counts[s.ID],
			"booked_by_me": mineSlot[s.ID],
			"ended":        okEnd && now.After(endAt),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"date":  date,
		"store": gin.H{"id": staff.WarehouseID, "name": staff.Warehouse.Name, "city": staff.Warehouse.City},
		"slots": out,
	})
}

// BookPutterSlot POST /warehouse/putter/slots/:id/book {"date":"YYYY-MM-DD"}
func BookPutterSlot(c *gin.Context) {
	ensurePutterTables()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	var req struct {
		Date string `json:"date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Date is required"})
		return
	}
	if !putterSlotDateOK(req.Date) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date"})
		return
	}
	if req.Date != putterSlotToday() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You can only book today's slot"})
		return
	}
	slotID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid slot id"})
		return
	}
	code := http.StatusInternalServerError
	var created models.PutterSlotBooking
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var slot models.PutterSlot
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_active = ? AND warehouse_id = ?", slotID, true, staff.WarehouseID).
			First(&slot).Error; e != nil {
			code = http.StatusNotFound
			return errors.New("Slot not found")
		}
		if endAt, okEnd := putterSlotAt(req.Date, slot.EndTime); okEnd && time.Now().After(endAt) {
			code = http.StatusConflict
			return errors.New("This slot is already over")
		}
		var existing int64
		tx.Model(&models.PutterSlotBooking{}).
			Where("staff_id = ? AND booking_date = ? AND status = ?", staff.ID, req.Date, "booked").Count(&existing)
		if existing > 0 {
			code = http.StatusConflict
			return errors.New("You already have a slot booked for this date")
		}
		var n int64
		tx.Model(&models.PutterSlotBooking{}).
			Where("slot_id = ? AND booking_date = ? AND status = ?", slot.ID, req.Date, "booked").Count(&n)
		if slot.Capacity > 0 && int(n) >= slot.Capacity {
			code = http.StatusConflict
			return errors.New("Slot is full")
		}
		created = models.PutterSlotBooking{StaffID: staff.ID, SlotID: slot.ID, BookingDate: req.Date, WarehouseID: slot.WarehouseID, Status: "booked"}
		if e := tx.Create(&created).Error; e != nil {
			code = http.StatusConflict
			return errors.New("Already booked")
		}
		return nil
	})
	if err != nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

// GetMyPutterBookings GET /warehouse/putter/bookings/me (today onwards)
func GetMyPutterBookings(c *gin.Context) {
	ensurePutterTables()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	var bookings []models.PutterSlotBooking
	database.DB.Where("staff_id = ? AND booking_date >= ? AND status = ?", staff.ID, putterSlotToday(), "booked").
		Order("booking_date, id").Find(&bookings)
	slotIDs := make([]uint, 0, len(bookings))
	for _, b := range bookings {
		slotIDs = append(slotIDs, b.SlotID)
	}
	slotMap := map[uint]models.PutterSlot{}
	if len(slotIDs) > 0 {
		var slots []models.PutterSlot
		database.DB.Where("id IN ?", slotIDs).Find(&slots)
		for _, s := range slots {
			slotMap[s.ID] = s
		}
	}
	out := make([]gin.H, 0, len(bookings))
	for _, b := range bookings {
		s := slotMap[b.SlotID]
		out = append(out, gin.H{
			"id": b.ID, "date": b.BookingDate, "slot_id": b.SlotID,
			"start_time": s.StartTime, "end_time": s.EndTime, "store_name": staff.Warehouse.Name,
		})
	}
	c.JSON(http.StatusOK, gin.H{"bookings": out})
}

// CancelPutterBooking DELETE /warehouse/putter/bookings/:id
func CancelPutterBooking(c *gin.Context) {
	ensurePutterTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking id"})
		return
	}
	var b models.PutterSlotBooking
	res := database.DB.Where("id = ? AND staff_id = ?", id, staffID).Limit(1).Find(&b)
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}
	if b.BookingDate == putterSlotToday() {
		if _, running := putterActiveShift(staffID); running {
			c.JSON(http.StatusConflict, gin.H{"error": "End your running shift before cancelling today's slot"})
			return
		}
	}
	if err := database.DB.Delete(&models.PutterSlotBooking{}, b.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not cancel the booking"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Booking cancelled"})
}
