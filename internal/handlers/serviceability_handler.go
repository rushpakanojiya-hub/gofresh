package handlers

import (
"math"
"net/http"
"strconv"

"github.com/gin-gonic/gin"
	"log"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// ---------------------------------------------------------------------------
// Serviceability check (public)
// ---------------------------------------------------------------------------

const earthRadiusKm = 6371.0

// haversineDistanceKm returns the great-circle distance between two
// lat/lng points in kilometers.
func haversineDistanceKm(lat1, lng1, lat2, lng2 float64) float64 {
lat1Rad := lat1 * math.Pi / 180
lat2Rad := lat2 * math.Pi / 180
deltaLat := (lat2 - lat1) * math.Pi / 180
deltaLng := (lng2 - lng1) * math.Pi / 180

a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
math.Cos(lat1Rad)*math.Cos(lat2Rad)*
math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
return earthRadiusKm * c
}

// FindNearestWarehouse returns the nearest active warehouse to the given
// coordinates and the distance in km. Shared by the public /serviceability
// endpoint and checkout (to assign + validate deliverability before an
// order is placed). Returns (nil, 0, nil) if there are no active warehouses.
func FindNearestWarehouse(lat, lng float64) (*models.Warehouse, float64, error) {
var warehouses []models.Warehouse
if err := database.DB.Where("is_active = ? AND status = ?", true, "open").Find(&warehouses).Error; err != nil {
return nil, 0, err
}
if len(warehouses) == 0 {
return nil, 0, nil
}

var nearest models.Warehouse
nearestDistance := math.MaxFloat64
for _, wh := range warehouses {
dist := haversineDistanceKm(lat, lng, wh.Lat, wh.Lng)
if dist < nearestDistance {
nearestDistance = dist
nearest = wh
}
}
return &nearest, nearestDistance, nil
}

// CheckServiceability godoc
// GET /api/v1/serviceability?lat=&lng=
// Finds the nearest active warehouse to the given coordinates and reports
// whether it is within that warehouse's service radius.
func CheckServiceability(c *gin.Context) {
latStr := c.Query("lat")
lngStr := c.Query("lng")
if latStr == "" || lngStr == "" {
c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lng query params are required"})
return
}

lat, err := strconv.ParseFloat(latStr, 64)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lat"})
return
}
lng, err := strconv.ParseFloat(lngStr, 64)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lng"})
return
}

nearest, distance, err := FindNearestWarehouse(lat, lng)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load warehouses"})
return
}
if nearest == nil {
c.JSON(http.StatusOK, gin.H{
"serviceable": false,
"message":     "No active warehouses available",
})
return
}

var hasPolygon bool
var containsPoint bool
row := database.DB.Raw(
`SELECT service_area IS NOT NULL, CASE WHEN service_area IS NOT NULL THEN ST_Contains(service_area, ST_SetSRID(ST_MakePoint(?, ?), 4326)) ELSE false END FROM warehouses WHERE id = ?`,
lng, lat, nearest.ID,
).Row()
if err := row.Scan(&hasPolygon, &containsPoint); err != nil {
	log.Printf("serviceability check failed: %v", err)
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check serviceability"})
return
}

var serviceable bool
method := "radius"
if hasPolygon {
serviceable = containsPoint
method = "polygon"
} else {
serviceable = distance <= nearest.ServiceRadiusKm
}
response := gin.H{
"serviceable": serviceable,
"distance_km": math.Round(distance*100) / 100,
"nearest_warehouse": gin.H{
"id":                nearest.ID,
"name":              nearest.Name,
"city":              nearest.City,
"service_radius_km": nearest.ServiceRadiusKm,
},
	"method":      method,
}
if !serviceable {
response["message"] = "Sorry, we don't deliver to this location yet"
}

c.JSON(http.StatusOK, response)
}
