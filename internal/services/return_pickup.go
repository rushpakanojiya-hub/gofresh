package services

import (
	"errors"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// returnPickupAcceptWindow is how long a delivery partner has to accept a
// return-pickup assignment before it's eligible for reassignment. Not wired
// to config yet - hardcoded so this doesn't depend on a config field name
// used elsewhere; wire it to config.AppConfig if you want it to match the
// forward-delivery assignment window exactly.
const returnPickupAcceptWindow = 5 * time.Minute

var (
	ErrReturnNotApprovedForPickup  = errors.New("return must be approved before a pickup can be assigned")
	ErrReturnPickupAlreadyAssigned = errors.New("this return already has an active pickup assignment")
	ErrReturnPickupNotYours        = errors.New("this pickup is not assigned to you")
	ErrReturnPickupWrongState      = errors.New("pickup is not in the expected state for this action")
	ErrReturnPickupPhotoRequired   = errors.New("a condition photo is required to confirm pickup")
)

// AssignReturnPickup assigns a delivery partner to pick up a return item
// from the customer. Callable from admin-panel or store-app (see
// return_pickup_handler.go) - both share this function, same pattern as
// ApproveReturn/RejectReturn being warehouse-scoped where applicable.
func AssignReturnPickup(returnReqID, partnerID uint) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var returnReq models.ReturnRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&returnReq, returnReqID).Error; err != nil {
			return err
		}
		if returnReq.Status != models.ReturnStatusApproved {
			return ErrReturnNotApprovedForPickup
		}
		// Allow (re)assignment only if there's no active assignment - a
		// rejected pickup clears PickupStatus back to nil (see
		// RejectReturnPickup) so it becomes assignable again.
		if returnReq.PickupStatus != nil && *returnReq.PickupStatus != "" {
			return ErrReturnPickupAlreadyAssigned
		}

		status := models.PickupStatusAssigned
		expiresAt := time.Now().Add(returnPickupAcceptWindow)
		returnReq.DeliveryPartnerID = &partnerID
		returnReq.PickupStatus = &status
		returnReq.PickupAssignmentExpiresAt = &expiresAt
		return tx.Save(&returnReq).Error
	})
}

// AcceptReturnPickup - delivery partner accepts an assigned return pickup.
func AcceptReturnPickup(returnReqID, partnerID uint) error {
	return transitionReturnPickup(returnReqID, partnerID, models.PickupStatusAssigned, models.PickupStatusAccepted, func(rr *models.ReturnRequest) error {
		if rr.PickupAssignmentExpiresAt != nil && time.Now().After(*rr.PickupAssignmentExpiresAt) {
			return ErrReturnPickupWrongState
		}
		return nil
	})
}

// RejectReturnPickup - delivery partner rejects an assigned return pickup.
// Clears the assignment so it's open for reassignment (manual, for now - a
// background sweep could call the same clearing logic on expiry if you want
// auto-reassignment like forward delivery already has).
func RejectReturnPickup(returnReqID, partnerID uint) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var returnReq models.ReturnRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&returnReq, returnReqID).Error; err != nil {
			return err
		}
		if returnReq.DeliveryPartnerID == nil || *returnReq.DeliveryPartnerID != partnerID {
			return ErrReturnPickupNotYours
		}
		if returnReq.PickupStatus == nil || *returnReq.PickupStatus != models.PickupStatusAssigned {
			return ErrReturnPickupWrongState
		}
		returnReq.DeliveryPartnerID = nil
		returnReq.PickupStatus = nil
		returnReq.PickupAssignmentExpiresAt = nil
		return tx.Save(&returnReq).Error
	})
}

// MarkReturnPickupEnRoute - partner has accepted and is heading to the customer.
func MarkReturnPickupEnRoute(returnReqID, partnerID uint) error {
	return transitionReturnPickup(returnReqID, partnerID, models.PickupStatusAccepted, models.PickupStatusEnRoute, nil)
}

// MarkReturnPickupArrived - partner has reached the customer's location.
func MarkReturnPickupArrived(returnReqID, partnerID uint) error {
	return transitionReturnPickup(returnReqID, partnerID, models.PickupStatusEnRoute, models.PickupStatusArrived, nil)
}

// ConfirmReturnPickedUp - partner has physically taken the product from the
// customer. Condition note + photo are mandatory (the partner's "as
// received" proof, separate from the customer's original return-request
// photo). Geofence checked against the order's delivery address, reusing
// verifyDeliveryGeofence (same package, same check as forward-delivery OTP).
func ConfirmReturnPickedUp(returnReqID, partnerID uint, conditionNotes, conditionPhotoURL string) error {
	if conditionPhotoURL == "" {
		return ErrReturnPickupPhotoRequired
	}
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var returnReq models.ReturnRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&returnReq, returnReqID).Error; err != nil {
			return err
		}
		if returnReq.DeliveryPartnerID == nil || *returnReq.DeliveryPartnerID != partnerID {
			return ErrReturnPickupNotYours
		}
		if returnReq.PickupStatus == nil || *returnReq.PickupStatus != models.PickupStatusArrived {
			return ErrReturnPickupWrongState
		}

		var order models.Order
		if err := tx.First(&order, returnReq.OrderID).Error; err != nil {
			return err
		}
		if err := verifyDeliveryGeofence(tx, &order, partnerID); err != nil {
			return err
		}

		now := time.Now()
		status := models.PickupStatusPickedUp
		returnReq.PickupStatus = &status
		returnReq.PickedUpAt = &now
		returnReq.ConditionNotes = &conditionNotes
		returnReq.ConditionPhotoURL = &conditionPhotoURL
		return tx.Save(&returnReq).Error
	})
}

// HandoverReturnToWarehouse - partner has physically handed the item to
// warehouse staff. Geofence checked against the warehouse, reusing
// VerifyWarehouseHandoverGeofence (same check as forward delivery's
// warehouse handover). This is the delivery partner's final step; the
// store-app's own confirm-received endpoint (ConfirmReturnReceived) is what
// actually triggers refund/stock restore.
func HandoverReturnToWarehouse(returnReqID, partnerID uint) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var returnReq models.ReturnRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&returnReq, returnReqID).Error; err != nil {
			return err
		}
		if returnReq.DeliveryPartnerID == nil || *returnReq.DeliveryPartnerID != partnerID {
			return ErrReturnPickupNotYours
		}
		if returnReq.PickupStatus == nil || *returnReq.PickupStatus != models.PickupStatusPickedUp {
			return ErrReturnPickupWrongState
		}

		var order models.Order
		if err := tx.First(&order, returnReq.OrderID).Error; err != nil {
			return err
		}
		if order.WarehouseID == nil {
			return errors.New("order has no warehouse assigned")
		}
		var warehouse models.Warehouse
		if err := tx.First(&warehouse, *order.WarehouseID).Error; err != nil {
			return err
		}
		if err := VerifyWarehouseHandoverGeofence(partnerID, warehouse); err != nil {
			return err
		}

		now := time.Now()
		status := models.PickupStatusHandedOver
		returnReq.PickupStatus = &status
		returnReq.HandedOverAt = &now
		return tx.Save(&returnReq).Error
	})
}

// transitionReturnPickup is the shared guard for the simple state-machine
// steps (accept/en-route/arrived) that just check ownership + current
// status and flip to the next one. extraCheck runs after the ownership and
// status check, before saving.
func transitionReturnPickup(returnReqID, partnerID uint, from, to string, extraCheck func(*models.ReturnRequest) error) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var returnReq models.ReturnRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&returnReq, returnReqID).Error; err != nil {
			return err
		}
		if returnReq.DeliveryPartnerID == nil || *returnReq.DeliveryPartnerID != partnerID {
			return ErrReturnPickupNotYours
		}
		if returnReq.PickupStatus == nil || *returnReq.PickupStatus != from {
			return ErrReturnPickupWrongState
		}
		if extraCheck != nil {
			if err := extraCheck(&returnReq); err != nil {
				return err
			}
		}
		status := to
		returnReq.PickupStatus = &status
		return tx.Save(&returnReq).Error
	})
}
