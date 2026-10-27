package handlers

import (
"net/http"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// ListAddresses godoc
// GET /api/v1/addresses (protected)
func ListAddresses(c *gin.Context) {
userID := c.MustGet("user_id").(uint)

var addresses []models.Address
if err := database.DB.
Where("user_id = ? AND is_deleted = false", userID).
Order("is_default DESC, created_at DESC").
Find(&addresses).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load addresses"})
return
}

c.JSON(http.StatusOK, gin.H{"addresses": addresses})
}

// DeleteAddress godoc
// DELETE /api/v1/addresses/:id (protected)
// If the deleted address was the default, the most recently created
// remaining address (if any) is promoted to default.
func DeleteAddress(c *gin.Context) {
userID := c.MustGet("user_id").(uint)
addressID := c.Param("id")

var address models.Address
if err := database.DB.Where("is_deleted = false").First(&address, addressID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
return
}
if address.UserID != userID {
c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this address"})
return
}

// BUG-69: remember the default flag, then clear it on the deleted row.
	wasDefault := address.IsDefault
	if err := database.DB.Model(&address).Updates(map[string]interface{}{"is_deleted": true, "is_default": false}).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete address"})
return
}

if wasDefault {
var nextAddress models.Address
if err := database.DB.
Where("user_id = ? AND is_deleted = false", userID).
Order("created_at DESC").
First(&nextAddress).Error; err == nil {
database.DB.Model(&nextAddress).Update("is_default", true)
}
}

c.JSON(http.StatusOK, gin.H{"message": "Address deleted successfully"})
}

