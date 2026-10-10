package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetPickerHistory: GET /api/v1/warehouse/picker/history
// Picking tasks the caller completed in the last 7 days (IST), newest first.
func GetPickerHistory(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -6)

	var tasks []models.PickingTask
	database.DB.Preload("Items").
		Where("picker_id = ? AND status = ? AND completed_at >= ?", staffID, "completed", start).
		Order("completed_at DESC").Limit(100).Find(&tasks)

	orders := make([]gin.H, 0, len(tasks))
	totalItems := 0
	for _, t := range tasks {
		needed, picked := 0, 0
		for _, it := range t.Items {
			needed += it.QuantityNeeded
			picked += it.QuantityPicked
		}
		totalItems += picked
		duration := 0
		onTime := false
		if t.StartedAt != nil && t.CompletedAt != nil {
			duration = int(t.CompletedAt.Sub(*t.StartedAt).Seconds())
			onTime = pickerSpeedBonus(pickerAllottedSeconds(needed), duration) > 0
		}
		orders = append(orders, gin.H{
			"order_id":         t.OrderID,
			"completed_at":     t.CompletedAt,
			"items_needed":     needed,
			"items_picked":     picked,
			"duration_seconds": duration,
			"on_time":          onTime,
		})
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders, "total_orders": len(orders), "total_items": totalItems})
}
