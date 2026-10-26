package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetWishlist godoc
// GET /api/v1/wishlist (protected)
func GetWishlist(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var items []models.Wishlist
	if err := database.DB.
		Preload("Product").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load wishlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"wishlist": items})
}

// AddToWishlist godoc
// POST /api/v1/wishlist (protected)
func AddToWishlist(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var req models.WishlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var product models.Product
	if err := database.DB.First(&product, req.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var existing models.Wishlist
	result := database.DB.Where("user_id = ? AND product_id = ?", userID, req.ProductID).First(&existing)
	if result.Error == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Already in wishlist"})
		return
	}

	item := models.Wishlist{
		UserID:    userID,
		ProductID: req.ProductID,
	}
	if err := database.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to wishlist"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Added to wishlist"})
}

// RemoveFromWishlist godoc
// DELETE /api/v1/wishlist/:product_id (protected)
func RemoveFromWishlist(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	productID := c.Param("product_id")

	result := database.DB.Where("user_id = ? AND product_id = ?", userID, productID).Delete(&models.Wishlist{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove from wishlist"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found in wishlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Removed from wishlist"})
}
