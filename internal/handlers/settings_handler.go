package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// GetSettings godoc
// GET /api/v1/admin/settings (admin only)
func GetSettings(c *gin.Context) {
	EnsureOpsSettings()
	var settings []models.Setting
	if err := database.DB.Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	c.JSON(http.StatusOK, gin.H{"settings": result})
}

// UpdateSettings godoc
// PUT /api/v1/admin/settings (admin only)
// Body: {"settings": {"free_delivery_threshold": "600", ...}}
// Only known keys already present in the table are updated - this prevents
// arbitrary key injection.
func UpdateSettings(c *gin.Context) {
	EnsureOpsSettings()
	var req models.SettingUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated := make(map[string]string)
	for key, value := range req.Settings {
		var existing models.Setting
		if err := database.DB.Where("key = ?", key).First(&existing).Error; err != nil {
			continue // unknown key - skip silently rather than allowing arbitrary keys
		}
		existing.Value = value
		if err := database.DB.Save(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting: " + key})
			return
		}
		updated[key] = value
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "update_settings", "settings", "-", "updated keys")

	c.JSON(http.StatusOK, gin.H{"success": true, "updated": updated})
}
