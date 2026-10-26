package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// BroadcastNotification godoc
// POST /api/v1/admin/notifications/broadcast (admin only)
// body: { "title": "...", "body": "..." }
// Sends a push notification to every registered device token.
func BroadcastNotification(c *gin.Context) {
	var req struct {
		Title string `json:"title" binding:"required"`
		Body  string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go services.SendPushToAll(req.Title, req.Body)

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "broadcast_notification", "notification", "-", "title: "+req.Title)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Broadcast queued"})
}
