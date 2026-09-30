package handlers

import (
"net/http"
"strconv"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// AdminStaffPerformanceRow adds warehouse context to StaffPerformanceRow so
// an admin looking at multiple warehouses at once can tell which staff
// belongs where - the warehouse-scoped version doesn't need this since the
// caller already knows their own warehouse.
type AdminStaffPerformanceRow struct {
StaffPerformanceRow
WarehouseID   uint   `json:"warehouse_id"`
WarehouseName string `json:"warehouse_name"`
}

// GetAdminPickerPerformance godoc
// GET /api/v1/admin/picker-performance?warehouse_id=3 (admin only)
// Reuses the same buildStaffPerformance computation the warehouse-facing
// endpoints use (GetWarehouseStaffPerformance, GetMyPerformance) so admin
// and warehouse-staff views can never disagree with each other - this is
// purely an aggregation layer, not a second calculation.
//
// With no warehouse_id, aggregates across every active warehouse so admin
// gets a single picker-performance view of the whole operation, not one
// warehouse at a time.
func GetAdminPickerPerformance(c *gin.Context) {
warehouseIDParam := c.Query("warehouse_id")

var warehouses []models.Warehouse
if warehouseIDParam != "" {
id, err := strconv.ParseUint(warehouseIDParam, 10, 64)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid warehouse_id"})
return
}
if err := database.DB.Where("id = ?", uint(id)).Find(&warehouses).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load warehouse"})
return
}
if len(warehouses) == 0 {
c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse not found"})
return
}
} else {
if err := database.DB.Where("is_active = ?", true).Find(&warehouses).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load warehouses"})
return
}
}

rows := make([]AdminStaffPerformanceRow, 0)
for _, w := range warehouses {
staffRows := buildStaffPerformance(w.ID, nil)
for _, sr := range staffRows {
rows = append(rows, AdminStaffPerformanceRow{
StaffPerformanceRow: sr,
WarehouseID:         w.ID,
WarehouseName:       w.Name,
})
}
}

c.JSON(http.StatusOK, gin.H{"staff_performance": rows})
}
