package handlers

import (
	"errors"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"gorm.io/gorm"
)

var (
	errAssignPartnerNotCheckedIn = errors.New("partner not checked in at the order's warehouse")
	errAssignPartnerCODLimit     = errors.New("partner would exceed COD cash limit")
)

// adminAssignableStatus (BUG-49): order statuses from which an admin may
// manually assign a delivery partner, including warehouse in-progress states.
var adminAssignableStatus = map[string]bool{
	models.OrderStatusConfirmed:        true,
	models.OrderStatusPicking:          true,
	models.OrderStatusPicked:           true,
	models.OrderStatusPacking:          true,
	models.OrderStatusPacked:           true,
	models.OrderStatusReadyForDispatch: true,
	models.OrderStatusHandedOver:       true,
	models.OrderStatusShipped:          true,
	"accepted":                         true,
}

// adminAssignExtraChecks applies the same store check-in and COD cash-limit
// rules that auto-assign uses (BUG-49).
func adminAssignExtraChecks(tx *gorm.DB, order *models.Order, partnerID uint) error {
	if order.WarehouseID != nil {
		var n int64
		if err := tx.Model(&models.DeliveryPartner{}).
			Where("id = ? AND checked_in_warehouse_id = ?", partnerID, *order.WarehouseID).
			Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return errAssignPartnerNotCheckedIn
		}
	}
	if order.PaymentMethod == models.PaymentMethodCOD {
		pending, err := services.PendingCODByPartner(tx)
		if err != nil {
			return err
		}
		inFlight, _ := services.InFlightCODByPartner(tx)
		held := pending[partnerID] + inFlight[partnerID]
		if held > 0 && held+order.TotalAmount > services.CODCashLimit {
			return errAssignPartnerCODLimit
		}
	}
	return nil
}
