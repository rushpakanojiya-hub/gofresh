package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var gigIST = time.FixedZone("IST", 5*3600+30*60)
var gigMigrateOnce sync.Once

// TODO: placeholder hourly rates for the estimate shown in the app.
const (
	gigPerHourMin = 60.0
	gigPerHourMax = 90.0
)

type gigSlotDef struct{ Start, End int }

type gigGroupDef struct {
	Name  string
	Start int
	End   int
	Slots []gigSlotDef
}

var gigGroups = []gigGroupDef{
	{"Breakfast gigs", 6, 11, []gigSlotDef{{6, 7}, {7, 8}, {8, 9}, {9, 11}}},
	{"Lunch gigs", 11, 15, []gigSlotDef{{11, 13}, {13, 15}}},
	{"Evening gigs", 15, 19, []gigSlotDef{{15, 17}, {17, 19}}},
	{"Dinner gigs", 19, 24, []gigSlotDef{{19, 21}, {21, 22}, {22, 23}, {23, 24}}},
	{"Late night gigs", 0, 6, []gigSlotDef{{0, 6}}},
}

func gigEnsureTables() {
	gigMigrateOnce.Do(func() { _ = database.DB.AutoMigrate(&models.GigBooking{}) })
}

func gigKey(s gigSlotDef) string { return fmt.Sprintf("%02d-%02d", s.Start, s.End) }

func gigHour(h int) string {
	h = h % 24
	suffix := "am"
	if h >= 12 {
		suffix = "pm"
	}
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	return fmt.Sprintf("%d:00%s", h12, suffix)
}

func gigFind(key string) (gigSlotDef, bool) {
	for _, g := range gigGroups {
		for _, s := range g.Slots {
			if gigKey(s) == key {
				return s, true
			}
		}
	}
	return gigSlotDef{}, false
}

// gigResolveDate accepts only today or tomorrow (IST). Empty means today.
func gigResolveDate(q string) (time.Time, string, bool) {
	now := time.Now().In(gigIST)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, gigIST)
	if q == "" {
		q = today.Format("2006-01-02")
	}
	d, err := time.ParseInLocation("2006-01-02", q, gigIST)
	if err != nil {
		return time.Time{}, "", false
	}
	if !d.Equal(today) && !d.Equal(today.AddDate(0, 0, 1)) {
		return time.Time{}, "", false
	}
	return d, q, true
}

// GetMyGigs: GET /api/v1/delivery/gigs?date=YYYY-MM-DD
func GetMyGigs(c *gin.Context) {
	gigEnsureTables()
	partnerID := c.MustGet("user_id").(uint)
	day, date, ok := gigResolveDate(c.Query("date"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You can only view gigs for today or tomorrow"})
		return
	}
	var rows []models.GigBooking
	if err := database.DB.Where("delivery_partner_id = ? AND date = ?", partnerID, date).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load gigs"})
		return
	}
	booked := map[string]bool{}
	for _, r := range rows {
		booked[r.SlotKey] = true
	}
	now := time.Now().In(gigIST)
	groups := make([]gin.H, 0, len(gigGroups))
	bookedCount := 0
	bookedHours := 0
	for _, g := range gigGroups {
		slots := make([]gin.H, 0, len(g.Slots))
		for _, s := range g.Slots {
			key := gigKey(s)
			start := day.Add(time.Duration(s.Start) * time.Hour)
			end := day.Add(time.Duration(s.End) * time.Hour)
			isBooked := booked[key]
			if isBooked {
				bookedCount++
				bookedHours += s.End - s.Start
			}
			slots = append(slots, gin.H{
				"key":        key,
				"label":      gigHour(s.Start) + " - " + gigHour(s.End),
				"start_hour": s.Start,
				"end_hour":   s.End,
				"hours":      s.End - s.Start,
				"start_at":   start.Format(time.RFC3339),
				"end_at":     end.Format(time.RFC3339),
				"booked":     isBooked,
				"started":    !now.Before(start),
				"ended":      !now.Before(end),
			})
		}
		groups = append(groups, gin.H{
			"name":        g.Name,
			"start_label": gigHour(g.Start),
			"end_label":   gigHour(g.End),
			"slot_count":  len(g.Slots),
			"slots":       slots,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"date":         date,
		"groups":       groups,
		"booked_count": bookedCount,
		"booked_hours": bookedHours,
		"per_hour_min": gigPerHourMin,
		"per_hour_max": gigPerHourMax,
	})
}

// BookMyGigs: POST /api/v1/delivery/gigs/book  {"date": "...", "slots": ["06-07", ...]}
func BookMyGigs(c *gin.Context) {
	gigEnsureTables()
	partnerID := c.MustGet("user_id").(uint)
	var body struct {
		Date  string   `json:"date"`
		Slots []string `json:"slots"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Slots) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Select at least one gig"})
		return
	}
	day, date, ok := gigResolveDate(body.Date)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You can only book gigs for today or tomorrow"})
		return
	}
	now := time.Now().In(gigIST)
	for _, k := range body.Slots {
		s, found := gigFind(k)
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown gig slot"})
			return
		}
		end := day.Add(time.Duration(s.End) * time.Hour)
		if !now.Before(end) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Gig " + gigHour(s.Start) + " - " + gigHour(s.End) + " has already ended"})
			return
		}
	}
	for _, k := range body.Slots {
		b := models.GigBooking{DeliveryPartnerID: partnerID, Date: date, SlotKey: k}
		if err := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&b).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to book gigs"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "Gigs booked", "count": len(body.Slots)})
}

// CancelMyGig: DELETE /api/v1/delivery/gigs/book?date=YYYY-MM-DD&slot=06-07
func CancelMyGig(c *gin.Context) {
	gigEnsureTables()
	partnerID := c.MustGet("user_id").(uint)
	day, date, ok := gigResolveDate(c.Query("date"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date"})
		return
	}
	s, found := gigFind(c.Query("slot"))
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown gig slot"})
		return
	}
	start := day.Add(time.Duration(s.Start) * time.Hour)
	if !time.Now().In(gigIST).Before(start) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A gig that has started can't be cancelled"})
		return
	}
	res := database.DB.Where("delivery_partner_id = ? AND date = ? AND slot_key = ?", partnerID, date, gigKey(s)).Delete(&models.GigBooking{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel gig"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Gig cancelled"})
}
