package handlers

import (
"net/http"
"strconv"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetMyNotifications godoc
// GET /api/v1/delivery/notifications?unread_only=true (delivery partner only)
func GetMyDeliveryNotifications(c *gin.Context) {
partnerID := c.MustGet("user_id").(uint)

query := database.DB.Where("delivery_partner_id = ?", partnerID)
if c.Query("unread_only") == "true" {
query = query.Where("is_read = ?", false)
}

var notifications []models.DeliveryNotification
if err := query.Order("created_at desc").Limit(100).Find(&notifications).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load notifications"})
return
}

var unreadCount int64
database.DB.Model(&models.DeliveryNotification{}).
Where("delivery_partner_id = ? AND is_read = ?", partnerID, false).
Count(&unreadCount)

c.JSON(http.StatusOK, gin.H{
"notifications": notifications,
"unread_count":  unreadCount,
})
}

// MarkNotificationRead godoc
// PUT /api/v1/delivery/notifications/:id/read (delivery partner only)
func MarkDeliveryNotificationRead(c *gin.Context) {
partnerID := c.MustGet("user_id").(uint)
id, err := strconv.ParseUint(c.Param("id"), 10, 64)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification id"})
return
}

now := time.Now()
result := database.DB.Model(&models.DeliveryNotification{}).
Where("id = ? AND delivery_partner_id = ?", id, partnerID).
Updates(map[string]interface{}{"is_read": true, "read_at": now})

if result.Error != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
return
}
if result.RowsAffected == 0 {
c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
return
}

c.JSON(http.StatusOK, gin.H{"message": "Marked as read"})
}

// MarkAllNotificationsRead godoc
// PUT /api/v1/delivery/notifications/read-all (delivery partner only)
func MarkAllDeliveryNotificationsRead(c *gin.Context) {
partnerID := c.MustGet("user_id").(uint)
now := time.Now()

if err := database.DB.Model(&models.DeliveryNotification{}).
Where("delivery_partner_id = ? AND is_read = ?", partnerID, false).
Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notifications as read"})
return
}

c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}
