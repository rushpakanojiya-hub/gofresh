package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetProductReviews godoc
// GET /api/v1/products/:id/reviews (public)
func GetProductReviews(c *gin.Context) {
	productID := c.Param("id")

	var reviews []models.Review
	if err := database.DB.
		Where("product_id = ?", productID).
		Order("created_at DESC").
		Find(&reviews).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load reviews"})
		return
	}

	var avgRating float64
	database.DB.Model(&models.Review{}).
		Where("product_id = ?", productID).
		Select("COALESCE(AVG(rating), 0)").
		Scan(&avgRating)

	c.JSON(http.StatusOK, gin.H{
		"reviews":        reviews,
		"average_rating": avgRating,
		"total_reviews":  len(reviews),
	})
}

// UpsertReview godoc
// POST /api/v1/products/:id/reviews (protected)
// Creates the user's review for this product, or updates it if one
// already exists (one review per user per product).
func UpsertReview(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	productIDStr := c.Param("id")
	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var req models.ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var existing models.Review
	result := database.DB.Where("user_id = ? AND product_id = ?", userID, productID).First(&existing)

	if result.Error == nil {
		existing.Rating = req.Rating
		existing.Comment = req.Comment
		if err := database.DB.Save(&existing).Error; err != nil { // BUG-70: do not ignore the save error
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update review"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Review updated", "review": existing})
		return
	}

	review := models.Review{
		UserID:    userID,
		ProductID: uint(productID),
		Rating:    req.Rating,
		Comment:   req.Comment,
	}
	if err := database.DB.Create(&review).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create review"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Review added", "review": review})
}

// DeleteReview godoc
// DELETE /api/v1/products/:id/reviews (protected)
// Deletes the logged-in user's own review for this product.
func DeleteReview(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	productID := c.Param("id")

	result := database.DB.Where("user_id = ? AND product_id = ?", userID, productID).Delete(&models.Review{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete review"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Review deleted"})
}
