package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---- settings ----

var opsDefaultSettings = map[string]string{
	"delayed_new_order_minutes":   "120",
	"delayed_in_progress_minutes": "240",
}
var opsSettingsOnce sync.Once

// EnsureOpsSettings inserts the SLA keys if missing (UpdateSettings only
// updates keys that already exist).
func EnsureOpsSettings() {
	opsSettingsOnce.Do(func() {
		for k, v := range opsDefaultSettings {
			database.DB.Where(models.Setting{Key: k}).Attrs(models.Setting{Value: v}).FirstOrCreate(&models.Setting{})
		}
	})
}

func opsMinutes(key string, def float64) time.Duration {
	EnsureOpsSettings()
	m := utils.GetSettingFloat(key, def)
	if m <= 0 {
		m = def
	}
	return time.Duration(m * float64(time.Minute))
}

func opsNewCutoff() time.Time {
	return time.Now().Add(-opsMinutes("delayed_new_order_minutes", 120))
}

func opsProgressCutoff() time.Time {
	return time.Now().Add(-opsMinutes("delayed_in_progress_minutes", 240))
}

// ---- IST day boundaries ----

var opsLoc = func() *time.Location {
	if l, err := time.LoadLocation("Asia/Kolkata"); err == nil {
		return l
	}
	return time.FixedZone("IST", 19800)
}()

func opsDayStart() time.Time {
	n := time.Now().In(opsLoc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, opsLoc)
}

func opsPeriodStart(p string) time.Time {
	d := opsDayStart()
	switch p {
	case "7d":
		return d.AddDate(0, 0, -6)
	case "30d":
		return d.AddDate(0, 0, -29)
	}
	return d
}

// ---- picking stats ----

// opsPickStats returns total units picked and total picking seconds for
// completed tasks since `since`. staffID 0 = whole warehouse.
func opsPickStats(wid uint, staffID uint, since time.Time) (int64, float64) {
	cond := "picking_tasks.warehouse_id = ? AND picking_tasks.status = 'completed' AND picking_tasks.started_at IS NOT NULL AND picking_tasks.completed_at >= ?"
	args := []interface{}{wid, since}
	if staffID > 0 {
		cond += " AND picking_tasks.picker_id = ?"
		args = append(args, staffID)
	}
	var items int64
	database.DB.Table("picking_task_items").
		Joins("JOIN picking_tasks ON picking_tasks.id = picking_task_items.picking_task_id").
		Where(cond, args...).
		Select("COALESCE(SUM(picking_task_items.quantity_picked), 0)").Scan(&items)
	var secs float64
	database.DB.Table("picking_tasks").Where(cond, args...).
		Select("COALESCE(SUM(EXTRACT(EPOCH FROM (picking_tasks.completed_at - picking_tasks.started_at))), 0)").Scan(&secs)
	return items, secs
}

func opsItemsPerHour(wid uint, staffID uint, since time.Time) float64 {
	items, secs := opsPickStats(wid, staffID, since)
	if secs <= 0 {
		return 0
	}
	return float64(items) / (secs / 3600)
}

func opsOnlinePickers(wid uint) int64 {
	var n int64
	database.DB.Model(&models.PickerPresence{}).
		Where("warehouse_id = ? AND is_online = ? AND workflow = ?", wid, true, "picker").Count(&n)
	return n
}

// ---- staff performance with period filter ----

type opsPerfRow struct {
	StaffID           uint    `json:"staff_id"`
	StaffName         string  `json:"staff_name"`
	Role              string  `json:"role"`
	OrdersPicked      int64   `json:"orders_picked"`
	OrdersPacked      int64   `json:"orders_packed"`
	AvgPickingMinutes float64 `json:"avg_picking_minutes"`
	AvgPackingMinutes float64 `json:"avg_packing_minutes"`
	TotalItemsPicked  int64   `json:"total_items_picked"`
	CleanPicks        int64   `json:"clean_picks"`
	AccuracyRate      float64 `json:"accuracy_rate"`
	ExceptionsCaused  int64   `json:"exceptions_caused"`
	ItemsPerHour      float64 `json:"items_per_hour"`
}

// GetStoreStaffPerformance: GET /warehouse/staff/performance?period=today|7d|30d
func GetStoreStaffPerformance(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	period := c.DefaultQuery("period", "today")
	if period != "7d" && period != "30d" {
		period = "today"
	}
	since := opsPeriodStart(period)

	var staff []models.WarehouseStaff
	database.DB.Where("warehouse_id = ?", wid).Find(&staff)

	rows := make([]opsPerfRow, 0, len(staff))
	for _, s := range staff {
		r := opsPerfRow{StaffID: s.ID, StaffName: s.Name, Role: s.Role}

		database.DB.Model(&models.PickingTask{}).
			Where("picker_id = ? AND warehouse_id = ? AND status = ? AND completed_at >= ?", s.ID, wid, "completed", since).
			Count(&r.OrdersPicked)
		database.DB.Model(&models.PackingTask{}).
			Where("packer_id = ? AND warehouse_id = ? AND status = ? AND completed_at >= ?", s.ID, wid, "completed", since).
			Count(&r.OrdersPacked)

		var secs float64
		database.DB.Model(&models.PickingTask{}).
			Where("picker_id = ? AND warehouse_id = ? AND status = ? AND started_at IS NOT NULL AND completed_at >= ?", s.ID, wid, "completed", since).
			Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)").Scan(&secs)
		r.AvgPickingMinutes = secs / 60

		secs = 0
		database.DB.Model(&models.PackingTask{}).
			Where("packer_id = ? AND warehouse_id = ? AND status = ? AND started_at IS NOT NULL AND completed_at >= ?", s.ID, wid, "completed", since).
			Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)").Scan(&secs)
		r.AvgPackingMinutes = secs / 60

		database.DB.Model(&models.PickingTaskItem{}).
			Joins("JOIN picking_tasks ON picking_tasks.id = picking_task_items.picking_task_id").
			Where("picking_tasks.picker_id = ? AND picking_tasks.warehouse_id = ? AND picking_tasks.created_at >= ? AND picking_task_items.status != ?", s.ID, wid, since, models.PickItemPending).
			Count(&r.TotalItemsPicked)
		database.DB.Model(&models.PickingTaskItem{}).
			Joins("JOIN picking_tasks ON picking_tasks.id = picking_task_items.picking_task_id").
			Where("picking_tasks.picker_id = ? AND picking_tasks.warehouse_id = ? AND picking_tasks.created_at >= ? AND picking_task_items.status = ?", s.ID, wid, since, models.PickItemPicked).
			Count(&r.CleanPicks)
		if r.TotalItemsPicked > 0 {
			r.AccuracyRate = float64(r.CleanPicks) / float64(r.TotalItemsPicked) * 100
		}

		database.DB.Model(&models.WarehouseException{}).
			Where("staff_id = ? AND warehouse_id = ? AND created_at >= ?", s.ID, wid, since).
			Count(&r.ExceptionsCaused)

		r.ItemsPerHour = opsItemsPerHour(wid, s.ID, since)
		rows = append(rows, r)
	}
	c.JSON(http.StatusOK, gin.H{"staff_performance": rows, "period": period, "since": since})
}

// ---- picking monitor ----

type opsTaskRow struct {
	TaskID         uint       `json:"task_id"`
	OrderID        uint       `json:"order_id"`
	PickerID       *uint      `json:"picker_id"`
	PickerName     string     `json:"picker_name"`
	PickerOnline   bool       `json:"picker_online"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at"`
	WaitingMinutes float64    `json:"waiting_minutes"`
	ItemsTotal     int        `json:"items_total"`
	ItemsDone      int        `json:"items_done"`
	Stale          bool       `json:"stale"`
	Delayed        bool       `json:"delayed"`
}

type opsPickerRow struct {
	StaffID     uint   `json:"staff_id"`
	Name        string `json:"name"`
	Online      bool   `json:"online"`
	ActiveTasks int    `json:"active_tasks"`
	State       string `json:"state"` // offline | idle | busy
}

// GetPickingMonitor: GET /warehouse/ops/picking-monitor
func GetPickingMonitor(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	now := time.Now()

	var staff []models.WarehouseStaff
	database.DB.Where("warehouse_id = ?", wid).Find(&staff)
	names := make(map[uint]string, len(staff))
	for _, s := range staff {
		names[s.ID] = s.Name
	}

	var pres []models.PickerPresence
	database.DB.Where("warehouse_id = ? AND is_online = ?", wid, true).Find(&pres)
	online := make(map[uint]bool, len(pres))
	for _, p := range pres {
		online[p.StaffID] = true
	}

	var tasks []models.PickingTask
	database.DB.Preload("Items").Preload("Order").
		Where("warehouse_id = ? AND status IN ?", wid, []string{"pending", "in_progress"}).
		Order("created_at ASC").Find(&tasks)

	progressCutoff := opsProgressCutoff()
	active := make(map[uint]int)
	rows := make([]opsTaskRow, 0, len(tasks))
	for _, t := range tasks {
		r := opsTaskRow{
			TaskID: t.ID, OrderID: t.OrderID, PickerID: t.PickerID, Status: t.Status,
			CreatedAt: t.CreatedAt, StartedAt: t.StartedAt,
			WaitingMinutes: now.Sub(t.CreatedAt).Minutes(),
			ItemsTotal:     len(t.Items),
			Stale:          t.CreatedAt.Before(now.Add(-pickerTaskMaxAge)),
			Delayed:        t.Order.UpdatedAt.Before(progressCutoff),
		}
		for _, it := range t.Items {
			if it.Status != models.PickItemPending {
				r.ItemsDone++
			}
		}
		if t.PickerID != nil {
			r.PickerName = names[*t.PickerID]
			r.PickerOnline = online[*t.PickerID]
			active[*t.PickerID]++
		}
		rows = append(rows, r)
	}

	pickers := make([]opsPickerRow, 0)
	for _, s := range staff {
		if s.Role != "picker" || !s.IsActive {
			continue
		}
		pr := opsPickerRow{StaffID: s.ID, Name: s.Name, Online: online[s.ID], ActiveTasks: active[s.ID]}
		switch {
		case !pr.Online:
			pr.State = "offline"
		case pr.ActiveTasks > 0:
			pr.State = "busy"
		default:
			pr.State = "idle"
		}
		pickers = append(pickers, pr)
	}
	c.JSON(http.StatusOK, gin.H{"tasks": rows, "pickers": pickers})
}

// ---- reassign ----

type opsErr struct {
	code int
	msg  string
}

func (e opsErr) Error() string { return e.msg }

// ReassignPickingTask: PUT /warehouse/ops/picking/:id/reassign  {"picker_id": N}
func ReassignPickingTask(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	actorID := c.MustGet("staff_id").(uint)

	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task id"})
		return
	}
	var req struct {
		PickerID uint `json:"picker_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var before, after string
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		var t models.PickingTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", taskID, wid).First(&t).Error; err != nil {
			return opsErr{http.StatusNotFound, "Picking task not found"}
		}
		if t.Status == "completed" {
			return opsErr{http.StatusConflict, "Task is already completed"}
		}
		if t.PickerID != nil && *t.PickerID == req.PickerID {
			return opsErr{http.StatusBadRequest, "Task is already assigned to this picker"}
		}

		var np models.WarehouseStaff
		if err := tx.Where("id = ? AND warehouse_id = ? AND role = ? AND is_active = ?", req.PickerID, wid, "picker", true).
			First(&np).Error; err != nil {
			return opsErr{http.StatusBadRequest, "Target must be an active picker of this warehouse"}
		}
		var npPres models.PickerPresence
		if err := tx.Where("staff_id = ? AND warehouse_id = ? AND is_online = ?", np.ID, wid, true).
			First(&npPres).Error; err != nil {
			return opsErr{http.StatusConflict, "Target picker is offline"}
		}

		upd := map[string]interface{}{"picker_id": np.ID}
		if t.Status == "in_progress" {
			if t.PickerID != nil {
				var oldPres models.PickerPresence
				if err := tx.Where("staff_id = ? AND is_online = ?", *t.PickerID, true).First(&oldPres).Error; err == nil {
					return opsErr{http.StatusConflict, "Current picker is online and working on this task"}
				}
			}
			upd["status"] = "pending"
			upd["started_at"] = nil
		}

		before = fmt.Sprintf("picker=%v status=%s", t.PickerID, t.Status)
		after = fmt.Sprintf("picker=%d status=%v", np.ID, func() interface{} {
			if v, ok := upd["status"]; ok {
				return v
			}
			return t.Status
		}())
		return tx.Model(&models.PickingTask{}).Where("id = ?", t.ID).Updates(upd).Error
	})
	if txErr != nil {
		var oe opsErr
		if errors.As(txErr, &oe) {
			c.JSON(oe.code, gin.H{"error": oe.msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reassign task"})
		return
	}

	var me models.WarehouseStaff
	database.DB.First(&me, actorID)
	services.LogWarehouseAction(wid, actorID, me.Name, "reassign_picking", "picking_task", strconv.Itoa(taskID), before, after)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": taskID, "picker_id": req.PickerID})
}
