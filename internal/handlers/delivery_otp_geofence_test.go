package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// ---------------------------------------------------------------------------
// Delivery OTP + geofence - test helpers
// ---------------------------------------------------------------------------

// setPartnerLocation directly sets a delivery partner's last-known GPS
// location and refreshes last_location_update, bypassing the HTTP layer
// so tests can position a partner precisely (including far outside any
// address, or with a stale timestamp).
func setPartnerLocation(t *testing.T, partnerID uint, lat, lng float64) {
	t.Helper()
	if err := database.DB.Model(&models.DeliveryPartner{}).
		Where("id = ?", partnerID).
		Updates(map[string]interface{}{
			"current_lat":          lat,
			"current_lng":          lng,
			"last_location_update": time.Now(),
		}).Error; err != nil {
		t.Fatalf("failed to set partner location: %v", err)
	}
}

// deliveryStatusURL builds the delivery-status endpoint path for an order.
func deliveryStatusURL(orderID uint) string {
	return fmt.Sprintf("/api/v1/delivery/orders/%d/delivery-status", orderID)
}

// advanceToArrivedWithOTP drives an already-ACCEPTED order through
// PICKED_UP -> OUT_FOR_DELIVERY -> ARRIVED and returns the plaintext OTP
// generated along the way. OUT_FOR_DELIVERY is triggered via the service
// directly (not HTTP) purely so the test can capture the code - the real
// HTTP response never includes it (see TestDeliveryOTP_NeverExposedInAPIResponses).
func advanceToArrivedWithOTP(t *testing.T, r *gin.Engine, partner models.DeliveryPartner, order models.Order, token string) string {
	t.Helper()
	if w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "going_to_store"}); w.Code != http.StatusOK {
t.Fatalf("setup: going_to_store failed: %d %s", w.Code, w.Body.String())
}
if w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "arrived_at_store"}); w.Code != http.StatusOK {
t.Fatalf("setup: arrived_at_store failed: %d %s", w.Code, w.Body.String())
}
if w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "picked_up"}); w.Code != http.StatusOK {
t.Fatalf("setup: picked_up failed: %d %s", w.Code, w.Body.String())
	}
	_, otpCode, err := services.UpdateDeliveryStatus(order.ID, partner.ID, models.DeliveryStatusOutForDelivery, "")
	if err != nil || otpCode == "" {
		t.Fatalf("setup: out_for_delivery failed: %v (otp=%q)", err, otpCode)
	}
	if w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "arrived_at_customer"}); w.Code != http.StatusOK {
		t.Fatalf("setup: arrived failed: %d %s", w.Code, w.Body.String())
	}
	return otpCode
}

// ---------------------------------------------------------------------------
// 1. OTP success (also exercises the complete ASSIGNED->DELIVERED flow)
// ---------------------------------------------------------------------------

func TestDeliveryOTP_CorrectOTPInsideGeofenceSucceeds(t *testing.T) {
	r := newStatusGPSTestRouter()
	partner, order, token := acceptedOrderForStatusTest(t, r)
	otpCode := advanceToArrivedWithOTP(t, r, partner, order, token)

	// Exactly at the seeded address coordinates - well within any radius.
	setPartnerLocation(t, partner.ID, 23.0225, 72.5714)

	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": otpCode})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct OTP inside geofence, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryStatus == nil || *fresh.DeliveryStatus != models.DeliveryStatusDelivered {
		t.Errorf("expected delivery_status 'delivered', got %v", fresh.DeliveryStatus)
	}
	if fresh.DeliveryOTPHash != nil {
		t.Errorf("expected delivery_otp_hash to be cleared after successful delivery")
	}
}

// ---------------------------------------------------------------------------
// 2. Wrong OTP rejected
// ---------------------------------------------------------------------------

func TestDeliveryOTP_WrongOTPRejected(t *testing.T) {
	r := newStatusGPSTestRouter()
	partner, order, token := acceptedOrderForStatusTest(t, r)
	advanceToArrivedWithOTP(t, r, partner, order, token)
	setPartnerLocation(t, partner.ID, 23.0225, 72.5714)

	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": "000000"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong OTP, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryStatus == nil || (*fresh.DeliveryStatus != models.DeliveryStatusArrived && *fresh.DeliveryStatus != models.DeliveryStatusArrivedAtCustomer) {
		t.Errorf("delivery_status must remain 'arrived' after a wrong OTP, got %v", fresh.DeliveryStatus)
	}
	if fresh.DeliveryOTPAttempts != 1 {
		t.Errorf("expected delivery_otp_attempts to be 1 after one wrong guess, got %d", fresh.DeliveryOTPAttempts)
	}
}

// ---------------------------------------------------------------------------
// 3. Expired OTP rejected
// ---------------------------------------------------------------------------

func TestDeliveryOTP_ExpiredOTPRejected(t *testing.T) {
	r := newStatusGPSTestRouter()
	partner, order, token := acceptedOrderForStatusTest(t, r)
	otpCode := advanceToArrivedWithOTP(t, r, partner, order, token)
	setPartnerLocation(t, partner.ID, 23.0225, 72.5714)

	// Force the OTP to already be expired.
	past := time.Now().Add(-time.Minute)
	database.DB.Model(&models.Order{}).Where("id = ?", order.ID).Update("delivery_otp_expires_at", past)

	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": otpCode})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired OTP, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 4. Too many failed attempts locks the OTP
// ---------------------------------------------------------------------------

func TestDeliveryOTP_TooManyAttemptsLocksOTP(t *testing.T) {
	r := newStatusGPSTestRouter()
	partner, order, token := acceptedOrderForStatusTest(t, r)
	otpCode := advanceToArrivedWithOTP(t, r, partner, order, token)
	setPartnerLocation(t, partner.ID, 23.0225, 72.5714)

	maxAttempts := config.AppConfig.DeliveryOTPMaxAttempts
	for i := 0; i < maxAttempts; i++ {
		w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": "000000"})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400 for wrong OTP, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	// The OTP is now locked, even though the correct code is supplied.
	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": otpCode})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (locked) after exceeding max attempts, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryStatus == nil || (*fresh.DeliveryStatus != models.DeliveryStatusArrived && *fresh.DeliveryStatus != models.DeliveryStatusArrivedAtCustomer) {
		t.Errorf("delivery_status must remain 'arrived' once the OTP is locked, got %v", fresh.DeliveryStatus)
	}
}

// ---------------------------------------------------------------------------
// 5. OTP is never exposed through any delivery-partner-facing response
// ---------------------------------------------------------------------------

func TestDeliveryOTP_NeverExposedInAPIResponses(t *testing.T) {
	r := newStatusGPSTestRouter()
	_, order, token := acceptedOrderForStatusTest(t, r)

	
doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "going_to_store"})
	
doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "arrived_at_store"})
	
doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "picked_up"})
	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "out_for_delivery"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for out_for_delivery, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	forbidden := []string{"delivery_otp_hash", "delivery_otp_expires_at", "delivery_otp_attempts", `"otp"`}
	for _, key := range forbidden {
		if strings.Contains(body, key) {
			t.Errorf("response must never expose %q, got body: %s", key, body)
		}
	}

	// GetMyDeliveries serializes orders via toAssignedOrderSummary, a
	// hand-built struct with no OTP field at all - confirm the same
	// forbidden markers are absent there too.
	w2 := doRequest(r, http.MethodGet, "/api/v1/delivery/orders", token, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for GetMyDeliveries, got %d: %s", w2.Code, w2.Body.String())
	}
	body2 := w2.Body.String()
	for _, key := range forbidden {
		if strings.Contains(body2, key) {
			t.Errorf("GetMyDeliveries must never expose %q, got body: %s", key, body2)
		}
	}
}

// ---------------------------------------------------------------------------
// 6. Outside geofence rejected
// ---------------------------------------------------------------------------

func TestDeliveryGeofence_OutsideRadiusRejected(t *testing.T) {
	r := newStatusGPSTestRouter()
	partner, order, token := acceptedOrderForStatusTest(t, r)
	otpCode := advanceToArrivedWithOTP(t, r, partner, order, token)

	// Mumbai - hundreds of km from the seeded Ahmedabad address, well
	// outside any sane geofence radius.
	setPartnerLocation(t, partner.ID, 19.0760, 72.8777)

	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": otpCode})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-geofence delivery, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryStatus == nil || (*fresh.DeliveryStatus != models.DeliveryStatusArrived && *fresh.DeliveryStatus != models.DeliveryStatusArrivedAtCustomer) {
		t.Errorf("delivery_status must remain 'arrived' when outside the geofence, got %v", fresh.DeliveryStatus)
	}
}

// ---------------------------------------------------------------------------
// 7. Missing/stale rider GPS handled safely
// ---------------------------------------------------------------------------

func TestDeliveryGeofence_MissingGPSRejected(t *testing.T) {
	r := newStatusGPSTestRouter()
	partner, order, token := acceptedOrderForStatusTest(t, r)
	otpCode := advanceToArrivedWithOTP(t, r, partner, order, token)
	// No location ever pushed for this partner - CurrentLat/Lng stay nil.

	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": otpCode})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing rider GPS, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeliveryGeofence_StaleGPSRejected(t *testing.T) {
	r := newStatusGPSTestRouter()
	partner, order, token := acceptedOrderForStatusTest(t, r)
	otpCode := advanceToArrivedWithOTP(t, r, partner, order, token)

	// Location was pushed, but over an hour ago - stale beyond the
	// 30-minute trackable window used for geofencing.
	database.DB.Model(&models.DeliveryPartner{}).Where("id = ?", partner.ID).Updates(map[string]interface{}{
		"current_lat":          23.0225,
		"current_lng":          72.5714,
		"last_location_update": time.Now().Add(-time.Hour),
	})

	w := doRequest(r, http.MethodPut, deliveryStatusURL(order.ID), token, gin.H{"status": "delivered", "otp": otpCode})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for stale rider GPS, got %d: %s", w.Code, w.Body.String())
	}
}
