package handlers

import (
"net/http"
"strings"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm/clause"
)

type RegisterDeviceTokenRequest struct {
Token    string `json:"token" binding:"required"`
Platform string `json:"platform"`
}

func RegisterDeviceToken(c *gin.Context) {
var req RegisterDeviceTokenRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var userID *uint
var deliveryPartnerID *uint
authHeader := c.GetHeader("Authorization")
if authHeader != "" {
parts := strings.Split(authHeader, " ")
if len(parts) == 2 && parts[0] == "Bearer" {
if claims, err := utils.ValidateJWT(parts[1]); err == nil {
id := claims.UserID
if claims.Role == "delivery_partner" {
deliveryPartnerID = &id
} else {
userID = &id
}
}
}
}

// Atomic upsert: two near-simultaneous requests for the same token
// (e.g. PushService.start() and a post-login re-register both firing at
// once) must not race on a select-then-insert - that let the second
// request lose a duplicate-key error instead of updating the row. On
// conflict we overwrite user_id/delivery_partner_id from the CURRENT
// auth context (nil included), so a token reused across roles never
// keeps a stale pointer to the previous role.
token := models.DeviceToken{
Token:             req.Token,
Platform:          req.Platform,
UserID:            userID,
DeliveryPartnerID: deliveryPartnerID,
}
err := database.DB.Clauses(clause.OnConflict{
Columns:   []clause.Column{{Name: "token"}},
DoUpdates: clause.AssignmentColumns([]string{"platform", "user_id", "delivery_partner_id", "updated_at"}),
}).Create(&token).Error
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save device token"})
return
}
c.JSON(http.StatusOK, gin.H{"message": "Token registered"})
}

