package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// CreateOffer godoc
// POST /api/v1/admin/offers (admin only)
func CreateOffer(c *gin.Context) {
	var req models.CreateOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date, use YYYY-MM-DD"})
		return
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date, use YYYY-MM-DD"})
		return
	}
	// BUG-37: end_date is inclusive - run through the end of that day.
	end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	if end.Before(start) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date must be after start_date"})
		return
	}

	offer := models.Offer{
		Title:        req.Title,
		Description:  req.Description,
		ImageURL:     req.ImageURL,
		DiscountText: req.DiscountText,
		CategoryID:   req.CategoryID,
		StartDate:    start,
		EndDate:      end,
		IsActive:     true,
	}

	if err := database.DB.Create(&offer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create offer"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_offer", "offer", strconv.Itoa(int(offer.ID)), "title: "+offer.Title)

	c.JSON(http.StatusCreated, offer)
}

// GetOffers godoc
// GET /api/v1/admin/offers (admin only) - returns all offers regardless of status/dates
func GetOffers(c *gin.Context) {
	var offers []models.Offer
	if err := database.DB.Order("created_at desc").Find(&offers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch offers"})
		return
	}
	c.JSON(http.StatusOK, offers)
}

// GetActiveOffers godoc
// GET /api/v1/offers (public) - returns only active, currently-running offers
// for the storefront to display.
func GetActiveOffers(c *gin.Context) {
	now := time.Now()
	var offers []models.Offer
	if err := database.DB.Where("is_active = ? AND start_date <= ? AND end_date >= ?", true, now, now).
		Order("created_at desc").Find(&offers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch offers"})
		return
	}
	c.JSON(http.StatusOK, offers)
}

// UpdateOfferStatus godoc
// PUT /api/v1/admin/offers/:id/status (admin only)
// body: { "is_active": true }
func UpdateOfferStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
		return
	}

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var offer models.Offer
	if err := database.DB.First(&offer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Offer not found"})
		return
	}
	offer.IsActive = body.IsActive
	if err := database.DB.Save(&offer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update offer"})
		return
	}

	c.JSON(http.StatusOK, offer)
}

// DeleteOffer godoc
// DELETE /api/v1/admin/offers/:id (admin only)
func DeleteOffer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
		return
	}

	var offer models.Offer
	if err := database.DB.First(&offer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Offer not found"})
		return
	}

	if err := database.DB.Delete(&offer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete offer"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "delete_offer", "offer", strconv.Itoa(id), "-")

	c.JSON(http.StatusOK, gin.H{"success": true})
}
