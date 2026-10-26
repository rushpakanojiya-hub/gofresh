package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetMyNotifications godoc
// GET /api/v1/notifications (protected)
// Returns the logged-in user's notification history (order updates,
// payment confirmations, etc.), matched by their phone number since
// Notification records are keyed by phone rather than user_id.
func GetMyNotifications(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var notifications []models.Notification
	if err := database.DB.
		Where("phone = ?", user.Phone).
		Order("created_at DESC").
		Limit(50).
		Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}
