package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// RenameRack: PUT /api/v1/warehouse/racks/:rack_id  {"name": "A-02"}
func RenameRack(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	rackID := c.Param("rack_id")

	var req models.WarehouseRackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	var rack models.WarehouseRack
	if err := database.DB.
		Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_racks.id = ? AND warehouse_zones.warehouse_id = ?", rackID, warehouseID).
		First(&rack).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rack not found for your warehouse"})
		return
	}

	if err := database.DB.Model(&models.WarehouseRack{}).Where("id = ?", rack.ID).Update("name", name).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A rack with this name already exists in this zone"})
		return
	}
	rack.Name = name
	c.JSON(http.StatusOK, rack)
}

// RenameBin: PUT /api/v1/warehouse/bins/:bin_id  {"name": "A-01-02"}
func RenameBin(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	binID := c.Param("bin_id")

	var req models.WarehouseBinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	var bin models.WarehouseBin
	if err := database.DB.
		Joins("JOIN warehouse_racks ON warehouse_racks.id = warehouse_bins.rack_id").
		Joins("JOIN warehouse_zones ON warehouse_zones.id = warehouse_racks.zone_id").
		Where("warehouse_bins.id = ? AND warehouse_zones.warehouse_id = ?", binID, warehouseID).
		First(&bin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bin not found for your warehouse"})
		return
	}

	if err := database.DB.Model(&models.WarehouseBin{}).Where("id = ?", bin.ID).Update("name", name).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A bin with this name already exists in this rack"})
		return
	}
	bin.Name = name
	c.JSON(http.StatusOK, bin)
}
