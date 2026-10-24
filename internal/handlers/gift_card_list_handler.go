package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetMyGiftCards GET /api/v1/gift-cards (cards redeemed by the logged-in user)
func GetMyGiftCards(c *gin.Context) {
	uid, ok := giftCardUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var cards []models.GiftCard
	if err := database.DB.Where("redeemed_by_user_id = ?", uid).Order("redeemed_at DESC").Find(&cards).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load gift cards"})
		return
	}
	out := make([]gin.H, 0, len(cards))
	for _, g := range cards {
		out = append(out, gin.H{
			"id":          fmt.Sprint(g.ID),
			"card_number": g.Code,
			"balance": 0, // BUG-42: redeemed card has no balance left,
			"expiry_date": g.ExpiresAt.Format(time.RFC3339),
			"status":      "redeemed",
		})
	}
	c.JSON(http.StatusOK, out)
}
