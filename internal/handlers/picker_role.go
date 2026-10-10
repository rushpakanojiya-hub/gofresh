package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// PickerRoleOnly lets only warehouse staff whose role is "picker" use the
// picker routes (presence, task claim, handover). Managers/putters get 403.
func PickerRoleOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID, ok := pickerStaffID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		var st models.WarehouseStaff
		if err := database.DB.Select("id", "role").First(&st, staffID).Error; err != nil || st.Role != "picker" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Only picker accounts can use the picker app"})
			return
		}
		c.Next()
	}
}
