package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
)

// SendPartnerOTP godoc
// POST /api/v1/delivery/send-otp
// Sends an OTP to a phone number that belongs to an existing, active
// delivery partner (created by the admin). Unlike customer signup, this
// does NOT create a new partner — the admin must add them first.
func SendPartnerOTP(c *gin.Context) {
	var req models.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var partner models.DeliveryPartner
	err := database.DB.Where("phone = ?", req.Phone).First(&partner).Error
    if err != nil && err != gorm.ErrRecordNotFound {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to look up partner"})
        return
    }
    // Unknown numbers may self-signup (partner row is created after OTP verify).
    if err == nil && partnerLoginBlocked(partner) {
		c.JSON(http.StatusForbidden, gin.H{"error": "This delivery partner account is inactive"})
		return
	}

	code, err := generateOTP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	database.DB.Where("phone = ? AND verified = ?", req.Phone, false).Delete(&models.OTP{})
	otp := models.OTP{
		Phone:     req.Phone,
		Code:      code,
		ExpiresAt: time.Now().Add(otpValidityMinutes * time.Minute),
		Verified:  false,
	}
	if err := database.DB.Create(&otp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create OTP"})
		return
	}

	c.JSON(http.StatusOK, otpDebugResponse(code, req.Phone))
}

// VerifyPartnerOTP godoc
// POST /api/v1/delivery/verify-otp
// Verifies the OTP and returns a JWT with role "delivery_partner".
func VerifyPartnerOTP(c *gin.Context) {
	var req models.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var otp models.OTP
	err := database.DB.
		Where("phone = ? AND verified = ?", req.Phone, false).
		Order("created_at DESC").
		First(&otp).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OTP"})
		return
	}
	if otp.Attempts >= 5 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many incorrect attempts. Please request a new OTP."})
		return
	}
	if time.Now().After(otp.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OTP has expired, please request a new one"})
		return
	}
	if otp.Code != req.OTP {
		database.DB.Model(&models.OTP{}).Where("id = ?", otp.ID).Update("attempts", gorm.Expr("attempts + 1"))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OTP"})
		return
	}
	database.DB.Model(&otp).Update("verified", true)

	var partner models.DeliveryPartner
	if err := database.DB.Where("phone = ?", req.Phone).First(&partner).Error; err != nil {
        if err != gorm.ErrRecordNotFound {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to look up partner"})
            return
        }
        partner = models.DeliveryPartner{Name: "New Partner", Phone: req.Phone, IsActive: false, ApprovalStatus: approvalOnboarding}
        if err := database.DB.Create(&partner).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create partner"})
            return
        }
        // gorm applies the column default (5) to a zero value on Create.
        database.DB.Model(&partner).Update("onboarding_step", 0)
        partner.OnboardingStep = 0
    }
    if partnerLoginBlocked(partner) {
		c.JSON(http.StatusForbidden, gin.H{"error": "This delivery partner account is inactive"})
		return
	}

	token, err := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "delivery_partner": partner})
}

// UpdateLocation godoc
// PUT /api/v1/delivery/location (delivery partner only)
// The partner's app calls this periodically (e.g. every 15-30s) while out
// for delivery, to push their current GPS coordinates. The partner is
// identified solely from the verified JWT ("user_id") - there is no
// partner_id in the request body, so a partner can never update anyone
// else's location.
func UpdateLocation(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	var req models.UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Lat < -90 || req.Lat > 90 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat must be between -90 and 90"})
		return
	}
	if req.Lng < -180 || req.Lng > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lng must be between -180 and 180"})
		return
	}

	now := time.Now()
	result := database.DB.Model(&models.DeliveryPartner{}).
		Where("id = ?", partnerID).
		Updates(map[string]interface{}{
			"current_lat":          req.Lat,
			"current_lng":          req.Lng,
			"last_location_update": now,
		})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Location updated"})
}

// trackingPayload builds the customer/admin-facing tracking response for a
// preloaded order+partner. Shared by GetOrderTracking (customer) and
// GetOrderTrackingAdmin (admin) so both surfaces stay consistent.
func trackingPayload(order models.Order) gin.H {
partner := order.DeliveryPartner
payload := gin.H{
"delivery_partner_name": partner.Name,
"vehicle_number":        partner.VehicleNumber,
"phone":                 partner.Phone,
"current_lat":           partner.CurrentLat,
"current_lng":           partner.CurrentLng,
"last_updated":          partner.LastLocationUpdate,
"order_status":          order.Status,
"delivery_status":       order.DeliveryStatus,
"address_lat":           order.Address.Lat,
"address_lng":           order.Address.Lng,
}
return payload
}

// GetOrderTracking godoc
// GET /api/v1/orders/:id/tracking (protected — order owner only)
// Returns the assigned delivery partner's current live location, so the
// customer app can show them on a map. Distinguishes three cases: no
// partner assigned yet, a partner assigned but with no location reported
// yet, and a partner with a live location.
func GetOrderTracking(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	orderID := c.Param("id")

	var order models.Order
	if err := database.DB.
		Preload("DeliveryPartner").
                Preload("Address").
		Where("id = ? AND user_id = ?", orderID, userID).
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.DeliveryPartner == nil {
		c.JSON(http.StatusOK, gin.H{"message": "No delivery partner assigned yet", "tracking": nil})
		return
	}

	trackDS := ""
    if order.DeliveryStatus != nil {
        trackDS = *order.DeliveryStatus
    }
    // Rider ki live location sirf tab share hoti hai jab delivery leg chal rahi ho.
    // Warna delivered/rejected/expired ke baad bhi customer us partner ko track karta rehta.
    if trackDS != models.DeliveryStatusOutForDelivery && trackDS != models.DeliveryStatusArrivedAtCustomer {
        payload := trackingPayload(order)
        payload["current_lat"] = nil
        payload["current_lng"] = nil
        payload["last_updated"] = nil
        c.JSON(http.StatusOK, gin.H{"message": "Live location is shared once your order is out for delivery", "tracking": payload})
        return
    }
	if order.DeliveryPartner.CurrentLat == nil || order.DeliveryPartner.CurrentLng == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Delivery partner assigned, live location not available yet", "tracking": trackingPayload(order)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tracking": trackingPayload(order)})
}

// GetOrderTrackingAdmin godoc
// GET /api/v1/admin/orders/:id/tracking (admin only)
// Same as GetOrderTracking but for admin/ops - not scoped to a customer's
// own orders.
func GetOrderTrackingAdmin(c *gin.Context) {
	orderID := c.Param("id")

	var order models.Order
	if err := database.DB.
		Preload("DeliveryPartner").
                Preload("Address").
		Where("id = ?", orderID).
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.DeliveryPartner == nil {
		c.JSON(http.StatusOK, gin.H{"message": "No delivery partner assigned yet", "tracking": nil})
		return
	}

	if order.DeliveryPartner.CurrentLat == nil || order.DeliveryPartner.CurrentLng == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Delivery partner assigned, live location not available yet", "tracking": trackingPayload(order)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tracking": trackingPayload(order)})
}
