package services

import (
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
)

// CODCashLimit is the max COD cash a partner may hold (delivered COD minus
// verified deposits) before they stop receiving new COD orders. Keep in sync
// with _cashLimit in delivery_app home_screen.dart.
const CODCashLimit = 1500.0

// PendingCODByPartner returns pending settlement per partner, using the same
// rule as GetMyCODSummary: delivered COD total minus verified deposits.
// Partners with nothing pending are absent from the map (reads as 0).
func PendingCODByPartner(tx *gorm.DB) (map[uint]float64, error) {
	type row struct {
		PartnerID uint
		Amt       float64
	}
	var collected, deposited []row

	if err := tx.Model(&models.Order{}).
		Select("delivery_partner_id AS partner_id, COALESCE(SUM(total_amount), 0) AS amt").
		Where("delivery_partner_id IS NOT NULL AND status = ? AND payment_method = ? AND (collected_via IS NULL OR collected_via <> 'upi' OR upi_status = 'rejected')", models.OrderStatusDelivered, models.PaymentMethodCOD).
		Group("delivery_partner_id").
		Scan(&collected).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&models.RiderCODDeposit{}).
		Select("delivery_partner_id AS partner_id, COALESCE(SUM(amount), 0) AS amt").
		Where("status = ?", "verified").
		Group("delivery_partner_id").
		Scan(&deposited).Error; err != nil {
		return nil, err
	}

	out := make(map[uint]float64, len(collected))
	for _, r := range collected {
		out[r.PartnerID] = r.Amt
	}
	for _, r := range deposited {
		out[r.PartnerID] -= r.Amt
	}
	for id, v := range out {
		if v < 0 {
			out[id] = 0
		}
	}
	return out, nil
}
