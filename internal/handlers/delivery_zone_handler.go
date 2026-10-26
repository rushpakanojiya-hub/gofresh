package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// CreateDeliveryZone godoc
// POST /api/v1/admin/delivery-zones (admin only)
func CreateDeliveryZone(c *gin.Context) {
	var req models.CreateDeliveryZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	codAvailable := true
	if req.IsCODAvailable != nil {
		codAvailable = *req.IsCODAvailable
	}
	days := req.EstimatedDays
	if days <= 0 {
		days = 3
	}

	zone := models.DeliveryZone{
		Name:           req.Name,
		City:           req.City,
		Pincodes:       normalizePincodes(req.Pincodes),
		DeliveryCharge: req.DeliveryCharge,
		IsCODAvailable: codAvailable,
		EstimatedDays:  days,
		IsActive:       true,
	}

	if err := database.DB.Create(&zone).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create delivery zone"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_delivery_zone", "delivery_zone", strconv.Itoa(int(zone.ID)), "name: "+zone.Name)

	c.JSON(http.StatusCreated, zone)
}

// GetDeliveryZones godoc
// GET /api/v1/admin/delivery-zones (admin only)
func GetDeliveryZones(c *gin.Context) {
	var zones []models.DeliveryZone
	if err := database.DB.Order("created_at desc").Find(&zones).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch delivery zones"})
		return
	}
	c.JSON(http.StatusOK, zones)
}

// UpdateDeliveryZone godoc
// PUT /api/v1/admin/delivery-zones/:id (admin only)
func UpdateDeliveryZone(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid zone id"})
		return
	}

	var zone models.DeliveryZone
	if err := database.DB.First(&zone, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery zone not found"})
		return
	}

	var body struct {
		Name           *string  `json:"name"`
		City           *string  `json:"city"`
		Pincodes       *string  `json:"pincodes"`
		DeliveryCharge *float64 `json:"delivery_charge"`
		IsCODAvailable *bool    `json:"is_cod_available"`
		EstimatedDays  *int     `json:"estimated_days"`
		IsActive       *bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.Name != nil {
		zone.Name = *body.Name
	}
	if body.City != nil {
		zone.City = *body.City
	}
	if body.Pincodes != nil {
		zone.Pincodes = normalizePincodes(*body.Pincodes)
	}
	if body.DeliveryCharge != nil {
		zone.DeliveryCharge = *body.DeliveryCharge
	}
	if body.IsCODAvailable != nil {
		zone.IsCODAvailable = *body.IsCODAvailable
	}
	if body.EstimatedDays != nil {
		zone.EstimatedDays = *body.EstimatedDays
	}
	if body.IsActive != nil {
		zone.IsActive = *body.IsActive
	}

	if err := database.DB.Save(&zone).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update delivery zone"})
		return
	}

	c.JSON(http.StatusOK, zone)
}

// DeleteDeliveryZone godoc
// DELETE /api/v1/admin/delivery-zones/:id (admin only)
func DeleteDeliveryZone(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid zone id"})
		return
	}

	var zone models.DeliveryZone
	if err := database.DB.First(&zone, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery zone not found"})
		return
	}

	if err := database.DB.Delete(&zone).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete delivery zone"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "delete_delivery_zone", "delivery_zone", strconv.Itoa(id), "-")

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CheckPincode godoc
// GET /api/v1/delivery-zones/check?pincode=380001 (public)
// Used at checkout to determine delivery availability, charge, COD, and ETA.
func CheckPincode(c *gin.Context) {
	pincode := strings.TrimSpace(c.Query("pincode"))
	if pincode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pincode query param is required"})
		return
	}

	var zones []models.DeliveryZone
	if err := database.DB.Where("is_active = ?", true).Find(&zones).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check delivery availability"})
		return
	}

	for _, zone := range zones {
		codes := strings.Split(zone.Pincodes, ",")
		for _, code := range codes {
			if strings.TrimSpace(code) == pincode {
				c.JSON(http.StatusOK, gin.H{
					"deliverable":      true,
					"zone_name":        zone.Name,
					"delivery_charge":  zone.DeliveryCharge,
					"is_cod_available": zone.IsCODAvailable,
					"estimated_days":   zone.EstimatedDays,
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"deliverable": false,
		"message":     "Delivery not available for this pincode",
	})
}

// normalizePincodes trims whitespace around each comma-separated pincode.
func normalizePincodes(raw string) string {
	parts := strings.Split(raw, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, ",")
}
