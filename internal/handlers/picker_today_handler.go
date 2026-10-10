package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetPickerToday: GET /api/v1/warehouse/picker/today
// All picking tasks of the caller's warehouse created today (IST), split into pending and completed.
func GetPickerToday(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var p models.PickerPresence
	wid := uint(0)
	if err := database.DB.Where("staff_id = ?", staffID).Order("updated_at DESC").First(&p).Error; err == nil {
		wid = uint(p.WarehouseID)
	}
	pending := []gin.H{}
	completed := []gin.H{}
	if wid == 0 {
		c.JSON(http.StatusOK, gin.H{"pending": pending, "completed": completed})
		return
	}
	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var tasks []models.PickingTask
	database.DB.Preload("Items").
		Where("warehouse_id = ? AND created_at >= ?", wid, start).
		Order("id ASC").Limit(300).Find(&tasks)

	for _, t := range tasks {
		needed, picked := 0, 0
		for _, it := range t.Items {
			needed += it.QuantityNeeded
			picked += it.QuantityPicked
		}
		row := gin.H{
			"task_id":      t.ID,
			"order_id":     t.OrderID,
			"status":       t.Status,
			"items_needed": needed,
			"items_picked": picked,
			"created_at":   t.CreatedAt,
			"completed_at": t.CompletedAt,
		}
		switch t.Status {
		case "completed":
			completed = append(completed, row)
		case "pending", "in_progress":
			pending = append(pending, row)
		}
	}
	c.JSON(http.StatusOK, gin.H{"pending": pending, "completed": completed})
}
