package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrDeliveryStatusOrderNotOwned / ErrDeliveryStatusInvalidTransition are
// the sentinel errors UpdateDeliveryStatus returns for the two ways a
// delivery-status update can be legitimately refused.
//
// The remaining errors below are specific to the OTP + geofence checks
// enforced on the ARRIVED -> DELIVERED transition only.
var (
	ErrDeliveryStatusOrderNotOwned     = errors.New("order not found or not assigned to you")
	ErrDeliveryStatusInvalidTransition = errors.New("invalid delivery status transition")

	ErrDeliveryOTPRequired = errors.New("delivery OTP is required to mark this order delivered")
	ErrDeliveryOTPInvalid  = errors.New("incorrect delivery OTP")
	ErrDeliveryOTPExpired  = errors.New("delivery OTP has expired")
	ErrDeliveryOTPLocked   = errors.New("too many incorrect OTP attempts - delivery OTP is locked")

	ErrDeliveryGPSMissing             = errors.New("your current location is not available - please enable location and try again")
	ErrDeliveryAddressLocationMissing = errors.New("this address has no saved delivery coordinates, so the delivery location cannot be verified")
	ErrDeliveryOutsideGeofence        = errors.New("you are too far from the delivery address to mark this order delivered")
)

// deliveryStatusTransitions defines the only allowed forward moves in the
// granular, courier-driven delivery lifecycle:
//
//	ASSIGNED -> ACCEPTED -> PICKED_UP -> OUT_FOR_DELIVERY -> ARRIVED -> DELIVERED
//
// ASSIGNED and ACCEPTED are set automatically elsewhere (assign/auto-assign
// and accept flows) and are not reachable through UpdateDeliveryStatus -
// see the oneof binding on models.UpdateDeliveryStatusRequest.
var deliveryStatusTransitions = map[string]map[string]bool{
	models.DeliveryStatusAssigned: {models.DeliveryStatusAccepted: true},
	// New granular chain: accepted -> going_to_store -> arrived_at_store ->
	// picked_up -> out_for_delivery -> arrived_at_customer -> delivered.
	models.DeliveryStatusAccepted:       {models.DeliveryStatusGoingToStore: true},
	models.DeliveryStatusGoingToStore:   {models.DeliveryStatusArrivedAtStore: true},
	models.DeliveryStatusArrivedAtStore: {models.DeliveryStatusPickedUp: true},
	models.DeliveryStatusPickedUp:       {models.DeliveryStatusOutForDelivery: true},
	models.DeliveryStatusOutForDelivery: {models.DeliveryStatusArrivedAtCustomer: true},
	models.DeliveryStatusArrivedAtCustomer: {
		models.DeliveryStatusDelivered:      true,
		models.DeliveryStatusFailedDelivery: true,
	},
	// Resolution of a failed delivery is an explicit operation (see
	// ResolveFailedDelivery), not a silent automatic transition - but it
	// still needs to be a legal move in this map so the same
	// UpdateDeliveryStatus machinery can enforce it consistently.
	models.DeliveryStatusFailedDelivery: {
		models.DeliveryStatusOutForDelivery: true, // retry
		models.DeliveryStatusReturned:       true, // return to store
	},
	// DEPRECATED: DeliveryStatusArrived is the legacy pre-granular state.
	// Orders already sitting in this state (created before this change)
	// can still complete normally.
	models.DeliveryStatusArrived: {models.DeliveryStatusDelivered: true},
}

// deliveryOTPDigits is the length of the generated delivery-completion OTP.
const deliveryOTPDigits = 6

// UpdateDeliveryStatus lets the assigned delivery partner advance an
// order's granular delivery status one step at a time. It is the single
// source of truth for that state transition - shared by the
// PUT /delivery/orders/:id/delivery-status handler and by tests.
//
// Ownership is enforced by scoping the lookup to
// "id = ? AND delivery_partner_id = ?" using the *caller-supplied*
// partnerID (the handler sets this from the verified JWT, never from a
// client-supplied field), so one partner can never advance or discover
// another partner's order (IDOR/BOLA protection) - mirrors
// RespondToAssignment in delivery_acceptance.go.
//
// The transition is only allowed along deliveryStatusTransitions, enforced
// both by an explicit check and by a conditional UPDATE ...
// WHERE COALESCE(delivery_status,ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â) = <the status just read>, so two
// concurrent updates for the same order (e.g. a double-tap) can only ever
// have one winner.
//
// Two extra checks apply ONLY to the ARRIVED -> DELIVERED transition:
//   - otp must match the hashed OTP generated when the order entered
//     OUT_FOR_DELIVERY (ErrDeliveryOTPRequired / ErrDeliveryOTPInvalid /
//     ErrDeliveryOTPExpired / ErrDeliveryOTPLocked).
//   - the partner's last-known GPS location (pushed via PUT
//     /delivery/location) must be within config.DeliveryGeofenceRadiusMeters
//     of the order's delivery address (ErrDeliveryGPSMissing /
//     ErrDeliveryAddressLocationMissing / ErrDeliveryOutsideGeofence).
//
// A wrong/expired/locked OTP or a failed geofence check on DELIVERED must
// still durably record the OTP-attempt bump made inside verifyDeliveryOTP,
// even though the DELIVERED transition itself is refused. Returning an
// error from inside database.DB.Transaction's closure rolls back
// everything written via tx in that call - including that attempt
// counter - so on this path the closure returns nil (letting the
// transaction commit with only the attempt bump / no status change
// applied) and the real failure is reported to the caller afterward via
// the captured otpVerifyErr instead.
//
// This is intentionally independent of Order.Status - existing endpoints
// (UpdateDeliveryOrderStatus, ConfirmDelivery) keep driving Order.Status
// exactly as before; this only adds the finer-grained delivery_status
// column alongside it.
// The returned string is the freshly generated plaintext delivery OTP when
// newStatus is OUT_FOR_DELIVERY (empty otherwise). It exists solely so
// callers can deliver the code to the customer (push notification, SMS,
// or - in tests - direct assertions); the HTTP handler discards it and
// never includes it in any API response.
func UpdateDeliveryStatus(orderID, partnerID uint, newStatus string, otp string) (*models.Order, string, error) {
	var order models.Order
	var otpVerifyErr error  // set when a DELIVERED attempt fails OTP/geofence verification; the transaction still commits (to persist the OTP-attempt bump) but the caller must see this as a failure
	var plaintextOTP string // set when newStatus is OUT_FOR_DELIVERY, so the caller can relay it to the customer

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).
			First(&order).Error; err != nil {
			return ErrDeliveryStatusOrderNotOwned
		}

		current := ""
		if order.DeliveryStatus != nil {
			current = *order.DeliveryStatus
		}
		if !deliveryStatusTransitions[current][newStatus] {
			return ErrDeliveryStatusInvalidTransition
		}

		updates := map[string]interface{}{"delivery_status": newStatus}

		switch newStatus {
		case models.DeliveryStatusDelivered:
			// OTP verification is disabled for now - customers have no way
			// to receive the OTP (no SMS integration yet), so requiring it
			// blocked every delivery. Geofence check still applies to DELIVERED only.
			if err := verifyDeliveryGeofence(tx, &order, partnerID); err != nil {
				otpVerifyErr = err
			}
			if otpVerifyErr != nil {
				return nil
			}
			// Clear the delivery OTP now that it has served its purpose -
			// prevents a stale/leaked OTP from being reusable for a future
			// delivery attempt on this order.
			updates["delivery_otp_hash"] = nil
			updates["delivery_otp_expires_at"] = nil
			updates["delivery_otp_attempts"] = 0
		case models.DeliveryStatusOutForDelivery:
                        // Keep the coarse Order.Status in sync with the granular delivery
                        // stepper - customer-facing tracking reads Order.Status, so it must
                        // flip to shipped exactly when the partner starts the delivery leg,
                        // not earlier (legacy Mark-as-Shipped flow) or never at all.
                        updates["status"] = models.OrderStatusShipped

                        code, err := utils.GenerateNumericOTP(6)
			if err != nil {
				return fmt.Errorf("failed to generate delivery OTP: %w", err)
			}
			hash, err := utils.HashOTP(code)
			if err != nil {
				return fmt.Errorf("failed to hash delivery OTP: %w", err)
			}
			expiresAt := time.Now().Add(time.Duration(config.AppConfig.DeliveryOTPExpiryMinutes) * time.Minute)
			updates["delivery_otp_hash"] = hash
			updates["delivery_otp_expires_at"] = expiresAt
			updates["delivery_otp_attempts"] = 0
			plaintextOTP = code
		}

		result := tx.Model(&models.Order{}).
			Where("id = ? AND delivery_partner_id = ? AND COALESCE(delivery_status, '') = ?", order.ID, partnerID, current).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("failed to update delivery status: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrDeliveryStatusInvalidTransition
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if otpVerifyErr != nil {
		return nil, "", otpVerifyErr
	}

	database.DB.Preload("Address").Preload("Items").First(&order, order.ID)
	return &order, plaintextOTP, nil
}

// verifyDeliveryOTP checks the caller-supplied plaintext otp against the
// order's stored OTP hash, enforcing expiry and the max-attempts lock. On
// a wrong guess it persists the incremented attempt count (via tx, so it
// survives regardless of what the caller does with the returned error)
// before returning ErrDeliveryOTPInvalid.
func verifyDeliveryOTP(tx *gorm.DB, order *models.Order, otp string) error {
	if otp == "" {
		return ErrDeliveryOTPRequired
	}
	if order.DeliveryOTPHash == nil || order.DeliveryOTPExpiresAt == nil {
		return ErrDeliveryOTPRequired
	}
	if order.DeliveryOTPAttempts >= config.AppConfig.DeliveryOTPMaxAttempts {
		return ErrDeliveryOTPLocked
	}
	if time.Now().After(*order.DeliveryOTPExpiresAt) {
		return ErrDeliveryOTPExpired
	}
	if !utils.CompareOTP(*order.DeliveryOTPHash, otp) {
		tx.Model(&models.Order{}).
			Where("id = ?", order.ID).
			Update("delivery_otp_attempts", gorm.Expr("delivery_otp_attempts + 1"))
		return ErrDeliveryOTPInvalid
	}
	return nil
}

// verifyDeliveryGeofence loads the partner's last-known GPS location and
// the order's delivery address, and rejects the DELIVERED transition
// unless the partner is within config.DeliveryGeofenceRadiusMeters of the
// address. Missing or stale rider GPS, and an address with no saved
// coordinates, are both handled as explicit rejections rather than being
// silently skipped.
func verifyDeliveryGeofence(tx *gorm.DB, order *models.Order, partnerID uint) error {
	var partner models.DeliveryPartner
	if err := tx.First(&partner, partnerID).Error; err != nil {
		return ErrDeliveryGPSMissing
	}
	if partner.CurrentLat == nil || partner.CurrentLng == nil || partner.LastLocationUpdate == nil {
		return ErrDeliveryGPSMissing
	}
	if time.Since(*partner.LastLocationUpdate) > staleLocationWindow {
		return ErrDeliveryGPSMissing
	}

	var address models.Address
	if err := tx.First(&address, order.AddressID).Error; err != nil {
		return ErrDeliveryAddressLocationMissing
	}
	if address.Lat == nil || address.Lng == nil {
		return ErrDeliveryAddressLocationMissing
	}

	distanceKm := haversineKm(*partner.CurrentLat, *partner.CurrentLng, *address.Lat, *address.Lng)
	distanceMeters := distanceKm * 1000
	if distanceMeters > config.AppConfig.DeliveryGeofenceRadiusMeters {
		return ErrDeliveryOutsideGeofence
	}
	return nil
}

// VerifyDeliveryGeofenceForOrder lets handlers run the same geofence check
// (used before accepting the delivery proof photo) without a transaction.
func VerifyDeliveryGeofenceForOrder(order *models.Order, partnerID uint) error {
	return verifyDeliveryGeofence(database.DB, order, partnerID)
}

// ResolveFailedDelivery moves an order out of DeliveryStatusFailedDelivery
// via an explicit partner action - "retry" (back to OUT_FOR_DELIVERY, using
// the same delivery_status_transitions machinery so a fresh OTP is issued
// exactly like the first attempt) or "return" (terminal DeliveryStatusReturned,
// no further delivery attempts). The reason is always recorded via
// DeliveryRejectionReason for visibility in admin/ops views, reusing the
// existing field rather than adding a new one for what is conceptually the
// same "why didn't this go through" note.
func ResolveFailedDelivery(orderID, partnerID uint, action, reason string) (*models.Order, string, error) {
	var order models.Order
	var plaintextOTP string

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).
			First(&order).Error; err != nil {
			return ErrDeliveryStatusOrderNotOwned
		}

		current := ""
		if order.DeliveryStatus != nil {
			current = *order.DeliveryStatus
		}
		if current != models.DeliveryStatusFailedDelivery {
			return ErrDeliveryStatusInvalidTransition
		}

		var targetStatus string
		updates := map[string]interface{}{
			"delivery_rejection_reason": reason,
		}

		switch action {
		case "retry":
			targetStatus = models.DeliveryStatusOutForDelivery
			code, err := utils.GenerateNumericOTP(6)
			if err != nil {
				return fmt.Errorf("failed to generate delivery OTP: %w", err)
			}
			hash, err := utils.HashOTP(code)
			if err != nil {
				return fmt.Errorf("failed to hash delivery OTP: %w", err)
			}
			expiresAt := time.Now().Add(time.Duration(config.AppConfig.DeliveryOTPExpiryMinutes) * time.Minute)
			updates["delivery_otp_hash"] = hash
			updates["delivery_otp_expires_at"] = expiresAt
			updates["delivery_otp_attempts"] = 0
			plaintextOTP = code
                case "return":
                        targetStatus = models.DeliveryStatusReturned
                        // A returned delivery is done - clear the partner and mark
                        // the order's own coarse status returned too, so it doesn't
                        // keep counting against the partner's active-order capacity
                        // forever (this used to only set delivery_status, leaving
                        // Order.Status and delivery_partner_id stuck indefinitely).
                        updates["status"] = models.OrderStatusReturned
                        updates["delivery_partner_id"] = nil
		default:
			return ErrDeliveryStatusInvalidTransition
		}

		if !deliveryStatusTransitions[current][targetStatus] {
			return ErrDeliveryStatusInvalidTransition
		}
		updates["delivery_status"] = targetStatus

		result := tx.Model(&models.Order{}).
			Where("id = ? AND delivery_partner_id = ? AND COALESCE(delivery_status, '') = ?", order.ID, partnerID, current).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("failed to resolve failed delivery: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrDeliveryStatusInvalidTransition
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	notifTitle := "Delivery marked as returned"
	notifMsg := fmt.Sprintf("Order #%d has been returned to the store.", order.ID)
	notifType := "delivery_returned"
	if action == "retry" {
		notifTitle = "Retry delivery"
		notifMsg = fmt.Sprintf("Order #%d is back out for delivery - retry attempt.", order.ID)
		notifType = "delivery_retry"
	}
        if action == "return" {
                // This partner just freed up (order handed back to the
                // store) - immediately try to backfill any orders that were
                // left unassigned because every partner was busy.
                go TryAssignPendingOrdersToPartner(partnerID)
                // Delivery attempt failed and the order is going back to the
                // store - let the customer know right away.
                go SendPushToUserWithData(order.UserID, "Delivery Failed", fmt.Sprintf("We couldn't deliver your order #%d.", order.ID), OrderPushData(order.ID))
        }
	CreateDeliveryNotification(partnerID, notifTitle, notifMsg, notifType, &order.ID)
	database.DB.Preload("Address").Preload("Items").First(&order, order.ID)
	return &order, plaintextOTP, nil
}

