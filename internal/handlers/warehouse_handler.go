package handlers

import (
"log"
"net/http"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// ---------------------------------------------------------------------------
// Warehouses (admin only)
// ---------------------------------------------------------------------------

// CreateWarehouse godoc
// POST /api/v1/admin/warehouses (admin only)
func CreateWarehouse(c *gin.Context) {
var req models.WarehouseRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

radius := req.ServiceRadiusKm
if radius <= 0 {
radius = 5
}

warehouseType := req.WarehouseType
if warehouseType == "" {
warehouseType = "dark_store"
}

status := req.Status
if status == "" {
status = "open"
}

warehouse := models.Warehouse{
Name:            req.Name,
City:            req.City,
Address:         req.Address,
Lat:             req.Lat,
Lng:             req.Lng,
ServiceRadiusKm: radius,
IsActive:        true,
WarehouseType:   warehouseType,
Status:          status,
Capacity:        req.Capacity,
OpeningTime:     req.OpeningTime,
ClosingTime:     req.ClosingTime,
}
if req.IsActive != nil {
warehouse.IsActive = *req.IsActive
}

if err := database.DB.Create(&warehouse).Error; err != nil {
c.JSON(http.StatusConflict, gin.H{"error": "Warehouse could not be created"})
return
}

c.JSON(http.StatusCreated, gin.H{"warehouse": warehouse})
}

// GetWarehouses godoc
// GET /api/v1/admin/warehouses (admin only)
func GetWarehouses(c *gin.Context) {
var warehouses []models.Warehouse
if err := database.DB.Order("created_at DESC").Find(&warehouses).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load warehouses"})
return
}
attachServiceAreas(warehouses)
c.JSON(http.StatusOK, gin.H{"warehouses": warehouses})
}

// attachServiceAreas fills in the ServiceArea field (ignored by GORM since
// it's a PostGIS geometry column) with each warehouse's service_area as a
// GeoJSON string, so the admin panel can render the saved polygon.
func attachServiceAreas(warehouses []models.Warehouse) {
if len(warehouses) == 0 {
return
}
type row struct {
ID      uint
GeoJSON string
}
var rows []row
if err := database.DB.Raw(
`SELECT id, ST_AsGeoJSON(service_area) AS geo_json FROM warehouses WHERE service_area IS NOT NULL AND deleted_at IS NULL`,
).Scan(&rows).Error; err != nil {
log.Printf("failed to load warehouse service areas: %v", err)
return
}
byID := make(map[uint]string, len(rows))
for _, r := range rows {
byID[r.ID] = r.GeoJSON
}
for i := range warehouses {
if gj, ok := byID[warehouses[i].ID]; ok {
warehouses[i].ServiceArea = gj
}
}
}

// GetWarehouse godoc
// GET /api/v1/admin/warehouses/:id (admin only)
func GetWarehouse(c *gin.Context) {
id := c.Param("id")

var warehouse models.Warehouse
if err := database.DB.First(&warehouse, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse not found"})
return
}
attachServiceAreas([]models.Warehouse{warehouse})

c.JSON(http.StatusOK, gin.H{"warehouse": warehouse})
}

// UpdateWarehouse godoc
// PUT /api/v1/admin/warehouses/:id (admin only)
func UpdateWarehouse(c *gin.Context) {
id := c.Param("id")

var warehouse models.Warehouse
if err := database.DB.First(&warehouse, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse not found"})
return
}

var req models.WarehouseRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

warehouse.Name = req.Name
warehouse.City = req.City
warehouse.Address = req.Address
warehouse.Lat = req.Lat
warehouse.Lng = req.Lng
if req.ServiceRadiusKm > 0 {
warehouse.ServiceRadiusKm = req.ServiceRadiusKm
}
if req.IsActive != nil {
warehouse.IsActive = *req.IsActive
}
if req.WarehouseType != "" {
warehouse.WarehouseType = req.WarehouseType
}
if req.Status != "" {
warehouse.Status = req.Status
}
warehouse.Capacity = req.Capacity
warehouse.OpeningTime = req.OpeningTime
warehouse.ClosingTime = req.ClosingTime

if err := database.DB.Save(&warehouse).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update warehouse"})
return
}

c.JSON(http.StatusOK, gin.H{"warehouse": warehouse})
}

// DeleteWarehouse godoc
// DELETE /api/v1/admin/warehouses/:id (admin only)
func DeleteWarehouse(c *gin.Context) {
id := c.Param("id")

result := database.DB.Delete(&models.Warehouse{}, id)
if result.Error != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete warehouse"})
return
}
if result.RowsAffected == 0 {
c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse not found"})
return
}

c.JSON(http.StatusOK, gin.H{"message": "Warehouse deleted"})
}

// SetWarehouseServiceArea sets the polygon geofence for a warehouse using GeoJSON.
// Body: {"geojson": "{\"type\":\"Polygon\",\"coordinates\":[[[lng,lat],[lng,lat],...]]}"}
func SetWarehouseServiceArea(c *gin.Context) {
id := c.Param("id")

var warehouse models.Warehouse
if err := database.DB.First(&warehouse, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse not found"})
return
}

var req struct {
GeoJSON string `json:"geojson" binding:"required"`
}
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

if err := database.DB.Exec(
`UPDATE warehouses SET service_area = ST_SetSRID(ST_GeomFromGeoJSON(?), 4326) WHERE id = ?`,
req.GeoJSON, warehouse.ID,
).Error; err != nil {
log.Printf("failed to set service_area for warehouse %v: %v", warehouse.ID, err)
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid GeoJSON polygon: " + err.Error()})
return
}

c.JSON(http.StatusOK, gin.H{"success": true, "warehouse_id": warehouse.ID})
}
