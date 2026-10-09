package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	putawayMu    sync.Mutex
	putawayReady bool
)

// EnsurePutawayTables creates the putaway_tasks table on first use.
func EnsurePutawayTables() {
	putawayMu.Lock()
	defer putawayMu.Unlock()
	if putawayReady {
		return
	}
	if err := database.DB.AutoMigrate(&models.PutawayTask{}); err != nil {
		log.Printf("putaway: automigrate failed: %v", err)
		return
	}
	putawayReady = true
}

func opsFail(c *gin.Context, err error, fallback string) {
	var oe opsErr
	if errors.As(err, &oe) {
		c.JSON(oe.code, gin.H{"error": oe.msg})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
}

func opsID(c *gin.Context) (uint, bool) {
	n, err := strconv.Atoi(c.Param("id"))
	if err != nil || n <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return 0, false
	}
	return uint(n), true
}

// opsCreatePutawayTask is called after QC accepts a receiving. Idempotent.
func opsCreatePutawayTask(rec models.Receiving) {
	if rec.Status != models.ReceivingStatusAccepted {
		return
	}
	EnsurePutawayTables()
	var suggested *uint
	var inv models.Inventory
	if err := database.DB.Where("product_id = ? AND warehouse_id = ?", rec.ProductID, rec.WarehouseID).First(&inv).Error; err == nil {
		suggested = inv.BinID
	}
	t := models.PutawayTask{
		ReceivingID: rec.ID, WarehouseID: rec.WarehouseID, ProductID: rec.ProductID,
		Quantity: rec.AcceptedQuantity, SuggestedBinID: suggested, Status: models.PutawayPending,
	}
	database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&t)
}

// opsBackfillPutaway creates tasks for accepted receivings that have none.
func opsBackfillPutaway(wid uint) {
	EnsurePutawayTables()
	var recs []models.Receiving
	database.DB.Where("warehouse_id = ? AND status = ? AND id NOT IN (?)", wid, models.ReceivingStatusAccepted,
		database.DB.Model(&models.PutawayTask{}).Select("receiving_id")).Find(&recs)
	for _, r := range recs {
		opsCreatePutawayTask(r)
	}
}

// opsClosePutawayTask is called by the legacy direct put-away so its task closes too.
func opsClosePutawayTask(recID, staffID uint, binID *uint) {
	EnsurePutawayTables()
	var t models.PutawayTask
	if err := database.DB.Where("receiving_id = ? AND status <> ?", recID, models.PutawayCompleted).First(&t).Error; err != nil {
		return
	}
	wrong := t.SuggestedBinID != nil && binID != nil && *t.SuggestedBinID != *binID
	upd := map[string]interface{}{"status": models.PutawayCompleted, "completed_at": time.Now(), "wrong_location": wrong}
	if binID != nil {
		upd["actual_bin_id"] = *binID
	}
	if t.PutterID == nil {
		upd["putter_id"] = staffID
	}
	database.DB.Model(&models.PutawayTask{}).Where("id = ?", t.ID).Updates(upd)
}

func opsPendingPutaway(wid uint) int64 {
	opsBackfillPutaway(wid)
	var n int64
	database.DB.Model(&models.PutawayTask{}).Where("warehouse_id = ? AND status <> ?", wid, models.PutawayCompleted).Count(&n)
	return n
}

type opsPutawayRow struct {
	models.PutawayTask
	PutterName string `json:"putter_name"`
}

func opsPutawayPreload(db *gorm.DB) *gorm.DB {
	return db.Preload("Product").Preload("Receiving").
		Preload("SuggestedBin.Rack.Zone").Preload("ActualBin.Rack.Zone")
}

func opsPutawayRows(tasks []models.PutawayTask) []opsPutawayRow {
	ids := make([]uint, 0)
	for _, t := range tasks {
		if t.PutterID != nil {
			ids = append(ids, *t.PutterID)
		}
	}
	names := map[uint]string{}
	if len(ids) > 0 {
		var staff []models.WarehouseStaff
		database.DB.Where("id IN ?", ids).Find(&staff)
		for _, s := range staff {
			names[s.ID] = s.Name
		}
	}
	rows := make([]opsPutawayRow, 0, len(tasks))
	for _, t := range tasks {
		r := opsPutawayRow{PutawayTask: t}
		if t.PutterID != nil {
			r.PutterName = names[*t.PutterID]
		}
		rows = append(rows, r)
	}
	return rows
}

// GetPutawayTasks: GET /warehouse/putaway/tasks?status=pending|in_progress|completed (default: all open)
func GetPutawayTasks(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	opsBackfillPutaway(wid)
	since7 := opsDayStart().AddDate(0, 0, -6)

	base := func() *gorm.DB {
		return database.DB.Model(&models.PutawayTask{}).Where("warehouse_id = ?", wid)
	}
	var cPending, cInProgress, cDoneToday, cWrong int64
	base().Where("status = ?", models.PutawayPending).Count(&cPending)
	base().Where("status = ?", models.PutawayInProgress).Count(&cInProgress)
	base().Where("status = ? AND completed_at >= ?", models.PutawayCompleted, opsDayStart()).Count(&cDoneToday)
	base().Where("status = ? AND wrong_location = ? AND completed_at >= ?", models.PutawayCompleted, true, since7).Count(&cWrong)

	q := base()
	switch s := c.Query("status"); s {
	case "completed":
		q = q.Where("status = ? AND completed_at >= ?", models.PutawayCompleted, since7).Order("completed_at DESC")
	case "pending", "in_progress":
		q = q.Where("status = ?", s).Order("created_at ASC")
	default:
		q = q.Where("status <> ?", models.PutawayCompleted).Order("created_at ASC")
	}
	var tasks []models.PutawayTask
	opsPutawayPreload(q).Limit(100).Find(&tasks)

	type putterOpt struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	var staff []models.WarehouseStaff
	database.DB.Where("warehouse_id = ? AND role = ? AND is_active = ?", wid, "putter", true).Order("name ASC").Find(&staff)
	putters := make([]putterOpt, 0, len(staff))
	for _, s := range staff {
		putters = append(putters, putterOpt{ID: s.ID, Name: s.Name})
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":   opsPutawayRows(tasks),
		"putters": putters,
		"summary": gin.H{
			"pending": cPending, "in_progress": cInProgress,
			"completed_today": cDoneToday, "wrong_location_7d": cWrong,
		},
	})
}

// AssignPutawayTask: PUT /warehouse/putaway/tasks/:id/assign  {"putter_id": N}
func AssignPutawayTask(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	actorID := c.MustGet("staff_id").(uint)
	id, ok := opsID(c)
	if !ok {
		return
	}
	var req struct {
		PutterID uint `json:"putter_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	EnsurePutawayTables()

	var before, after string
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		var t models.PutawayTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", id, wid).First(&t).Error; err != nil {
			return opsErr{http.StatusNotFound, "Putaway task not found"}
		}
		if t.Status == models.PutawayCompleted {
			return opsErr{http.StatusConflict, "Task is already completed"}
		}
		var p models.WarehouseStaff
		if err := tx.Where("id = ? AND warehouse_id = ? AND role = ? AND is_active = ?", req.PutterID, wid, "putter", true).
			First(&p).Error; err != nil {
			return opsErr{http.StatusBadRequest, "Target must be an active putter of this warehouse"}
		}
		before = fmt.Sprintf("putter=%v status=%s", t.PutterID, t.Status)
		after = fmt.Sprintf("putter=%d status=pending", p.ID)
		return tx.Model(&models.PutawayTask{}).Where("id = ?", t.ID).Updates(map[string]interface{}{
			"putter_id": p.ID, "assigned_at": time.Now(), "status": models.PutawayPending, "started_at": nil,
		}).Error
	})
	if txErr != nil {
		opsFail(c, txErr, "Failed to assign task")
		return
	}
	services.LogWarehouseAction(wid, actorID, c.GetString("staff_name"), "assign_putaway", "putaway_task", strconv.Itoa(int(id)), before, after)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": id, "putter_id": req.PutterID})
}

// GetMyPutawayTasks: GET /warehouse/putaway/my-tasks
func GetMyPutawayTasks(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	EnsurePutawayTables()
	var tasks []models.PutawayTask
	opsPutawayPreload(database.DB.Where("warehouse_id = ? AND putter_id = ? AND status <> ?", wid, staffID, models.PutawayCompleted)).
		Order("created_at ASC").Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"tasks": opsPutawayRows(tasks)})
}

// StartPutawayTask: PUT /warehouse/putaway/tasks/:id/start
func StartPutawayTask(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	id, ok := opsID(c)
	if !ok {
		return
	}
	EnsurePutawayTables()
	res := database.DB.Model(&models.PutawayTask{}).
		Where("id = ? AND warehouse_id = ? AND putter_id = ? AND status = ?", id, wid, staffID, models.PutawayPending).
		Updates(map[string]interface{}{"status": models.PutawayInProgress, "started_at": time.Now()})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start task"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Task is not assigned to you or already started"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CompletePutawayTask: PUT /warehouse/putaway/tasks/:id/complete  {"bin_id": N, "note": ""}
// Adds the accepted quantity to inventory, same as the legacy put-away.
func CompletePutawayTask(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	role := c.GetString("staff_role")
	id, ok := opsID(c)
	if !ok {
		return
	}
	var req struct {
		BinID uint   `json:"bin_id" binding:"required"`
		Note  string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	EnsurePutawayTables()

	var wrong bool
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		var t models.PutawayTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", id, wid).First(&t).Error; err != nil {
			return opsErr{http.StatusNotFound, "Putaway task not found"}
		}
		if t.Status == models.PutawayCompleted {
			return opsErr{http.StatusConflict, "Task is already completed"}
		}
		if role == "putter" && (t.PutterID == nil || *t.PutterID != staffID) {
			return opsErr{http.StatusForbidden, "This task is not assigned to you"}
		}

		var bin models.WarehouseBin
		if err := tx.Joins("JOIN warehouse_racks ON warehouse_racks.id = warehouse_bins.rack_id").
			Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
			Where("warehouse_bins.id = ? AND warehouse_zones.warehouse_id = ?", req.BinID, wid).
			First(&bin).Error; err != nil {
			return opsErr{http.StatusBadRequest, "Bin not found in your warehouse"}
		}

		var rec models.Receiving
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", t.ReceivingID, wid).First(&rec).Error; err != nil {
			return opsErr{http.StatusNotFound, "Receiving record not found"}
		}
		if rec.Status != models.ReceivingStatusAccepted {
			return opsErr{http.StatusConflict, "Receiving is not in accepted state, current status: " + rec.Status}
		}

		var inv models.Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", rec.ProductID, wid).First(&inv).Error
		if err == gorm.ErrRecordNotFound {
			inv = models.Inventory{ProductID: rec.ProductID, WarehouseID: wid, Stock: 0, InStock: false}
			if err := tx.Create(&inv).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		binID := req.BinID
		prev := inv.Stock
		inv.Stock += rec.AcceptedQuantity
		inv.InStock = inv.Stock > 0
		inv.BinID = &binID
		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.StockMovement{
			ProductID: rec.ProductID, WarehouseID: wid, PreviousQty: prev, Change: rec.AcceptedQuantity,
			NewQty: inv.Stock, MovementType: models.MovementReceive, StaffID: &staffID, ReferenceID: &rec.ID,
			Notes: fmt.Sprintf("Receiving #%d from %s (putaway task #%d)", rec.ID, rec.SupplierName, t.ID),
		}).Error; err != nil {
			return err
		}

		now := time.Now()
		rec.Status = models.ReceivingStatusPutAway
		rec.PutAwayByStaffID = &staffID
		rec.PutAwayAt = &now
		rec.BinID = &binID
		if err := tx.Save(&rec).Error; err != nil {
			return err
		}

		wrong = t.SuggestedBinID != nil && *t.SuggestedBinID != binID
		upd := map[string]interface{}{
			"status": models.PutawayCompleted, "completed_at": now, "actual_bin_id": binID,
			"wrong_location": wrong, "note": req.Note,
		}
		if t.PutterID == nil {
			upd["putter_id"] = staffID
		}
		return tx.Model(&models.PutawayTask{}).Where("id = ?", t.ID).Updates(upd).Error
	})
	if txErr != nil {
		opsFail(c, txErr, "Failed to complete putaway")
		return
	}
	services.LogWarehouseAction(wid, staffID, c.GetString("staff_name"), "putaway_completed", "putaway_task",
		strconv.Itoa(int(id)), "", fmt.Sprintf("bin=%d wrong_location=%v", req.BinID, wrong))
	c.JSON(http.StatusOK, gin.H{"success": true, "wrong_location": wrong})
}

// ---- layout occupancy ----

type opsBinProd struct {
	ProductID uint   `json:"product_id"`
	Name      string `json:"name"`
	Stock     int    `json:"stock"`
}
type opsBinOcc struct {
	ID           uint         `json:"id"`
	Name         string       `json:"name"`
	ProductCount int          `json:"product_count"`
	TotalQty     int          `json:"total_qty"`
	Products     []opsBinProd `json:"products"`
}
type opsRackOcc struct {
	ID   uint        `json:"id"`
	Name string      `json:"name"`
	Bins []opsBinOcc `json:"bins"`
}
type opsZoneOcc struct {
	ID          uint         `json:"id"`
	Name        string       `json:"name"`
	Racks       []opsRackOcc `json:"racks"`
	StorageType string       `json:"storage_type"`
}

// GetLocationOccupancy: GET /warehouse/locations/occupancy
func GetLocationOccupancy(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)

	var zones []models.WarehouseZone
	database.DB.Where("warehouse_id = ?", wid).Order("name ASC").Find(&zones)
	zoneIDs := make([]uint, 0, len(zones))
	for _, z := range zones {
		zoneIDs = append(zoneIDs, z.ID)
	}
	var racks []models.WarehouseRack
	if len(zoneIDs) > 0 {
		database.DB.Where("zone_id IN ?", zoneIDs).Order("name ASC").Find(&racks)
	}
	rackIDs := make([]uint, 0, len(racks))
	for _, r := range racks {
		rackIDs = append(rackIDs, r.ID)
	}
	var bins []models.WarehouseBin
	if len(rackIDs) > 0 {
		database.DB.Where("rack_id IN ?", rackIDs).Order("name ASC").Find(&bins)
	}

	var invs []models.Inventory
	database.DB.Preload("Product").Where("warehouse_id = ? AND bin_id IS NOT NULL", wid).Find(&invs)
	byBin := map[uint][]opsBinProd{}
	for _, inv := range invs {
		if inv.BinID == nil {
			continue
		}
		byBin[*inv.BinID] = append(byBin[*inv.BinID], opsBinProd{ProductID: inv.ProductID, Name: inv.Product.Name, Stock: inv.Stock})
	}

	binsByRack := map[uint][]opsBinOcc{}
	occupied := 0
	for _, b := range bins {
		prods := byBin[b.ID]
		if prods == nil {
			prods = []opsBinProd{}
		}
		qty := 0
		for _, p := range prods {
			qty += p.Stock
		}
		if len(prods) > 0 {
			occupied++
		}
		binsByRack[b.RackID] = append(binsByRack[b.RackID], opsBinOcc{
			ID: b.ID, Name: b.Name, ProductCount: len(prods), TotalQty: qty, Products: prods,
		})
	}
	racksByZone := map[uint][]opsRackOcc{}
	for _, r := range racks {
		rb := binsByRack[r.ID]
		if rb == nil {
			rb = []opsBinOcc{}
		}
		racksByZone[r.ZoneID] = append(racksByZone[r.ZoneID], opsRackOcc{ID: r.ID, Name: r.Name, Bins: rb})
	}
	out := make([]opsZoneOcc, 0, len(zones))
	for _, z := range zones {
		zr := racksByZone[z.ID]
		if zr == nil {
			zr = []opsRackOcc{}
		}
		out = append(out, opsZoneOcc{ID: z.ID, Name: z.Name, StorageType: z.StorageType, Racks: zr})
	}

	var unassigned int64
	database.DB.Model(&models.Inventory{}).Where("warehouse_id = ? AND bin_id IS NULL AND stock > 0", wid).Count(&unassigned)

	c.JSON(http.StatusOK, gin.H{
		"zones": out,
		"summary": gin.H{
			"zones": len(zones), "racks": len(racks), "bins": len(bins),
			"occupied_bins": occupied, "unassigned_products": unassigned,
		},
	})
}
