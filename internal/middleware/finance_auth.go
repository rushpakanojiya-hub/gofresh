package middleware

import (
"net/http"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// FinanceOnly restricts access to admin users whose AdminRole is
// "finance_manager" or "super_admin". Finance data (revenue, GST, P&L,
// expenses) is sensitive and kept separate from general admin/ops staff -
// per the same DB-driven sub-role pattern as RequirePermission in
// permissions.go. Must run after AuthMiddleware + AdminOnly.
//
// Legacy admin accounts with AdminRole == "" retain access too, same as
// RequirePermission, so existing single-admin setups aren't locked out
// until sub-roles are actually assigned.
func FinanceOnly() gin.HandlerFunc {
return func(c *gin.Context) {
userID, exists := c.Get("user_id")
if !exists {
c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
c.Abort()
return
}

var user models.User
if err := database.DB.First(&user, userID).Error; err != nil {
c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
c.Abort()
return
}

// BUG-77: role was changed or account blocked since the token was issued.
	if user.Role != "admin" || user.IsBlocked {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		c.Abort()
		return
	}

	if user.AdminRole == "" || user.AdminRole == "super_admin" || user.AdminRole == "finance_manager" {
c.Next()
return
}

c.JSON(http.StatusForbidden, gin.H{"error": "Finance panel access required"})
c.Abort()
}
}
