package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// DeliveryPartnerAnyStatus is DeliveryPartnerOnly minus the is_active gate,
// for onboarding endpoints. A partner is blocked only if the admin
// deactivated an approved account.
func DeliveryPartnerAnyStatus() gin.HandlerFunc {
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
        if err := database.DB.Select("id", "is_active", "approval_status").First(&partner, partnerID).Error; err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
            c.Abort()
            return
        }
        if !partner.IsActive && partner.ApprovalStatus == "approved" {
            c.JSON(http.StatusForbidden, gin.H{"error": "This delivery partner account is inactive"})
            c.Abort()
            return
        }
        c.Next()
    }
}