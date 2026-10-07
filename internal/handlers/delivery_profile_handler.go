package handlers

import (
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// ---------------------------------------------------------------------------
// Delivery Partner Profile + Availability (Phase 2, delivery partner only)
// ---------------------------------------------------------------------------
//
// Every handler below identifies the delivery partner exclusively from the
// "user_id" set by AuthMiddleware from the verified JWT. No handler ever
// accepts a partner/delivery-boy ID from the request body, query string, or
// URL param, so a partner can never read or modify another partner's data
// (no IDOR/BOLA). Routes are additionally guarded by
// middleware.DeliveryPartnerOnly() so only accounts with role
// "delivery_partner" can reach them.

// GetDeliveryProfile godoc
// GET /api/v1/delivery/profile (delivery partner only)
// Returns the authenticated delivery partner's own profile. Only fields
// relevant to the partner themselves are returned - no password/OTP/token
// data exists on this model, and none is ever selected here.
func GetDeliveryProfile(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	var partner models.DeliveryPartner
	if err := database.DB.First(&partner, partnerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             partner.ID,
		"name":           partner.Name,
		"phone":          partner.Phone,
		"vehicle_number": partner.VehicleNumber,
		// account_status reflects the admin-controlled IsActive flag.
		// "Assigned warehouse" and "email"/"profile photo" are not part of
		// the current delivery-partner architecture (no warehouse_id,
		// email, or photo column exists on this model), so they're
		// intentionally omitted rather than fabricated here.
		"vehicle_type":      partner.VehicleType,
		"profile_photo_url": partner.ProfilePhotoURL,
		"warehouse_id":      partner.WarehouseID,
		"onboarding_step":   partner.OnboardingStep,
		"approval_status":   partner.ApprovalStatus,
		"account_status":    accountStatusLabel(partner.IsActive),
		"is_online":         partner.IsOnline,
		"created_at":        partner.CreatedAt,
	})
}

// accountStatusLabel maps the IsActive flag to a human-readable status.
func accountStatusLabel(isActive bool) string {
	if isActive {
		return "active"
	}
	return "inactive"
}

// UpdateDeliveryProfile godoc
// PUT /api/v1/delivery/profile (delivery partner only)
// Lets the authenticated delivery partner update their own non-sensitive
// profile fields. Phone (login identity), IsActive (account status),
// role, and any future warehouse-assignment field are never accepted here
// - only name and vehicle_number are bindable, so there is no way for a
// client to smuggle in a protected field even if they add it to the
// request body.
func UpdateDeliveryProfile(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	var req models.UpdateDeliveryProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var partner models.DeliveryPartner
	if err := database.DB.First(&partner, partnerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}

	partner.Name = req.Name
	partner.VehicleNumber = req.VehicleNumber

	if err := database.DB.Model(&partner).
		Select("name", "vehicle_number").
		Updates(models.DeliveryPartner{Name: partner.Name, VehicleNumber: partner.VehicleNumber}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             partner.ID,
		"name":           partner.Name,
		"phone":          partner.Phone,
		"vehicle_number": partner.VehicleNumber,
		"account_status": accountStatusLabel(partner.IsActive),
		"is_online":      partner.IsOnline,
	})
}

// GetDeliveryAvailability godoc
// GET /api/v1/delivery/availability (delivery partner only)
// Returns the authenticated delivery partner's current online/offline
// availability.
func GetDeliveryAvailability(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	var partner models.DeliveryPartner
	if err := database.DB.First(&partner, partnerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": availabilityLabel(partner.IsOnline), "is_online": partner.IsOnline})
}

// availabilityLabel maps the IsOnline flag to the "online"/"offline" string
// used on the wire.
func availabilityLabel(isOnline bool) string {
	if isOnline {
		return "online"
	}
	return "offline"
}

// UpdateDeliveryAvailability godoc
// PUT /api/v1/delivery/availability (delivery partner only)
// Lets the authenticated delivery partner switch themselves ONLINE or
// OFFLINE. This only flips the availability flag used to gate *new*
// assignment eligibility - it never touches orders already assigned to
// the partner, and it never creates/removes any assignment itself (the
// order-assignment architecture that will consume this flag is a
// separate, later phase).
func UpdateDeliveryAvailability(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	var req models.UpdateAvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid availability status: must be 'online' or 'offline'"})
		return
	}

	isOnline := req.Status == "online"

	if isOnline && !partnerHasActiveGig(partnerID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Book a gig slot to go online. You can only go online during a booked gig.",
			"code":  "GIG_REQUIRED",
		})
		return
	}
	result := database.DB.Model(&models.DeliveryPartner{}).
		Where("id = ?", partnerID).
		Update("is_online", isOnline)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update availability"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}

	if !isOnline {
		_ = services.ClearPartnerCheckin(partnerID)
	}

	if isOnline {
		go services.TryAssignPendingOrdersToPartner(partnerID)
	}

	c.JSON(http.StatusOK, gin.H{"status": availabilityLabel(isOnline), "is_online": isOnline})
}

// UpdateDeliveryProfilePhoto godoc
// PUT /api/v1/delivery/profile/photo (delivery partner only)
// Saves the partner's profile photo URL (uploaded via POST /delivery/upload).
// Kept separate from the onboarding selfie (KYC) so it never overwrites it.
func UpdateDeliveryProfilePhoto(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	var req struct {
		PhotoURL string `json:"photo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !validURL(req.PhotoURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A valid photo URL is required"})
		return
	}

	res := database.DB.Model(&models.DeliveryPartner{}).
		Where("id = ?", partnerID).
		Update("profile_photo_url", req.PhotoURL)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save profile photo"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"profile_photo_url": req.PhotoURL})
}
