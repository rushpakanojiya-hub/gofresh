package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// InjectWarehouseScope looks up the authenticated warehouse staff member's
// warehouse_id and injects it into the Gin context as "warehouse_id".
// Must run after AuthMiddleware + WarehouseStaffOnly. Every warehouse-scoped
// handler should read this value and filter its queries by it - this is
// what prevents Warehouse A staff from seeing Warehouse B's data.
func InjectWarehouseScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		var staff models.WarehouseStaff
		if err := database.DB.First(&staff, staffID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Warehouse staff not found"})
			c.Abort()
			return
		}

		if !staff.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been deactivated"})
			c.Abort()
			return
		}

		effectiveWarehouseID := staff.WarehouseID
		database.EnsurePickerTables()
		var presence models.PickerPresence
		if err := database.DB.Where("staff_id = ? AND is_online = ?", staff.ID, true).First(&presence).Error; err == nil && presence.WarehouseID != 0 {
			effectiveWarehouseID = presence.WarehouseID
		}
		c.Set("warehouse_id", effectiveWarehouseID)
		c.Set("staff_id", staff.ID)
		c.Set("staff_name", staff.Name)
		c.Set("staff_role", staff.Role)
		c.Next()
	}
}
