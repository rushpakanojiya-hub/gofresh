package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// AuthMiddleware validates the "Authorization: Bearer <token>" header
// and injects user_id, email, role into the Gin context for handlers to use.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		claims, err := utils.ValidateJWT(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("phone", claims.Phone)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// AdminOnly restricts access to users with role "admin". Must run after AuthMiddleware.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// DeliveryPartnerOnly restricts access to users with role "delivery_partner".
// Must run after AuthMiddleware. In addition to checking the JWT role claim,
// this re-checks the partner's is_active flag against the DB on every
// request, so a partner deactivated by admin loses API access immediately -
// not only after their existing token happens to expire.
func DeliveryPartnerOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "delivery_partner" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Delivery partner access required"})
			c.Abort()
			return
		}

		partnerID, ok := c.Get("user_id")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		var partner models.DeliveryPartner
		if err := database.DB.Select("id", "is_active").First(&partner, partnerID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
			c.Abort()
			return
		}
		if !partner.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "This delivery partner account is inactive"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// WarehouseStaffOnly restricts access to users with role "warehouse_staff". Must run after AuthMiddleware.
func WarehouseStaffOnly() gin.HandlerFunc {
return func(c *gin.Context) {
role, exists := c.Get("role")
if !exists || role != "warehouse_staff" {
c.JSON(http.StatusForbidden, gin.H{"error": "Warehouse staff access required"})
c.Abort()
return
}
		staffID, _ := c.Get("user_id")
		var staff models.WarehouseStaff
		if err := database.DB.Select("id", "is_active").First(&staff, staffID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
			c.Abort()
			return
		}
		if !staff.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "This warehouse staff account is inactive"})
			c.Abort()
			return
		}
		c.Next()
}
}

// CustomerOnly restricts access to users with role "customer". Must run after AuthMiddleware.
func CustomerOnly() gin.HandlerFunc {
return func(c *gin.Context) {
role, exists := c.Get("role")
if !exists || role != "customer" {
c.JSON(http.StatusForbidden, gin.H{"error": "Customer access required"})
c.Abort()
return
}
c.Next()
}
}

// AdminOrCustomerOnly restricts access to users with role "admin" or
// "customer". Must run after AuthMiddleware. Used for endpoints like image
// upload that both an admin (product/category images) and a customer
// (return request photos) legitimately need, but warehouse staff and
// delivery partners do not.
func AdminOrCustomerOnly() gin.HandlerFunc {
return func(c *gin.Context) {
role, exists := c.Get("role")
if !exists || (role != "admin" && role != "customer") {
c.JSON(http.StatusForbidden, gin.H{"error": "Access required"})
c.Abort()
return
}
c.Next()
}
}
