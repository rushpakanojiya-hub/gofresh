package middleware

import (
"net/http"

"github.com/gin-gonic/gin"
)

// RequireWarehouseRole restricts access to warehouse staff whose Role
// (injected by InjectWarehouseScope as "staff_role") is one of the given
// values. Must run after InjectWarehouseScope.
//
// This is server-side enforcement, not a frontend convenience: a picker's
// token used directly against, say, PUT /warehouse/batches/:id/quantity
// must fail here even if the picker's app never shows that button.
func RequireWarehouseRole(roles ...string) gin.HandlerFunc {
allowed := make(map[string]bool, len(roles))
for _, r := range roles {
allowed[r] = true
}
return func(c *gin.Context) {
role, exists := c.Get("staff_role")
if !exists {
c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
c.Abort()
return
}
if !allowed[role.(string)] {
c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to perform this action"})
c.Abort()
return
}
c.Next()
}
}

// Role groups used across warehouse routes. Keeping these as named slices
// (rather than repeating string literals at every route) is what keeps the
// permission matrix auditable in one place.
var (
RoleManagement   = []string{"warehouse_manager", "supervisor"}
RoleInventoryOps = []string{"warehouse_manager", "supervisor", "inventory_staff"}
RolePickers      = []string{"warehouse_manager", "supervisor", "picker"}
RolePackers      = []string{"warehouse_manager", "supervisor", "packer"}
RolePutters      = []string{"warehouse_manager", "supervisor", "putter"}
RoleAnyStaff     = []string{"warehouse_manager", "supervisor", "picker", "packer", "inventory_staff", "putter"}
)
