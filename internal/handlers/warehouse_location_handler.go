package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// ---- Zones ----

// GetWarehouseZones godoc
// GET /api/v1/warehouse/zones (warehouse staff only)
func GetWarehouseZones(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	var zones []models.WarehouseZone
	database.DB.Where("warehouse_id = ?", warehouseID).Order("name ASC").Find(&zones)
	c.JSON(http.StatusOK, gin.H{"zones": zones})
}

// CreateWarehouseZone godoc
// POST /api/v1/warehouse/zones (warehouse staff only)
func CreateWarehouseZone(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	var req models.WarehouseZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	storageType := "dry"
	if req.StorageType == "cold" {
		storageType = "cold"
	}
	zone := models.WarehouseZone{WarehouseID: warehouseID, Name: req.Name, StorageType: storageType}
	if err := database.DB.Create(&zone).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A zone with this name already exists in your warehouse"})
		return
	}
	c.JSON(http.StatusCreated, zone)
}

// ---- Racks ----

// GetZoneRacks godoc
// GET /api/v1/warehouse/zones/:zone_id/racks (warehouse staff only)
func GetZoneRacks(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	zoneID := c.Param("zone_id")

	var zone models.WarehouseZone
	if err := database.DB.Where("id = ? AND warehouse_id = ?", zoneID, warehouseID).First(&zone).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zone not found for your warehouse"})
		return
	}

	var racks []models.WarehouseRack
	database.DB.Where("zone_id = ?", zoneID).Order("name ASC").Find(&racks)
	c.JSON(http.StatusOK, gin.H{"racks": racks})
}

// CreateRack godoc
// POST /api/v1/warehouse/zones/:zone_id/racks (warehouse staff only)
func CreateRack(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	zoneID := c.Param("zone_id")

	var zone models.WarehouseZone
	if err := database.DB.Where("id = ? AND warehouse_id = ?", zoneID, warehouseID).First(&zone).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zone not found for your warehouse"})
		return
	}

	var req models.WarehouseRackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	zoneIDUint64, _ := strconv.ParseUint(zoneID, 10, 64)
	rack := models.WarehouseRack{ZoneID: uint(zoneIDUint64), Name: req.Name}
	if err := database.DB.Create(&rack).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A rack with this name already exists in this zone"})
		return
	}
	c.JSON(http.StatusCreated, rack)
}

// ---- Bins ----

// GetRackBins godoc
// GET /api/v1/warehouse/racks/:rack_id/bins (warehouse staff only)
func GetRackBins(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	rackID := c.Param("rack_id")

	var rack models.WarehouseRack
	if err := database.DB.Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_racks.id = ? AND warehouse_zones.warehouse_id = ?", rackID, warehouseID).
		First(&rack).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rack not found for your warehouse"})
		return
	}

	var bins []models.WarehouseBin
	database.DB.Where("rack_id = ?", rackID).Order("name ASC").Find(&bins)
	c.JSON(http.StatusOK, gin.H{"bins": bins})
}

// CreateBin godoc
// POST /api/v1/warehouse/racks/:rack_id/bins (warehouse staff only)
func CreateBin(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	rackID := c.Param("rack_id")

	var rack models.WarehouseRack
	if err := database.DB.Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_racks.id = ? AND warehouse_zones.warehouse_id = ?", rackID, warehouseID).
		First(&rack).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rack not found for your warehouse"})
		return
	}

	var req models.WarehouseBinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rackIDUint64, _ := strconv.ParseUint(rackID, 10, 64)
	bin := models.WarehouseBin{RackID: uint(rackIDUint64), Name: req.Name}
	if err := database.DB.Create(&bin).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A bin with this name already exists in this rack"})
		return
	}
	c.JSON(http.StatusCreated, bin)
}

// ---- Assign product to bin ----

// AssignProductBin godoc
// PUT /api/v1/warehouse/inventory/:product_id/bin (warehouse staff only)
// Assigns (or clears, if bin_id is null) the bin location for a product's
// inventory row in the caller's warehouse.
func AssignProductBin(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	productID := c.Param("productId")

	var req models.AssignBinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var inv models.Inventory
	if err := database.DB.Where("product_id = ? AND warehouse_id = ?", productID, warehouseID).First(&inv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No inventory record for this product in your warehouse"})
		return
	}

	if req.BinID != nil {
		var bin models.WarehouseBin
		if err := database.DB.Joins("JOIN warehouse_racks ON warehouse_racks.id = warehouse_bins.rack_id").
			Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
			Where("warehouse_bins.id = ? AND warehouse_zones.warehouse_id = ?", *req.BinID, warehouseID).
			First(&bin).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Bin not found in your warehouse"})
			return
		}
	}

	inv.BinID = req.BinID
	database.DB.Save(&inv)
	c.JSON(http.StatusOK, inv)
}

// GetProductInventory godoc
// GET /api/v1/warehouse/inventory/:product_id (warehouse staff only)
// Returns the caller's warehouse inventory row for one product, including
// its bin/rack/zone location if assigned. Used by the frontend to show
// current stock + location before an adjustment or bin (re)assignment.
func GetProductInventory(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	productID := c.Param("productId")

	var inv models.Inventory
	if err := database.DB.Where("product_id = ? AND warehouse_id = ?", productID, warehouseID).
		Preload("Product").Preload("Bin.Rack.Zone").First(&inv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No inventory record for this product in your warehouse"})
		return
	}
	c.JSON(http.StatusOK, inv)
}

// DeleteZone godoc
// DELETE /api/v1/warehouse/zones/:zone_id (warehouse staff only)
// Refuses to delete a zone that still has racks under it.
func DeleteZone(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	zoneID := c.Param("zone_id")

	var zone models.WarehouseZone
	if err := database.DB.Where("id = ? AND warehouse_id = ?", zoneID, warehouseID).First(&zone).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zone not found for your warehouse"})
		return
	}

	var rackCount int64
	database.DB.Model(&models.WarehouseRack{}).Where("zone_id = ?", zoneID).Count(&rackCount)
	if rackCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete a zone that still has racks - delete its racks first"})
		return
	}

	database.DB.Delete(&zone)
	staffID := c.MustGet("staff_id").(uint)
	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "delete_zone", "warehouse_zone", zoneID, zone.Name, "")
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteRack godoc
// DELETE /api/v1/warehouse/racks/:rack_id (warehouse staff only)
// Refuses to delete a rack that still has bins under it.
func DeleteRack(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	rackID := c.Param("rack_id")

	var rack models.WarehouseRack
	if err := database.DB.Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_racks.id = ? AND warehouse_zones.warehouse_id = ?", rackID, warehouseID).
		First(&rack).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rack not found for your warehouse"})
		return
	}

	var binCount int64
	database.DB.Model(&models.WarehouseBin{}).Where("rack_id = ?", rackID).Count(&binCount)
	if binCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete a rack that still has bins - delete its bins first"})
		return
	}

	database.DB.Delete(&rack)
	staffID := c.MustGet("staff_id").(uint)
	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "delete_rack", "warehouse_rack", rackID, rack.Name, "")
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteBin godoc
// DELETE /api/v1/warehouse/bins/:bin_id (warehouse staff only)
// Refuses to delete a bin that's still assigned to a product's inventory row.
func DeleteBin(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	binID := c.Param("bin_id")

	var bin models.WarehouseBin
	if err := database.DB.Joins("JOIN warehouse_racks ON warehouse_racks.id = warehouse_bins.rack_id").
		Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_bins.id = ? AND warehouse_zones.warehouse_id = ?", binID, warehouseID).
		First(&bin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bin not found for your warehouse"})
		return
	}

	var assignedCount int64
	database.DB.Model(&models.Inventory{}).Where("bin_id = ?", binID).Count(&assignedCount)
	if assignedCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete a bin that's still assigned to a product - unassign it first"})
		return
	}

	database.DB.Delete(&bin)
	staffID := c.MustGet("staff_id").(uint)
	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "delete_bin", "warehouse_bin", binID, bin.Name, "")
	c.JSON(http.StatusOK, gin.H{"success": true})
}
