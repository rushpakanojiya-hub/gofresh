package handlers

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	pickerBaseSeconds    = 40
	pickerPerItemSeconds = 12
	pickerMaxSeconds     = 900
	pickerStaleAfter     = 30 * time.Second
	pickerOrderMaxAge    = 30 * time.Minute
	pickerTaskMaxAge     = 2 * time.Hour
)

func pickerTaskCutoff() time.Time {
	return time.Now().Add(-pickerTaskMaxAge)
}

// pickerAllottedSeconds: more items -> more time (5 items = 1:40).
func pickerAllottedSeconds(qty int) int {
	s := pickerBaseSeconds + pickerPerItemSeconds*qty
	if s > pickerMaxSeconds {
		s = pickerMaxSeconds
	}
	return s
}

func pickerStatsFor(staffID uint, now time.Time) models.PickerStats {
	today := pickerToday()
	var s models.PickerStats
	if err := database.DB.Where("staff_id = ?", staffID).First(&s).Error; err != nil {
		return models.PickerStats{StaffID: staffID, StatsDate: today}
	}
	if s.StatsDate != today {
		s.StatsDate = today
		s.ActiveSeconds = 0
		s.FirstOnlineAt = nil
		if s.OnlineSince != nil {
			a := now
			b := now
			s.OnlineSince = &a
			s.FirstOnlineAt = &b
		}
	}
	return s
}

func pickerSaveStats(s *models.PickerStats) {
	database.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(s)
}

func pickerStatsOut(s models.PickerStats, now time.Time) (active int, login int) {
	active = s.ActiveSeconds
	if s.OnlineSince != nil {
		active += int(now.Sub(*s.OnlineSince).Seconds())
	}
	if s.FirstOnlineAt != nil {
		login = int(now.Sub(*s.FirstOnlineAt).Seconds())
	}
	return
}

func pickerGoOnline(staffID uint, now time.Time) {
	s := pickerStatsFor(staffID, now)
	if s.OnlineSince == nil {
		t := now
		s.OnlineSince = &t
	}
	if s.FirstOnlineAt == nil {
		t := now
		s.FirstOnlineAt = &t
	}
	pickerSaveStats(&s)
}

func pickerGoOffline(staffID uint, now time.Time) {
	s := pickerStatsFor(staffID, now)
	if s.OnlineSince != nil {
		s.ActiveSeconds += int(now.Sub(*s.OnlineSince).Seconds())
		s.OnlineSince = nil
	}
	pickerSaveStats(&s)
}

func pickerForceOffline(staffID uint) {
	now := time.Now()
	database.DB.Model(&models.PickerPresence{}).Where("staff_id = ?", staffID).Update("is_online", false)
	pickerGoOffline(staffID, now)
}

func pickerTaskOut(task models.PickingTask) gin.H {
	qty := 0
	for _, it := range task.Items {
		qty += it.QuantityNeeded
	}
	return gin.H{
		"task_id":          task.ID,
		"order_id":         task.OrderID,
		"status":           task.Status,
		"quantity":         qty,
		"item_count":       len(task.Items),
		"allotted_seconds": pickerAllottedSeconds(qty),
		"urgent":           true,
		"assigned_at":      task.CreatedAt,
		"started_at":       task.StartedAt,
	}
}

// pickerIsFirstInLine: among free, recently-seen online pickers of the warehouse,
// the one who has been online the longest gets the next order.
func pickerIsFirstInLine(staffID, warehouseID uint, now time.Time) bool {
	var cands []models.PickerPresence
	database.DB.Where("warehouse_id = ? AND is_online = ? AND workflow = ? AND updated_at > ?",
		warehouseID, true, "picker", now.Add(-pickerStaleAfter)).Find(&cands)
	if len(cands) == 0 {
		return true
	}
	ids := make([]uint, 0, len(cands))
	for _, cd := range cands {
		ids = append(ids, cd.StaffID)
	}
	var busyIDs []uint
	database.DB.Model(&models.PickingTask{}).
		Where("picker_id IN ? AND status IN ? AND created_at > ?", ids, []string{"pending", "in_progress"}, now.Add(-pickerTaskMaxAge)).
		Pluck("picker_id", &busyIDs)
	busy := map[uint]bool{}
	for _, id := range busyIDs {
		busy[id] = true
	}
	var stats []models.PickerStats
	database.DB.Where("staff_id IN ?", ids).Find(&stats)
	since := map[uint]time.Time{}
	for _, s := range stats {
		if s.OnlineSince != nil {
			since[s.StaffID] = *s.OnlineSince
		}
	}
	free := make([]models.PickerPresence, 0, len(cands))
	for _, cd := range cands {
		if !busy[cd.StaffID] {
			free = append(free, cd)
		}
	}
	if len(free) == 0 {
		return false
	}
	far := now.Add(24 * time.Hour)
	sort.SliceStable(free, func(i, j int) bool {
		ti, okI := since[free[i].StaffID]
		if !okI {
			ti = far
		}
		tj, okJ := since[free[j].StaffID]
		if !okJ {
			tj = far
		}
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return free[i].StaffID < free[j].StaffID
	})
	return free[0].StaffID == staffID
}

// pickerClaimTask gives the picker either an unowned pending picking task
// (accepted from the store panel) or the oldest confirmed order of the warehouse.
func pickerClaimTask(staffID, warehouseID uint) (uint, error) {
	var claimed uint
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var t models.PickingTask
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("warehouse_id = ? AND status = ? AND picker_id IS NULL AND created_at > ?", warehouseID, "pending", pickerTaskCutoff()).
			Order("id ASC").First(&t).Error
		if e == nil {
			t.PickerID = &staffID
			if err := tx.Save(&t).Error; err != nil {
				return err
			}
			claimed = t.ID
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}

		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("warehouse_id = ? AND status = ? AND "+
				"NOT EXISTS (SELECT 1 FROM picking_tasks WHERE picking_tasks.order_id = orders.id) AND "+
				"EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id) AND "+
				"orders.created_at > ?",
				warehouseID, models.OrderStatusConfirmed, time.Now().Add(-pickerOrderMaxAge)).
			Order("orders.id ASC").First(&order).Error; err != nil {
			return err
		}
		var items []models.OrderItem
		if err := tx.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
			return err
		}
		task := models.PickingTask{OrderID: order.ID, WarehouseID: warehouseID, PickerID: &staffID, Status: "pending"}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		for _, it := range items {
			ti := models.PickingTaskItem{
				PickingTaskID:  task.ID,
				OrderItemID:    it.ID,
				ProductID:      it.ProductID,
				QuantityNeeded: it.Quantity,
				Status:         models.PickItemPending,
			}
			if err := tx.Create(&ti).Error; err != nil {
				return err
			}
		}
		order.Status = models.OrderStatusPicking
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		claimed = task.ID
		return nil
	})
	return claimed, err
}

// GetPickerTask GET /warehouse/picker/task (polled by the picker app while online).
func GetPickerTask(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var p models.PickerPresence
	if err := database.DB.Where("staff_id = ? AND is_online = ?", staffID, true).First(&p).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"task": nil})
		return
	}
	now := time.Now()
	database.DB.Model(&models.PickerPresence{}).Where("staff_id = ?", staffID).UpdateColumn("updated_at", now)

	var mine models.PickingTask
	if err := database.DB.Preload("Items").
		Where("picker_id = ? AND status IN ? AND created_at > ?", staffID, []string{"pending", "in_progress"}, now.Add(-pickerTaskMaxAge)).
		Order("id ASC").First(&mine).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"task": pickerTaskOut(mine)})
		return
	}
	if p.Workflow != "picker" || p.WarehouseID == 0 {
		c.JSON(http.StatusOK, gin.H{"task": nil})
		return
	}
	if !pickerIsFirstInLine(staffID, p.WarehouseID, now) {
		c.JSON(http.StatusOK, gin.H{"task": nil})
		return
	}
	id, err := pickerClaimTask(staffID, p.WarehouseID)
	if err != nil || id == 0 {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("picker: claim failed: %v", err)
		}
		c.JSON(http.StatusOK, gin.H{"task": nil})
		return
	}
	var t models.PickingTask
	if err := database.DB.Preload("Items").First(&t, id).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"task": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": pickerTaskOut(t)})
}
