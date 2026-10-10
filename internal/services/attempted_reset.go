package services

import (
	"fmt"
	"strings"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// clearAttemptedPartner removes one partner from an unassigned order's
// attempted list, so a fresh check-in scan makes them eligible again.
func clearAttemptedPartner(orderID, partnerID uint) {
	var o models.Order
	if err := database.DB.Select("id", "delivery_attempted_partner_ids").First(&o, orderID).Error; err != nil {
		return
	}
	m := parseAttemptedIDs(o.DeliveryAttemptedPartnerIDs)
	if !m[partnerID] {
		return
	}
	delete(m, partnerID)
	parts := make([]string, 0, len(m))
	for id := range m {
		parts = append(parts, fmt.Sprint(id))
	}
	database.DB.Model(&models.Order{}).
		Where("id = ? AND delivery_partner_id IS NULL", orderID).
		Update("delivery_attempted_partner_ids", strings.Join(parts, ","))
}
