package handlers

import (
"fmt"
"net/http"
"sort"
"strings"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// CityStoreSummary is one city's warehouse counts for the Control Tower.
type CityStoreSummary struct {
City         string `json:"city"`
TotalStores  int64  `json:"total_stores"`
OpenStores   int64  `json:"open_stores"`
PausedStores int64  `json:"paused_stores"`
ClosedStores int64  `json:"closed_stores"`
}

// GetControlTowerOverview godoc
// GET /api/v1/admin/control-tower (admin only)
// Single-screen operational view: platform-wide pause state, per-city store
// status breakdown, and open ticket count - the read side of the kill
// switches (store-level pause itself already exists via Warehouses page).
func GetControlTowerOverview(c *gin.Context) {
var pauseSetting models.Setting
platformPaused := false
if err := database.DB.Where("key = ?", "platform_orders_paused").First(&pauseSetting).Error; err == nil {
platformPaused = pauseSetting.Value == "true"
}

type cityRow struct {
City   string
Status string
}
var rows []cityRow
if err := database.DB.Model(&models.Warehouse{}).Select("city, status").Find(&rows).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load store data"})
return
}

byCity := make(map[string]*CityStoreSummary)
for _, r := range rows {
s, ok := byCity[r.City]
if !ok {
s = &CityStoreSummary{City: r.City}
byCity[r.City] = s
}
s.TotalStores++
switch r.Status {
case "open":
s.OpenStores++
case "paused":
s.PausedStores++
case "closed":
s.ClosedStores++
}
}
cities := make([]CityStoreSummary, 0, len(byCity))
for _, s := range byCity {
cities = append(cities, *s)
}

var openTickets int64
database.DB.Model(&models.SupportTicket{}).Where("status = ? OR status = ?", "open", "in_progress").Count(&openTickets)

var totalStores int64
database.DB.Model(&models.Warehouse{}).Count(&totalStores)

var pausedOrClosedStores int64
database.DB.Model(&models.Warehouse{}).Where("status != ?", "open").Count(&pausedOrClosedStores)

c.JSON(http.StatusOK, gin.H{
"platform_orders_paused": platformPaused,
"total_stores":           totalStores,
"paused_or_closed_stores": pausedOrClosedStores,
"open_support_tickets":   openTickets,
"cities":                 cities,
})
}

// GetControlTowerSettings godoc
// GET /api/v1/admin/control-tower/settings (admin only)
// Returns the raw platform-level toggles (platform_orders_paused,
// cod_enabled) as booleans - a small companion to GetControlTowerOverview
// for screens that only need the toggle states, not the full dashboard.
func GetControlTowerSettings(c *gin.Context) {
var rows []models.Setting
if err := database.DB.Where("key IN ?", []string{"platform_orders_paused", "cod_enabled"}).Find(&rows).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
return
}

result := map[string]bool{
"platform_orders_paused": false,
"cod_enabled":             true,
}
for _, r := range rows {
result[r.Key] = r.Value == "true"
}

c.JSON(http.StatusOK, result)
}

// UpdateCODEnabled godoc
// PUT /api/v1/admin/control-tower/settings/cod (admin only)
// Body: {"enabled": false, "reason": "..."}
// Platform-wide Cash on Delivery kill switch. Checkout() rejects any
// cod payment_method while this is false - enforced server-side, not
// just hidden in the customer app, so a direct API call can't bypass it.
func UpdateCODEnabled(c *gin.Context) {
var body struct {
Enabled bool   `json:"enabled"`
Reason  string `json:"reason" binding:"required"`
}
if err := c.ShouldBindJSON(&body); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var setting models.Setting
if err := database.DB.Where("key = ?", "cod_enabled").First(&setting).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "cod_enabled setting not found - run the latest migrations"})
return
}

previousValue := setting.Value
newValue := "false"
if body.Enabled {
newValue = "true"
}
setting.Value = newValue
if err := database.DB.Save(&setting).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update COD setting"})
return
}

action := "COD_DISABLED"
if body.Enabled {
action = "COD_ENABLED"
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
details := fmt.Sprintf("reason: %s | previous_state: %s | new_state: %s", body.Reason, previousValue, newValue)
utils.LogAudit(adminID, adminPhone, action, "settings", "cod_enabled", details)

c.JSON(http.StatusOK, gin.H{"cod_enabled": body.Enabled})
}

// UpdatePlatformPause godoc
// PUT /api/v1/admin/control-tower/platform-pause (admin only)
// Body: {"paused": true}
// Platform-wide kill switch - Checkout() rejects all new orders while this
// is true, regardless of store/city status. Existing in-flight orders are
// untouched.
func UpdatePlatformPause(c *gin.Context) {
var body struct {
Paused bool   `json:"paused"`
Reason string `json:"reason" binding:"required"`
}
if err := c.ShouldBindJSON(&body); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var setting models.Setting
if err := database.DB.Where("key = ?", "platform_orders_paused").First(&setting).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "platform_orders_paused setting not found - run the latest migrations"})
return
}

previousValue := setting.Value
newValue := "false"
if body.Paused {
newValue = "true"
}
setting.Value = newValue
if err := database.DB.Save(&setting).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update platform pause state"})
return
}

action := "PLATFORM_RESUMED"
if body.Paused {
action = "PLATFORM_PAUSED"
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
details := fmt.Sprintf("reason: %s | previous_state: %s | new_state: %s", body.Reason, previousValue, newValue)
utils.LogAudit(adminID, adminPhone, action, "platform", "platform_orders_paused", details)

c.JSON(http.StatusOK, gin.H{"platform_orders_paused": body.Paused})
}

// UpdateCityStoreStatus godoc
// PUT /api/v1/admin/control-tower/city-status (admin only)
// Body: {"city": "Mumbai", "status": "paused"}
// City-level kill switch: bulk-sets every warehouse in that city to the
// given status. Reuses the same Warehouse.Status field the Warehouses page
// and serviceability check already respect - this just applies it in bulk.
func UpdateCityStoreStatus(c *gin.Context) {
var body struct {
City   string `json:"city" binding:"required"`
Status string `json:"status" binding:"required"`
Reason string `json:"reason" binding:"required"`
}
if err := c.ShouldBindJSON(&body); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

validStatuses := map[string]bool{"open": true, "paused": true, "closed": true}
if !validStatuses[body.Status] {
c.JSON(http.StatusBadRequest, gin.H{"error": "status must be one of: open, paused, closed"})
return
}

// Capture the per-store status mix before the bulk update, so the audit
// log records what actually changed rather than just the new value.
var previousStatuses []string
database.DB.Model(&models.Warehouse{}).Where("city = ?", body.City).Pluck("status", &previousStatuses)
previousSummary := summarizeStatuses(previousStatuses)

result := database.DB.Model(&models.Warehouse{}).Where("city = ?", body.City).Update("status", body.Status)
if result.Error != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update city stores"})
return
}
if result.RowsAffected == 0 {
c.JSON(http.StatusNotFound, gin.H{"error": "No stores found in that city"})
return
}

action := "CITY_STATUS_UPDATED"
switch body.Status {
case "paused":
action = "CITY_PAUSED"
case "closed":
action = "CITY_CLOSED"
case "open":
action = "CITY_OPENED"
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
newSummary := fmt.Sprintf("%s:%d", body.Status, result.RowsAffected)
details := fmt.Sprintf("reason: %s | previous_state: %s | new_state: %s", body.Reason, previousSummary, newSummary)
utils.LogAudit(adminID, adminPhone, action, "city", body.City, details)

c.JSON(http.StatusOK, gin.H{"success": true, "city": body.City, "status": body.Status, "stores_updated": result.RowsAffected})
}

// summarizeStatuses turns a list of per-store statuses into a compact
// "open:2,paused:1" style summary for audit-log details.
func summarizeStatuses(statuses []string) string {
if len(statuses) == 0 {
return "none"
}
counts := map[string]int{}
for _, s := range statuses {
counts[s]++
}
parts := make([]string, 0, len(counts))
for status, count := range counts {
parts = append(parts, fmt.Sprintf("%s:%d", status, count))
}
sort.Strings(parts)
return strings.Join(parts, ",")
}