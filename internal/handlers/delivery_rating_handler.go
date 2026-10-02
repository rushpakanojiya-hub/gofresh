package handlers

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// DeliveryRatingRecord is one customer rating for one delivered order.
// order_id is UNIQUE in the DB, so an order can only be rated once.
type DeliveryRatingRecord struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	OrderID           uint      `gorm:"not null;uniqueIndex" json:"order_id"`
	DeliveryPartnerID uint      `gorm:"not null;index" json:"delivery_partner_id"`
	UserID            uint      `gorm:"not null" json:"user_id"`
	Rating            int       `gorm:"not null" json:"rating"`
	Review            string    `json:"review"`
	CreatedAt         time.Time `json:"created_at"`
}

func (DeliveryRatingRecord) TableName() string { return "delivery_ratings" }

// loadOwnOrderForRating loads the order only if it belongs to the caller.
func loadOwnOrderForRating(c *gin.Context) (*models.Order, bool) {
	userID := c.MustGet("user_id").(uint)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return nil, false
	}
	var order models.Order
	if err := database.DB.
		Select("id", "user_id", "status", "delivery_partner_id").
		Where("id = ? AND user_id = ?", orderID, userID).
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return nil, false
	}
	return &order, true
}

// SubmitDeliveryRating: POST /orders/:id/rating  {rating: 1-5, review?: string}
func SubmitDeliveryRating(c *gin.Context) {
	order, ok := loadOwnOrderForRating(c)
	if !ok {
		return
	}
	var req struct {
		Rating int    `json:"rating" binding:"required,min=1,max=5"`
		Review string `json:"review"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating must be between 1 and 5"})
		return
	}
	review := strings.TrimSpace(req.Review)
	if len([]rune(review)) > 300 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "review must be 300 characters or less"})
		return
	}
	if order.Status != "delivered" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only delivered orders can be rated"})
		return
	}
	if order.DeliveryPartnerID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no delivery partner assigned to this order"})
		return
	}

	rec := DeliveryRatingRecord{
		OrderID:           order.ID,
		DeliveryPartnerID: *order.DeliveryPartnerID,
		UserID:            order.UserID,
		Rating:            req.Rating,
		Review:            review,
	}
	if err := database.DB.Create(&rec).Error; err != nil {
		var n int64
		database.DB.Model(&DeliveryRatingRecord{}).Where("order_id = ?", order.ID).Count(&n)
		if n > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "this order is already rated"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save rating"})
		return
	}
	go services.SendPushToPartnerWithData(rec.DeliveryPartnerID, "New Customer Rating", "A customer has rated your delivery. Tap to view your rating.", map[string]string{"type": "rating", "order_id": strconv.FormatUint(uint64(rec.OrderID), 10)})
	c.JSON(http.StatusCreated, gin.H{"message": "rating submitted", "rating": rec.Rating, "review": rec.Review})
}

// GetDeliveryRatingForOrder: GET /orders/:id/rating
func GetDeliveryRatingForOrder(c *gin.Context) {
	order, ok := loadOwnOrderForRating(c)
	if !ok {
		return
	}
	var rec DeliveryRatingRecord
	err := database.DB.Where("order_id = ?", order.ID).First(&rec).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"rated": true, "can_rate": false, "rating": rec.Rating, "review": rec.Review})
		return
	}
	canRate := order.Status == "delivered" && order.DeliveryPartnerID != nil
	c.JSON(http.StatusOK, gin.H{"rated": false, "can_rate": canRate})
}

// GetMyDeliveryRating: GET /delivery/rating  (delivery partner only)
// Always aggregated from the DB so a new rating is reflected immediately.
func GetMyDeliveryRating(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	var res struct {
		Avg   float64
		Count int64
	}
	if err := database.DB.Raw(
		"SELECT COALESCE(AVG(rating), 0) AS avg, COUNT(*) AS count FROM delivery_ratings WHERE delivery_partner_id = ?",
		partnerID,
	).Scan(&res).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load rating"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"avg_rating":   math.Round(res.Avg*10) / 10,
		"rating_count": res.Count,
	})
}

// partnerRatingAgg is one row of the per-partner rating aggregate.
type partnerRatingAgg struct {
	DeliveryPartnerID uint
	Avg               float64
	Cnt               int64
}

// deliveryPartnerWithRating is a DeliveryPartner plus its rating summary.
// The embedded struct's JSON fields stay flat, so existing consumers are unaffected.
type deliveryPartnerWithRating struct {
	models.DeliveryPartner
	AvgRating   float64 `json:"avg_rating"`
	RatingCount int64   `json:"rating_count"`
}

// buildPartnersWithRatings attaches avg rating + count to every partner using
// one grouped query (no per-partner queries).
func buildPartnersWithRatings(partners []models.DeliveryPartner) []deliveryPartnerWithRating {
	var rows []partnerRatingAgg
	database.DB.Raw(
		"SELECT delivery_partner_id, AVG(rating) AS avg, COUNT(*) AS cnt FROM delivery_ratings GROUP BY delivery_partner_id",
	).Scan(&rows)
	byID := make(map[uint]partnerRatingAgg, len(rows))
	for _, r := range rows {
		byID[r.DeliveryPartnerID] = r
	}
	out := make([]deliveryPartnerWithRating, 0, len(partners))
	for _, p := range partners {
		a := byID[p.ID]
		out = append(out, deliveryPartnerWithRating{
			DeliveryPartner: p,
			AvgRating:       math.Round(a.Avg*10) / 10,
			RatingCount:     a.Cnt,
		})
	}
	return out
}

// GetDeliveryOrderRating: GET /delivery/orders/:id/rating (delivery partner only)
// Returns the customer's rating for an order only if it belongs to the caller.
func GetDeliveryOrderRating(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	var rec DeliveryRatingRecord
	if err := database.DB.
		Where("order_id = ? AND delivery_partner_id = ?", orderID, partnerID).
		First(&rec).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"rated": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rated": true, "rating": rec.Rating, "review": rec.Review})
}

// GetMyDeliveryRatings: GET /delivery/ratings  (delivery partner only)
// Every rating this partner received, newest first, with the order id.
func GetMyDeliveryRatings(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	var recs []DeliveryRatingRecord
	if err := database.DB.
		Where("delivery_partner_id = ?", partnerID).
		Order("created_at DESC").
		Limit(200).
		Find(&recs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load ratings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ratings": recs})
}
