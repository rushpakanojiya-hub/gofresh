package handlers

import (
"net/http"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ListMismatches godoc
// GET /api/v1/admin/finance/mismatch-center?check_type=&dismissed=
// Runs every mismatch detection check live and returns the combined
// result set. check_type filters to one check; dismissed=false (default
// when omitted, callers should pass it explicitly) filters out already
// acknowledged items.
func ListMismatches(c *gin.Context) {
items, err := services.DetectMismatches()
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run mismatch detection: " + err.Error()})
return
}

if checkType := c.Query("check_type"); checkType != "" {
filtered := items[:0]
for _, item := range items {
if item.CheckType == checkType {
filtered = append(filtered, item)
}
}
items = filtered
}

if dismissed := c.Query("dismissed"); dismissed == "false" {
filtered := items[:0]
for _, item := range items {
if !item.IsDismissed {
filtered = append(filtered, item)
}
}
items = filtered
}

summary := gin.H{}
byType := map[string]int{}
for _, item := range items {
byType[item.CheckType]++
}
summary["total"] = len(items)
summary["by_type"] = byType

c.JSON(http.StatusOK, gin.H{"mismatches": items, "summary": summary})
}

// DismissMismatch godoc
// POST /api/v1/admin/finance/mismatch-center/dismiss
// body: { check_type, entity_id, reason }
func DismissMismatch(c *gin.Context) {
var req models.MismatchDismissRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

adminID := c.MustGet("user_id").(uint)

var existing models.MismatchDismissal
if err := database.DB.Where("check_type = ? AND entity_id = ?", req.CheckType, req.EntityID).First(&existing).Error; err == nil {
c.JSON(http.StatusOK, existing)
return
}

dismissal := models.MismatchDismissal{
CheckType:     req.CheckType,
EntityID:      req.EntityID,
Reason:        req.Reason,
DismissedByID: adminID,
DismissedAt:   time.Now(),
}
if err := database.DB.Create(&dismissal).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dismiss mismatch"})
return
}

adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "dismiss_mismatch", "mismatch", req.CheckType, req.Reason)

c.JSON(http.StatusCreated, dismissal)
}

// UndismissMismatch godoc
// POST /api/v1/admin/finance/mismatch-center/undismiss
// body: { check_type, entity_id }
func UndismissMismatch(c *gin.Context) {
var req struct {
CheckType string `json:"check_type" binding:"required"`
EntityID  uint   `json:"entity_id" binding:"required"`
}
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

if err := database.DB.Where("check_type = ? AND entity_id = ?", req.CheckType, req.EntityID).
Delete(&models.MismatchDismissal{}).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to undismiss mismatch"})
return
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "undismiss_mismatch", "mismatch", req.CheckType, "")

c.JSON(http.StatusOK, gin.H{"success": true})
}