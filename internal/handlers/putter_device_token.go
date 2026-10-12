package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm/clause"
)

type putterDeviceTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"`
}

// RegisterPutterDeviceToken: POST /warehouse/putter/device-token
func RegisterPutterDeviceToken(c *gin.Context) {
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req putterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tok := strings.TrimSpace(req.Token)
	if tok == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	platform := req.Platform
	if platform == "" {
		platform = "android"
	}
	sid := staffID
	row := models.DeviceToken{Token: tok, Platform: platform, StaffID: &sid}
	err := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"platform", "staff_id", "user_id", "delivery_partner_id", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save device token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Token registered"})
}

// UnregisterPutterDeviceToken: DELETE /warehouse/putter/device-token?token=...
func UnregisterPutterDeviceToken(c *gin.Context) {
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	tok := strings.TrimSpace(c.Query("token"))
	if tok == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	database.DB.Where("token = ? AND staff_id = ?", tok, staffID).Delete(&models.DeviceToken{})
	c.JSON(http.StatusOK, gin.H{"message": "Token removed"})
}
