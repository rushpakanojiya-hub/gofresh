package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

func storeCheckinWarehouseID(c *gin.Context) (uint, bool) {
	v, ok := c.Get("warehouse_id")
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case uint:
		return x, true
	case *uint:
		if x != nil {
			return *x, true
		}
	}
	return 0, false
}

// GetStoreCheckinQR: GET /warehouse/checkin-qr (warehouse staff)
// Returns a short-lived signed token; the store app renders it as a QR.
func GetStoreCheckinQR(c *gin.Context) {
	wid, ok := storeCheckinWarehouseID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Warehouse not found for this staff"})
		return
	}
	token, exp := services.GenerateStoreCheckinToken(wid)
	c.JSON(http.StatusOK, gin.H{
		"token":        token,
		"expires_at":   exp,
		"ttl_seconds":  int(services.StoreQRTTL.Seconds()),
		"warehouse_id": wid,
	})
}

// CheckInToStore: POST /delivery/checkin (delivery partner)
func CheckInToStore(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	var req struct {
		Token string   `json:"token" binding:"required"`
		Lat   *float64 `json:"lat"`
		Lng   *float64 `json:"lng"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	wh, err := services.CheckInPartner(partnerID, req.Token, req.Lat, req.Lng)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, services.ErrCheckinInvalidToken), errors.Is(err, services.ErrCheckinNoLocation):
			status = http.StatusBadRequest
		case errors.Is(err, services.ErrCheckinWrongStore), errors.Is(err, services.ErrCheckinTooFar):
			status = http.StatusForbidden
		case errors.Is(err, services.ErrCheckinNotFound):
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	go services.TryAssignPendingOrdersToPartner(partnerID)
	go services.TryAssignPendingReturnsToPartner(partnerID)
	c.JSON(http.StatusOK, gin.H{
		"checked_in":     true,
		"warehouse_id":   wh.ID,
		"warehouse_name": wh.Name,
	})
}

// GetMyCheckin: GET /delivery/checkin (delivery partner)
func GetMyCheckin(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	var p models.DeliveryPartner
	if err := database.DB.First(&p, partnerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}
	if p.CheckedInWarehouseID == nil || p.CheckedInAt == nil ||
		time.Since(*p.CheckedInAt) > services.StoreCheckinMaxAge {
		c.JSON(http.StatusOK, gin.H{"checked_in": false})
		return
	}
	var wh models.Warehouse
	database.DB.First(&wh, *p.CheckedInWarehouseID)
	c.JSON(http.StatusOK, gin.H{
		"checked_in":     true,
		"warehouse_id":   *p.CheckedInWarehouseID,
		"warehouse_name": wh.Name,
		"checked_in_at":  *p.CheckedInAt,
	})
}
