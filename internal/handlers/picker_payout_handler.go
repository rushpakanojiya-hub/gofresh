package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// pickerMeterBonus: highest unlocked tier amount for one day's on-time orders.
func pickerMeterBonus(onTime int) float64 {
	b := 0.0
	for _, t := range pickerMeterTiers {
		if onTime >= t.Orders {
			b = t.Amount
		}
	}
	return b
}

// GetPickerPayout GET /warehouse/picker/payout?range=day|week&date=YYYY-MM-DD
func GetPickerPayout(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Now().In(loc)
	day := now
	if d := c.Query("date"); d != "" {
		p, err := time.ParseInLocation("2006-01-02", d, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date"})
			return
		}
		day = p
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	week := c.Query("range") == "week"
	if week {
		off := (int(start.Weekday()) + 6) % 7
		start = start.AddDate(0, 0, -off)
		end = start.AddDate(0, 0, 7)
	}

	var tasks []models.PickingTask
	database.DB.Preload("Items").
		Where("picker_id = ? AND status = ? AND completed_at >= ? AND completed_at < ?", staffID, "completed", start, end).
		Find(&tasks)

	items := 0
	speedBonus := 0.0
	onTimeTotal := 0
	onTimeByDay := map[string]int{}
	for _, t := range tasks {
		qty := 0
		for _, it := range t.Items {
			qty += it.QuantityNeeded
			items += it.QuantityPicked
		}
		if t.StartedAt == nil || t.CompletedAt == nil {
			continue
		}
		actual := int(t.CompletedAt.Sub(*t.StartedAt).Seconds())
		b := pickerSpeedBonus(pickerAllottedSeconds(qty), actual)
		if b > 0 {
			onTimeTotal++
			speedBonus += b
			onTimeByDay[t.CompletedAt.In(loc).Format("2006-01-02")]++
		}
	}

	meterBonus := 0.0
	for _, n := range onTimeByDay {
		meterBonus += pickerMeterBonus(n)
	}
	itemPicking := float64(items) * pickerPayPerItem
	bonus := speedBonus + meterBonus
	penalty := 0.0

	// Active time is only stored for the current day.
	var active interface{}
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if !week && start.Equal(todayStart) {
		s := pickerStatsFor(staffID, time.Now())
		a, _ := pickerStatsOut(s, time.Now())
		active = a
	}

	c.JSON(http.StatusOK, gin.H{
		"range":            map[bool]string{true: "week", false: "day"}[week],
		"from":             start.Format("2006-01-02"),
		"to":               end.AddDate(0, 0, -1).Format("2006-01-02"),
		"total":            itemPicking + bonus - penalty,
		"item_picking":     itemPicking,
		"speed_bonus":      speedBonus,
		"meter_bonus":      meterBonus,
		"bonus":            bonus,
		"penalty":          penalty,
		"items_picked":     items,
		"orders_completed": len(tasks),
		"on_time_orders":   onTimeTotal,
		"complaints":       0,
		"active_seconds":   active,
	})
}
