package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetStockTransferSuppliers GET /warehouse/stock-transfers/suppliers
// Active mother warehouses (other than the caller's own) a store can request stock from.
func GetStockTransferSuppliers(c *gin.Context) {
	staffID := c.MustGet("user_id").(uint)
	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
		return
	}
	var rows []models.Warehouse
	database.DB.Where("is_active = ? AND warehouse_type = ? AND id <> ?", true, "mother_warehouse", staff.WarehouseID).
		Order("name").Find(&rows)
	out := make([]gin.H, 0, len(rows))
	for _, w := range rows {
		out = append(out, gin.H{"id": w.ID, "name": w.Name, "city": w.City})
	}
	c.JSON(http.StatusOK, gin.H{"suppliers": out})
}

// GetSupplierStock GET /warehouse/stock-transfers/suppliers/:id/stock?product_ids=1,2,3
// Stock of the given products at a supplying (mother) warehouse.
func GetSupplierStock(c *gin.Context) {
	sid, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid warehouse id"})
		return
	}
	var wh models.Warehouse
	if err := database.DB.Where("id = ? AND is_active = ? AND warehouse_type = ?", sid, true, "mother_warehouse").First(&wh).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplying warehouse not found"})
		return
	}
	ids := []uint{}
	for _, p := range strings.Split(c.Query("product_ids"), ",") {
		v, e := strconv.ParseUint(strings.TrimSpace(p), 10, 64)
		if e == nil && v > 0 {
			ids = append(ids, uint(v))
		}
	}
	stock := map[uint]int{}
	if len(ids) > 0 {
		var invs []models.Inventory
		database.DB.Where("warehouse_id = ? AND product_id IN ?", sid, ids).Find(&invs)
		for _, i := range invs {
			stock[i.ProductID] = i.Stock
		}
	}
	c.JSON(http.StatusOK, gin.H{"stock": stock})
}
