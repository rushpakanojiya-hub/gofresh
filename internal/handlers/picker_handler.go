package handlers

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var pickerIST = time.FixedZone("IST", 5*3600+1800)

func pickerToday() string { return time.Now().In(pickerIST).Format("2006-01-02") }

func pickerValidDate(s string) bool {
	d, err := time.ParseInLocation("2006-01-02", s, pickerIST)
	if err != nil {
		return false
	}
	today, _ := time.ParseInLocation("2006-01-02", pickerToday(), pickerIST)
	return !d.Before(today) && !d.After(today.AddDate(0, 0, 7))
}

func pickerHaversine(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func pickerStaffID(c *gin.Context) (uint, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	switch id := v.(type) {
	case uint:
		return id, true
	case int:
		return uint(id), true
	case int64:
		return uint(id), true
	case float64:
		return uint(id), true
	}
	return 0, false
}

type pickerSlotOut struct {
	models.PickerSlot
	Booked     int  `json:"booked"`
	BookedByMe bool `json:"booked_by_me"`
}

type pickerStoreOut struct {
	models.Warehouse
	DistanceKm float64         `json:"distance_km"`
	Slots      []pickerSlotOut `json:"slots"`
}

// GetPickerStores GET /warehouse/picker/stores?lat=&lng=&date=
func GetPickerStores(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	date := c.DefaultQuery("date", pickerToday())
	if !pickerValidDate(date) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date"})
		return
	}
	lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
	lng, errLng := strconv.ParseFloat(c.Query("lng"), 64)
	hasLoc := errLat == nil && errLng == nil

	var whs []models.Warehouse
	if err := database.DB.Where("is_active = ?", true).Find(&whs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load stores"})
		return
	}
	ids := make([]uint, 0, len(whs))
	for _, w := range whs {
		ids = append(ids, w.ID)
	}

	slotsByWh := map[uint][]models.PickerSlot{}
	counts := map[uint]int{}
	mine := map[uint]bool{}
	if len(ids) > 0 {
		var slots []models.PickerSlot
		database.DB.Where("warehouse_id IN ? AND is_active = ?", ids, true).Order("start_time").Find(&slots)
		for _, s := range slots {
			slotsByWh[s.WarehouseID] = append(slotsByWh[s.WarehouseID], s)
		}
		var rows []struct {
			SlotID uint
			N      int
		}
		database.DB.Model(&models.PickerSlotBooking{}).
			Select("slot_id, COUNT(*) AS n").
			Where("booking_date = ? AND status = ?", date, "booked").
			Group("slot_id").Scan(&rows)
		for _, r := range rows {
			counts[r.SlotID] = r.N
		}
		var my []models.PickerSlotBooking
		database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, date, "booked").Find(&my)
		for _, b := range my {
			mine[b.SlotID] = true
		}
	}

	out := make([]pickerStoreOut, 0, len(whs))
	for _, w := range whs {
		so := pickerStoreOut{Warehouse: w, Slots: []pickerSlotOut{}}
		if hasLoc && (w.Lat != 0 || w.Lng != 0) {
			so.DistanceKm = math.Round(pickerHaversine(lat, lng, w.Lat, w.Lng)*10) / 10
		}
		for _, s := range slotsByWh[w.ID] {
			so.Slots = append(so.Slots, pickerSlotOut{PickerSlot: s, Booked: counts[s.ID], BookedByMe: mine[s.ID]})
		}
		out = append(out, so)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if hasLoc {
			return out[i].DistanceKm < out[j].DistanceKm
		}
		return out[i].Name < out[j].Name
	})
	c.JSON(http.StatusOK, gin.H{"date": date, "stores": out})
}

// BookPickerSlot POST /warehouse/picker/slots/:id/book  {"date":"YYYY-MM-DD"}
func BookPickerSlot(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req struct {
		Date string `json:"date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !pickerValidDate(req.Date) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date"})
		return
	}
	slotID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid slot id"})
		return
	}
	var slot models.PickerSlot
	if err := database.DB.Where("id = ? AND is_active = ?", slotID, true).First(&slot).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Slot not found"})
		return
	}
	var existing int64
	database.DB.Model(&models.PickerSlotBooking{}).
		Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, req.Date, "booked").Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "You already have a slot booked for this date"})
		return
	}
	var n int64
	database.DB.Model(&models.PickerSlotBooking{}).
		Where("slot_id = ? AND booking_date = ? AND status = ?", slot.ID, req.Date, "booked").Count(&n)
	if slot.Capacity > 0 && int(n) >= slot.Capacity {
		c.JSON(http.StatusConflict, gin.H{"error": "Slot is full"})
		return
	}
	b := models.PickerSlotBooking{StaffID: staffID, SlotID: slot.ID, BookingDate: req.Date, WarehouseID: slot.WarehouseID, Status: "booked"}
	if err := database.DB.Create(&b).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Already booked"})
		return
	}
	c.JSON(http.StatusCreated, b)
}

type pickerBookingOut struct {
	ID            uint   `json:"id"`
	Date          string `json:"date"`
	SlotID        uint   `json:"slot_id"`
	WarehouseID   uint   `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

// GetMyPickerBookings GET /warehouse/picker/bookings/me
func GetMyPickerBookings(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var bookings []models.PickerSlotBooking
	database.DB.Where("staff_id = ? AND booking_date >= ? AND status = ?", staffID, pickerToday(), "booked").
		Order("booking_date, id").Find(&bookings)

	slotIDs := make([]uint, 0, len(bookings))
	whIDs := make([]uint, 0, len(bookings))
	for _, b := range bookings {
		slotIDs = append(slotIDs, b.SlotID)
		whIDs = append(whIDs, b.WarehouseID)
	}
	slotMap := map[uint]models.PickerSlot{}
	whMap := map[uint]models.Warehouse{}
	if len(bookings) > 0 {
		var slots []models.PickerSlot
		database.DB.Where("id IN ?", slotIDs).Find(&slots)
		for _, s := range slots {
			slotMap[s.ID] = s
		}
		var whs []models.Warehouse
		database.DB.Where("id IN ?", whIDs).Find(&whs)
		for _, w := range whs {
			whMap[w.ID] = w
		}
	}
	out := make([]pickerBookingOut, 0, len(bookings))
	for _, b := range bookings {
		s := slotMap[b.SlotID]
		out = append(out, pickerBookingOut{
			ID: b.ID, Date: b.BookingDate, SlotID: b.SlotID, WarehouseID: b.WarehouseID,
			WarehouseName: whMap[b.WarehouseID].Name, StartTime: s.StartTime, EndTime: s.EndTime,
		})
	}
	c.JSON(http.StatusOK, gin.H{"bookings": out})
}

// CancelPickerBooking DELETE /warehouse/picker/bookings/:id
func CancelPickerBooking(c *gin.Context) {
	database.EnsurePickerTables()
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
	res := database.DB.Where("id = ? AND staff_id = ?", id, staffID).Delete(&models.PickerSlotBooking{})
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}
	// No booking left today -> force offline.
	var left int64
	database.DB.Model(&models.PickerSlotBooking{}).
		Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, pickerToday(), "booked").Count(&left)
	if left == 0 {
		database.DB.Model(&models.PickerPresence{}).Where("staff_id = ?", staffID).Update("is_online", false)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Booking cancelled"})
}

func pickerPresenceResponse(p models.PickerPresence) gin.H {
	name := ""
	if p.WarehouseID != 0 {
		var w models.Warehouse
		if err := database.DB.Select("id", "name").First(&w, p.WarehouseID).Error; err == nil {
			name = w.Name
		}
	}
	return gin.H{"is_online": p.IsOnline, "workflow": p.Workflow, "warehouse_id": p.WarehouseID, "warehouse_name": name}
}

// GetPickerPresence GET /warehouse/picker/presence
func GetPickerPresence(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var p models.PickerPresence
	if err := database.DB.Where("staff_id = ?", staffID).First(&p).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"is_online": false, "workflow": "", "warehouse_id": 0, "warehouse_name": ""})
		return
	}
	c.JSON(http.StatusOK, pickerPresenceResponse(p))
}

// SetPickerPresence PUT /warehouse/picker/presence {"online":true,"workflow":"picker"}
func SetPickerPresence(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req struct {
		Online   bool   `json:"online"`
		Workflow string `json:"workflow"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wf := req.Workflow
	if wf != "picker" && wf != "packer" {
		wf = "picker"
	}
	p := models.PickerPresence{StaffID: staffID, IsOnline: req.Online, Workflow: wf}
	if req.Online {
		var b models.PickerSlotBooking
		if err := database.DB.Where("staff_id = ? AND booking_date = ? AND status = ?", staffID, pickerToday(), "booked").
			Order("id DESC").First(&b).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Book a slot for today first"})
			return
		}
		p.WarehouseID = b.WarehouseID
	}
	if err := database.DB.Save(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}
	c.JSON(http.StatusOK, pickerPresenceResponse(p))
}

// GetPickerRoster GET /warehouse/picker-roster (store panel): today's slot bookings + online pickers of this warehouse.
func GetPickerRoster(c *gin.Context) {
	database.EnsurePickerTables()
	wid := c.MustGet("warehouse_id").(uint)
	date := c.DefaultQuery("date", pickerToday())

	var bookings []models.PickerSlotBooking
	database.DB.Where("warehouse_id = ? AND booking_date = ? AND status = ?", wid, date, "booked").
		Order("slot_id, id").Find(&bookings)
	var presences []models.PickerPresence
	database.DB.Where("warehouse_id = ? AND is_online = ?", wid, true).Find(&presences)

	staffSet := map[uint]bool{}
	slotSet := map[uint]bool{}
	for _, b := range bookings {
		staffSet[b.StaffID] = true
		slotSet[b.SlotID] = true
	}
	for _, p := range presences {
		staffSet[p.StaffID] = true
	}
	staffMap := map[uint]models.WarehouseStaff{}
	if len(staffSet) > 0 {
		ids := make([]uint, 0, len(staffSet))
		for id := range staffSet {
			ids = append(ids, id)
		}
		var staff []models.WarehouseStaff
		database.DB.Where("id IN ?", ids).Find(&staff)
		for _, s := range staff {
			staffMap[s.ID] = s
		}
	}
	slotMap := map[uint]models.PickerSlot{}
	if len(slotSet) > 0 {
		ids := make([]uint, 0, len(slotSet))
		for id := range slotSet {
			ids = append(ids, id)
		}
		var slots []models.PickerSlot
		database.DB.Where("id IN ?", ids).Find(&slots)
		for _, s := range slots {
			slotMap[s.ID] = s
		}
	}

	bookedOut := make([]gin.H, 0, len(bookings))
	for _, b := range bookings {
		s := staffMap[b.StaffID]
		sl := slotMap[b.SlotID]
		bookedOut = append(bookedOut, gin.H{
			"staff_id": b.StaffID, "name": s.Name, "phone": s.Phone,
			"slot_id": b.SlotID, "start_time": sl.StartTime, "end_time": sl.EndTime,
		})
	}
	onlineOut := make([]gin.H, 0, len(presences))
	for _, p := range presences {
		s := staffMap[p.StaffID]
		onlineOut = append(onlineOut, gin.H{
			"staff_id": p.StaffID, "name": s.Name, "phone": s.Phone,
			"workflow": p.Workflow, "updated_at": p.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"date": date, "bookings": bookedOut, "online": onlineOut})
}
