package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var putterReportOnce database.MigrateOnce

func ensurePutterReportTable() {
	putterReportOnce.Migrate(&models.PutterPutawayReport{})
}

func putterBinLabel(b models.WarehouseBin) string {
	parts := []string{}
	for _, p := range []string{b.Rack.Zone.Name, b.Rack.Name, b.Name} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, strings.TrimSpace(p))
		}
	}
	return strings.Join(parts, " - ")
}

// putterLoadTask loads a putaway task that belongs to this putter.
func putterLoadTask(c *gin.Context, staff models.WarehouseStaff) (models.PutawayTask, bool) {
	var t models.PutawayTask
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task id"})
		return t, false
	}
	res := opsPutawayPreload(database.DB.Where("id = ? AND warehouse_id = ? AND putter_id = ?", id, staff.WarehouseID, staff.ID)).
		Limit(1).Find(&t)
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return t, false
	}
	return t, true
}

// GetPutterTask GET /warehouse/putter/tasks/:id
func GetPutterTask(c *gin.Context) {
	EnsurePutawayTables()
	ensurePutterReportTable()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	t, ok := putterLoadTask(c, staff)
	if !ok {
		return
	}
	var batches []models.Batch
	database.DB.Where("receiving_id = ?", t.ReceivingID).Order("expiry_date ASC").Find(&batches)
	bl := make([]gin.H, 0, len(batches))
	for _, b := range batches {
		bl = append(bl, gin.H{
			"id":               b.ID,
			"batch_number":     b.BatchNumber,
			"expiry_date":      b.ExpiryDate,
			"manufacture_date": b.ManufactureDate,
			"quantity":         b.Quantity,
			"days_left":        int(time.Until(b.ExpiryDate).Hours() / 24),
		})
	}
	var reports []models.PutterPutawayReport
	database.DB.Where("task_id = ?", t.ID).Order("id DESC").Find(&reports)
	rec := t.Receiving
	sbl := ""
	if t.SuggestedBin != nil {
		sbl = putterBinLabel(*t.SuggestedBin)
	}
	c.JSON(http.StatusOK, gin.H{
		"task": opsPutawayRows([]models.PutawayTask{t})[0],
		"grn": gin.H{
			"id":                rec.ID,
			"reference_number":  rec.ReferenceNumber,
			"supplier_name":     rec.SupplierName,
			"expected_quantity": rec.ExpectedQuantity,
			"received_quantity": rec.ReceivedQuantity,
			"damaged_quantity":  rec.DamagedQuantity,
			"accepted_quantity": rec.AcceptedQuantity,
			"status":            rec.Status,
			"received_at":       rec.ReceivedAt,
			"qc_at":             rec.QCAt,
		},
		"batches":             bl,
		"reports":             reports,
		"suggested_bin_label": sbl,
		"has_barcode":         strings.TrimSpace(t.Product.Barcode) != "",
	})
}

// VerifyPutterItem POST /warehouse/putter/tasks/:id/verify-item {"code":"..."}
func VerifyPutterItem(c *gin.Context) {
	EnsurePutawayTables()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	t, ok := putterLoadTask(c, staff)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code is required"})
		return
	}
	bc := strings.TrimSpace(t.Product.Barcode)
	if bc == "" {
		c.JSON(http.StatusOK, gin.H{"match": true, "no_barcode": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"match": strings.EqualFold(strings.TrimSpace(req.Code), bc), "no_barcode": false})
}

// ScanPutterBin POST /warehouse/putter/tasks/:id/scan-bin {"code":"..."}
func ScanPutterBin(c *gin.Context) {
	EnsurePutawayTables()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	t, ok := putterLoadTask(c, staff)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code is required"})
		return
	}
	code := strings.ToLower(strings.TrimSpace(req.Code))

	var bins []models.WarehouseBin
	database.DB.Preload("Rack.Zone").
		Select("warehouse_bins.*").
		Joins("JOIN warehouse_racks ON warehouse_racks.id = warehouse_bins.rack_id").
		Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_zones.warehouse_id = ?", staff.WarehouseID).
		Find(&bins)

	find := func(keys func(b models.WarehouseBin) []string) (models.WarehouseBin, int) {
		var hit models.WarehouseBin
		n := 0
		for _, b := range bins {
			for _, k := range keys(b) {
				if k == code {
					hit = b
					n++
					break
				}
			}
		}
		return hit, n
	}
	hit, n := find(func(b models.WarehouseBin) []string {
		return []string{
			strings.ToLower(putterBinLabel(b)),
			strings.ToLower(b.Rack.Name + "-" + b.Name),
			strings.ToLower(b.Rack.Name + "/" + b.Name),
			"bin-" + strconv.Itoa(int(b.ID)),
		}
	})
	if n > 1 {
		c.JSON(http.StatusOK, gin.H{"found": false, "error": "Several bins match this code. Scan the full rack-bin label."})
		return
	}
	if n == 0 {
		hit, n = find(func(b models.WarehouseBin) []string {
			return []string{strings.ToLower(strings.TrimSpace(b.Name)), strconv.Itoa(int(b.ID))}
		})
		if n > 1 {
			c.JSON(http.StatusOK, gin.H{"found": false, "error": "Several bins have this name. Scan the full rack-bin label."})
			return
		}
	}
	if n == 0 {
		c.JSON(http.StatusOK, gin.H{"found": false, "error": "Bin not found in your warehouse"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"found":        true,
		"bin":          gin.H{"id": hit.ID, "label": putterBinLabel(hit)},
		"is_suggested": t.SuggestedBinID != nil && *t.SuggestedBinID == hit.ID,
	})
}

// ReportPutterIssue POST /warehouse/putter/tasks/:id/report {"type":"damaged|short","quantity":N,"reason":"..."}
func ReportPutterIssue(c *gin.Context) {
	EnsurePutawayTables()
	ensurePutterReportTable()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	t, ok := putterLoadTask(c, staff)
	if !ok {
		return
	}
	var req struct {
		Type     string `json:"type"`
		Quantity int    `json:"quantity"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	typ := strings.ToLower(strings.TrimSpace(req.Type))
	if typ != "damaged" && typ != "short" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Type must be damaged or short"})
		return
	}
	if t.Status == models.PutawayCompleted {
		c.JSON(http.StatusConflict, gin.H{"error": "Task is already completed"})
		return
	}
	if req.Quantity <= 0 || req.Quantity > t.Quantity {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Quantity must be between 1 and " + strconv.Itoa(t.Quantity)})
		return
	}
	rep := models.PutterPutawayReport{
		TaskID: t.ID, StaffID: staff.ID, WarehouseID: staff.WarehouseID,
		Type: typ, Quantity: req.Quantity, Reason: strings.TrimSpace(req.Reason),
	}
	if err := database.DB.Create(&rep).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save the report"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "report": rep})
}

// GetPutterHistory GET /warehouse/putter/history (last 30 days)
func GetPutterHistory(c *gin.Context) {
	EnsurePutawayTables()
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	since := time.Now().AddDate(0, 0, -30)
	var tasks []models.PutawayTask
	opsPutawayPreload(database.DB.Where("warehouse_id = ? AND putter_id = ? AND status = ? AND completed_at >= ?",
		staff.WarehouseID, staff.ID, models.PutawayCompleted, since)).
		Order("completed_at DESC").Limit(100).Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"history": opsPutawayRows(tasks)})
}
