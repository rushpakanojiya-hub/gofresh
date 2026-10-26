package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// CreateBanner godoc
// POST /api/v1/admin/banners (admin only)
func CreateBanner(c *gin.Context) {
	var req models.CreateBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.LinkType == "" {
		req.LinkType = "none"
	}

	banner := models.Banner{
		ImageURL:     req.ImageURL,
		Title:        req.Title,
		LinkType:     req.LinkType,
		LinkValue:    req.LinkValue,
		DisplayOrder: req.DisplayOrder,
		IsActive:     true,
	}

	if err := database.DB.Create(&banner).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create banner"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_banner", "banner", strconv.Itoa(int(banner.ID)), "title: "+banner.Title)

	c.JSON(http.StatusCreated, banner)
}

// GetBanners godoc
// GET /api/v1/admin/banners (admin only) - all banners regardless of status
func GetBanners(c *gin.Context) {
	var banners []models.Banner
	if err := database.DB.Order("display_order asc, created_at desc").Find(&banners).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch banners"})
		return
	}
	c.JSON(http.StatusOK, banners)
}

// GetActiveBanners godoc
// GET /api/v1/banners (public) - active banners ordered for storefront carousel
func GetActiveBanners(c *gin.Context) {
	var banners []models.Banner
	if err := database.DB.Where("is_active = ?", true).
		Order("display_order asc, created_at desc").Find(&banners).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch banners"})
		return
	}
	c.JSON(http.StatusOK, banners)
}

// UpdateBanner godoc
// PUT /api/v1/admin/banners/:id (admin only)
// body: any subset of { image_url, title, link_type, link_value, display_order, is_active }
func UpdateBanner(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid banner id"})
		return
	}

	var banner models.Banner
	if err := database.DB.First(&banner, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Banner not found"})
		return
	}

	var body struct {
		ImageURL     *string `json:"image_url"`
		Title        *string `json:"title"`
		LinkType     *string `json:"link_type"`
		LinkValue    *string `json:"link_value"`
		DisplayOrder *int    `json:"display_order"`
		IsActive     *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.ImageURL != nil {
		banner.ImageURL = *body.ImageURL
	}
	if body.Title != nil {
		banner.Title = *body.Title
	}
	if body.LinkType != nil {
		banner.LinkType = *body.LinkType
	}
	if body.LinkValue != nil {
		banner.LinkValue = *body.LinkValue
	}
	if body.DisplayOrder != nil {
		banner.DisplayOrder = *body.DisplayOrder
	}
	if body.IsActive != nil {
		banner.IsActive = *body.IsActive
	}

	if err := database.DB.Save(&banner).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update banner"})
		return
	}

	c.JSON(http.StatusOK, banner)
}

// DeleteBanner godoc
// DELETE /api/v1/admin/banners/:id (admin only)
func DeleteBanner(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid banner id"})
		return
	}

	var banner models.Banner
	if err := database.DB.First(&banner, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Banner not found"})
		return
	}

	if err := database.DB.Delete(&banner).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete banner"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "delete_banner", "banner", strconv.Itoa(id), "-")

	c.JSON(http.StatusOK, gin.H{"success": true})
}
