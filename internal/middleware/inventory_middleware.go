package middleware

import (
"net/http"

"github.com/gin-gonic/gin"
)

// InventoryManagerOnly restricts stock-modifying actions (adjust, damage,
// expire, receive, transfer, zone/rack/bin management) to warehouse staff
// whose specific role is inventory-facing. Pickers and packers (Store App
// staff) can still read inventory via the plain WarehouseStaffOnly routes,
// but are blocked from these write endpoints.
// Must run after AuthMiddleware + WarehouseStaffOnly + InjectWarehouseScope,
// since it relies on "staff_role" set by InjectWarehouseScope.
func InventoryManagerOnly() gin.HandlerFunc {
allowedRoles := map[string]bool{
"warehouse_manager": true,
"inventory_staff":   true,
"supervisor":        true,
}
return func(c *gin.Context) {
staffRole, exists := c.Get("staff_role")
if !exists || !allowedRoles[staffRole.(string)] {
c.JSON(http.StatusForbidden, gin.H{"error": "Inventory management access required"})
c.Abort()
return
}
c.Next()
}
}
