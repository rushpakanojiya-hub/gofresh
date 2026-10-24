package services

import (
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
)

// InFlightCODByPartner (BUG-48) returns the COD value of orders currently
// assigned to each partner that are not yet delivered/cancelled/returned, so
// cash they are about to collect counts toward CODCashLimit. On error it
// returns an empty map (limit then falls back to delivered-only behaviour).
func InFlightCODByPartner(tx *gorm.DB) (map[uint]float64, error) {
	type row struct {
		PartnerID uint
		Amt       float64
	}
	var rows []row
	out := map[uint]float64{}
	if err := tx.Model(&models.Order{}).
		Select("delivery_partner_id AS partner_id, COALESCE(SUM(total_amount), 0) AS amt").
		Where("delivery_partner_id IS NOT NULL AND payment_method = ? AND status NOT IN ?", models.PaymentMethodCOD,
			[]string{models.OrderStatusDelivered, models.OrderStatusCancelled, models.OrderStatusReturned}).
		Group("delivery_partner_id").
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, r := range rows {
		out[r.PartnerID] = r.Amt
	}
	return out, nil
}
