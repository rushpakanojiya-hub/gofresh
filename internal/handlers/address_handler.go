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

// CreateAddress godoc
// POST /api/v1/addresses (protected)
// The first address a user adds is automatically made the default one.
// BUG-69: deleted addresses are ignored by the count, update, delete and set-default paths.
func CreateAddress(c *gin.Context) {
userID := c.MustGet("user_id").(uint)

var req models.AddressRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var existingCount int64
database.DB.Model(&models.Address{}).Where("user_id = ? AND is_deleted = false", userID).Count(&existingCount)
makeDefault := req.IsDefault || existingCount == 0

if makeDefault {
database.DB.Model(&models.Address{}).
Where("user_id = ? AND is_default = ?", userID, true).
Update("is_default", false)
}

address := models.Address{
UserID:    userID,
Label:     req.Label,
FullName:  req.FullName,
Phone:     req.Phone,
Line1:     req.Line1,
Line2:     req.Line2,
City:      req.City,
State:     req.State,
Pincode:   req.Pincode,
Lat:       req.Lat,
Lng:       req.Lng,
IsDefault: makeDefault,
}
if err := database.DB.Create(&address).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
return
}

c.JSON(http.StatusCreated, address)
}

// UpdateAddress godoc
// PUT /api/v1/addresses/:id (protected)
func UpdateAddress(c *gin.Context) {
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

var req models.AddressRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

if req.IsDefault && !address.IsDefault {
database.DB.Model(&models.Address{}).
Where("user_id = ? AND is_default = ?", userID, true).
Update("is_default", false)
}

address.Label = req.Label
address.FullName = req.FullName
address.Phone = req.Phone
address.Line1 = req.Line1
address.Line2 = req.Line2
address.City = req.City
address.State = req.State
address.Pincode = req.Pincode
if req.Lat != nil {
address.Lat = req.Lat
}
if req.Lng != nil {
address.Lng = req.Lng
}
address.IsDefault = req.IsDefault || address.IsDefault

if err := database.DB.Save(&address).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
return
}

c.JSON(http.StatusOK, address)
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

// SetDefaultAddress godoc
// PUT /api/v1/addresses/:id/default (protected)
func SetDefaultAddress(c *gin.Context) {
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

database.DB.Model(&models.Address{}).
Where("user_id = ? AND is_default = ?", userID, true).
Update("is_default", false)

address.IsDefault = true
if err := database.DB.Save(&address).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default address"})
return
}

c.JSON(http.StatusOK, address)
}


