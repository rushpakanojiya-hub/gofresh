package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

var validAdminRoles = map[string]bool{
	"super_admin":       true,
	"admin":             true,
	"ops_manager":       true,
	"warehouse_manager": true,
	"support_agent":     true,
	"finance_manager":   true,
}

// GetAdminStaff godoc
// GET /api/v1/admin/staff (admin only)
func GetAdminStaff(c *gin.Context) {
	var users []models.User
	if err := database.DB.Where("role = ?", "admin").Order("created_at ASC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch staff"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"staff": users})
}

// UpdateStaffRole godoc
// PUT /api/v1/admin/staff/:id/role (admin only, requires staff:manage permission)
func UpdateStaffRole(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		AdminRole string `json:"admin_role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validAdminRoles[req.AdminRole] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid admin role"})
		return
	}

	var user models.User
	if err := database.DB.Where("role = ?", "admin").First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Staff member not found"})
		return
	}

	user.AdminRole = req.AdminRole
	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "update_staff_role", "user", id, "new_role: "+req.AdminRole)

	c.JSON(http.StatusOK, gin.H{"success": true, "admin_role": req.AdminRole})
}
