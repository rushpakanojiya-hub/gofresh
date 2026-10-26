package middleware

import (
"net/http"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// Permission keys used across the admin panel. Keep flat and explicit -
// each string maps to one sensitive action.
const (
PermApproveRefund   = "refund:approve"
PermBlockCustomer   = "customer:block"
PermManageStaff     = "staff:manage"
PermEditPrice       = "product:price_edit"
PermDeleteCoupon    = "coupon:delete"
	PermManageSettings  = "settings:manage"
)

// rolePermissions maps each admin sub-role to the set of permissions it holds.
// super_admin implicitly has everything (checked separately below).
var rolePermissions = map[string]map[string]bool{
"admin": {
PermApproveRefund:  true,
PermBlockCustomer:  true,
PermManageStaff:    true,
PermEditPrice:      true,
PermDeleteCoupon:   true,
PermManageSettings: true,
},
"ops_manager": {
PermBlockCustomer: true,
PermEditPrice:     true,
},
"warehouse_manager": {},
"support_agent": {
PermBlockCustomer: true,
},
"finance_manager": {
PermApproveRefund: true,
},
}

// RequirePermission enforces that the authenticated admin's sub-role grants
// the given permission. Must run after AuthMiddleware + AdminOnly.
// super_admin (or an admin user with no admin_role set, for backward
// compatibility with existing admin accounts) always passes.
func RequirePermission(permission string) gin.HandlerFunc {
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

	// super_admin, or legacy admin accounts with no sub-role set yet,
// retain full access so existing admins aren't locked out.
if user.AdminRole == "" || user.AdminRole == "super_admin" {
c.Next()
return
}

perms, ok := rolePermissions[user.AdminRole]
if !ok || !perms[permission] {
c.JSON(http.StatusForbidden, gin.H{"error": "Your role does not have permission to perform this action"})
c.Abort()
return
}

c.Next()
}
}
