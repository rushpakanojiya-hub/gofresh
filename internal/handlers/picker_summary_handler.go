package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// Tune these to change pay / bonus.
const pickerPayPerItem = 3.0

// pickerSpeedBonus: bonus for one order based on how much of the allotted time was left.
func pickerSpeedBonus(allotted, actual int) float64 {
	if allotted <= 0 || actual > allotted {
		return 0
	}
	saved := float64(allotted-actual) / float64(allotted)
	switch {
	case saved >= 0.5:
		return 10
	case saved >= 0.25:
		return 5
	default:
		return 2
	}
}

// Daily meter: number of on-time orders today -> extra bonus.
var pickerMeterTiers = []struct {
	Orders int
	Amount float64
}{
	{3, 25},
	{6, 50},
	{10, 75},
}

// GetPickerSummary GET /warehouse/picker/summary
func GetPickerSummary(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var tasks []models.PickingTask
	database.DB.Preload("Items").
		Where("picker_id = ? AND status = ? AND completed_at >= ?", staffID, "completed", start).
		Find(&tasks)

	items := 0
	onTime := 0
	speedBonus := 0.0
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
			onTime++
			speedBonus += b
		}
	}

	meterBonus := 0.0
	tiers := make([]gin.H, 0, len(pickerMeterTiers))
	for _, t := range pickerMeterTiers {
		unlocked := onTime >= t.Orders
		if unlocked {
			meterBonus = t.Amount
		}
		tiers = append(tiers, gin.H{"orders": t.Orders, "amount": t.Amount, "unlocked": unlocked})
	}
	bonus := speedBonus + meterBonus
	earnings := float64(items)*pickerPayPerItem + bonus

	c.JSON(http.StatusOK, gin.H{
		"items_picked":     items,
		"orders_completed": len(tasks),
		"on_time_orders":   onTime,
		"pay_per_item":     pickerPayPerItem,
		"speed_bonus":      speedBonus,
		"meter_bonus":      meterBonus,
		"bonus":            bonus,
		"earnings":         earnings,
		"complaints":       0,
		"tiers":            tiers,
	})
}
