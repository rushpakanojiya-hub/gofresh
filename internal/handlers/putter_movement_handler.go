package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// putterFindBin resolves a scanned code to a bin of this warehouse. Returns an error message if not found.
func putterFindBin(wid uint, raw string) (models.WarehouseBin, string) {
	code := strings.ToLower(strings.TrimSpace(raw))
	var bins []models.WarehouseBin
	database.DB.Preload("Rack.Zone").
		Select("warehouse_bins.*").
		Joins("JOIN warehouse_racks ON warehouse_racks.id = warehouse_bins.rack_id").
		Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_zones.warehouse_id = ?", wid).
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
		return hit, "Several bins match this code. Scan the full rack-bin label."
	}
	if n == 0 {
		hit, n = find(func(b models.WarehouseBin) []string {
			return []string{strings.ToLower(strings.TrimSpace(b.Name)), strconv.Itoa(int(b.ID))}
		})
		if n > 1 {
			return hit, "Several bins have this name. Scan the full rack-bin label."
		}
	}
	if n == 0 {
		return hit, "Bin not found in your warehouse"
	}
	return hit, ""
}

func putterBinByID(tx *gorm.DB, wid, id uint) (models.WarehouseBin, error) {
	var bin models.WarehouseBin
	err := tx.Preload("Rack.Zone").
		Select("warehouse_bins.*").
		Joins("JOIN warehouse_racks ON warehouse_racks.id = warehouse_bins.rack_id").
		Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_bins.id = ? AND warehouse_zones.warehouse_id = ?", id, wid).
		First(&bin).Error
	return bin, err
}

// PutterMovementLookup POST /warehouse/putter/movements/lookup {"code":"..."}
func PutterMovementLookup(c *gin.Context) {
	staff, ok := putterLoadStaff(c)
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
	bin, msg := putterFindBin(staff.WarehouseID, req.Code)
	if msg != "" {
		c.JSON(http.StatusOK, gin.H{"found": false, "error": msg})
		return
	}
	var invs []models.Inventory
	database.DB.Preload("Product").
		Where("warehouse_id = ? AND bin_id = ? AND stock > 0", staff.WarehouseID, bin.ID).
		Find(&invs)
	items := make([]gin.H, 0, len(invs))
	for _, iv := range invs {
		items = append(items, gin.H{
			"product_id": iv.ProductID,
			"name":       iv.Product.Name,
			"image_url":  iv.Product.ImageURL,
			"barcode":    iv.Product.Barcode,
			"stock":      iv.Stock,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"found": true,
		"bin":   gin.H{"id": bin.ID, "label": putterBinLabel(bin)},
		"items": items,
	})
}

// PutterMoveStock POST /warehouse/putter/movements {"product_id","from_bin_id","to_bin_id"}
// Inventory keeps one bin per product, so the whole product moves to the new bin.
func PutterMoveStock(c *gin.Context) {
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	var req struct {
		ProductID uint `json:"product_id" binding:"required"`
		FromBinID uint `json:"from_bin_id" binding:"required"`
		ToBinID   uint `json:"to_bin_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.FromBinID == req.ToBinID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Destination bin is the same as source"})
		return
	}
	if _, on := putterActiveShift(staff.ID); !on {
		c.JSON(http.StatusConflict, gin.H{"error": "Start your shift from Home first"})
		return
	}
	wid := staff.WarehouseID
	var fromLabel, toLabel string
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		var inv models.Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", req.ProductID, wid).First(&inv).Error; err != nil {
			return opsErr{http.StatusNotFound, "Stock not found"}
		}
		if inv.BinID == nil || *inv.BinID != req.FromBinID {
			return opsErr{http.StatusConflict, "Item is no longer in the source bin. Scan again."}
		}
		from, err := putterBinByID(tx, wid, req.FromBinID)
		if err != nil {
			return opsErr{http.StatusBadRequest, "Source bin not found in your warehouse"}
		}
		to, err := putterBinByID(tx, wid, req.ToBinID)
		if err != nil {
			return opsErr{http.StatusBadRequest, "Destination bin not found in your warehouse"}
		}
		fromLabel = putterBinLabel(from)
		toLabel = putterBinLabel(to)
		if err := tx.Model(&models.Inventory{}).Where("id = ?", inv.ID).Update("bin_id", req.ToBinID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Batch{}).
			Where("product_id = ? AND warehouse_id = ? AND bin_id = ?", req.ProductID, wid, req.FromBinID).
			Update("bin_id", req.ToBinID).Error; err != nil {
			return err
		}
		sid := staff.ID
		return tx.Create(&models.StockMovement{
			ProductID: req.ProductID, WarehouseID: wid,
			PreviousQty: inv.Stock, Change: 0, NewQty: inv.Stock,
			MovementType: models.MovementTransfer, Reason: "bin_move", StaffID: &sid,
			Notes: fmt.Sprintf("%s -> %s", fromLabel, toLabel),
		}).Error
	})
	if txErr != nil {
		opsFail(c, txErr, "Could not move stock")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "from": fromLabel, "to": toLabel})
}

// GetPutterMovements GET /warehouse/putter/movements (last 30 days, this putter)
func GetPutterMovements(c *gin.Context) {
	staff, ok := putterLoadStaff(c)
	if !ok {
		return
	}
	since := time.Now().AddDate(0, 0, -30)
	var rows []models.StockMovement
	database.DB.Preload("Product").
		Where("warehouse_id = ? AND staff_id = ? AND movement_type = ? AND reason = ? AND created_at >= ?",
			staff.WarehouseID, staff.ID, models.MovementTransfer, "bin_move", since).
		Order("created_at DESC").Limit(100).Find(&rows)
	c.JSON(http.StatusOK, gin.H{"movements": rows})
}
