package services

import (
	"errors"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var (
	ErrWarehouseHandoverGPSMissing      = errors.New("delivery partner's current location is not available - ask them to enable location and try again")
	ErrWarehouseHandoverOutsideGeofence = errors.New("delivery partner is too far from the warehouse to confirm this handover")
)

// VerifyWarehouseHandoverGeofence confirms the delivery partner's last-known
// GPS location is within config.WarehouseHandoverGeofenceRadiusMeters of the
// warehouse before a staff member is allowed to record a handover - this
// prevents any staff member who knows the delivery_partner_id from
// confirming a handover with nobody actually there.
func VerifyWarehouseHandoverGeofence(partnerID uint, warehouse models.Warehouse) error {
	var partner models.DeliveryPartner
	if err := database.DB.First(&partner, partnerID).Error; err != nil {
		return ErrWarehouseHandoverGPSMissing
	}
	if partner.CurrentLat == nil || partner.CurrentLng == nil || partner.LastLocationUpdate == nil {
		return ErrWarehouseHandoverGPSMissing
	}
	if time.Since(*partner.LastLocationUpdate) > staleLocationWindow {
		return ErrWarehouseHandoverGPSMissing
	}

	distanceKm := haversineKm(*partner.CurrentLat, *partner.CurrentLng, warehouse.Lat, warehouse.Lng)
	distanceMeters := distanceKm * 1000
	if distanceMeters > config.AppConfig.WarehouseHandoverGeofenceRadiusMeters {
		return ErrWarehouseHandoverOutsideGeofence
	}
	return nil
}
