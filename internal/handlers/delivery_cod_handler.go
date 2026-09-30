package handlers

import (
"net/http"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// CODSummary is the response for GET /delivery/cod-summary. Every figure is
// computed live from Orders and RiderCODDeposit rows - nothing is cached
// or estimated, so it always reflects the current, real state.
type CODSummary struct {
// TotalCollected is the sum of TotalAmount across every delivered COD
// order ever assigned to this partner - the total cash they have ever
// physically received from customers.
TotalCollected float64 `json:"total_collected"`
// TotalDeposited is the sum of this partner's verified deposits only -
// a pending (not yet admin-verified) deposit does not reduce what the
// partner still owes, since it hasn't been confirmed received.
TotalDeposited float64 `json:"total_deposited"`
// PendingSettlement is what the partner is still holding in cash.
// Never negative - clamped to 0 if deposits somehow exceed collections
// (e.g. a correction), since a negative "amount owed" is meaningless
// here and would be confusing to show a rider.
PendingSettlement float64 `json:"pending_settlement"`
TodayCollected    float64 `json:"today_collected"`
TodayDeliveries   int64   `json:"today_cod_deliveries"`
}

// GetMyCODSummary godoc
// GET /api/v1/delivery/cod-summary (delivery partner only)
func GetMyCODSummary(c *gin.Context) {
partnerID := c.MustGet("user_id").(uint)

var summary CODSummary

database.DB.Model(&models.Order{}).
Where("delivery_partner_id = ? AND status = ? AND payment_method = ?", partnerID, models.OrderStatusDelivered, models.PaymentMethodCOD).
Select("COALESCE(SUM(total_amount), 0)").
Scan(&summary.TotalCollected)

database.DB.Model(&models.RiderCODDeposit{}).
Where("delivery_partner_id = ? AND status = ?", partnerID, "verified").
Select("COALESCE(SUM(amount), 0)").
Scan(&summary.TotalDeposited)

summary.PendingSettlement = summary.TotalCollected - summary.TotalDeposited
if summary.PendingSettlement < 0 {
summary.PendingSettlement = 0
}

todayStart := time.Now().Truncate(24 * time.Hour)
database.DB.Model(&models.Order{}).
Where("delivery_partner_id = ? AND status = ? AND payment_method = ? AND updated_at >= ?", partnerID, models.OrderStatusDelivered, models.PaymentMethodCOD, todayStart).
Select("COALESCE(SUM(total_amount), 0)").
Scan(&summary.TodayCollected)

database.DB.Model(&models.Order{}).
Where("delivery_partner_id = ? AND status = ? AND payment_method = ? AND updated_at >= ?", partnerID, models.OrderStatusDelivered, models.PaymentMethodCOD, todayStart).
Count(&summary.TodayDeliveries)

c.JSON(http.StatusOK, summary)
}

// GetMyCODSettlements godoc
// GET /api/v1/delivery/cod-settlements (delivery partner only)
// This partner's own deposit history - what they've paid in, and its
// verification status. Read-only; deposits are created by admin/finance,
// not by the rider themselves.
func GetMyCODSettlements(c *gin.Context) {
partnerID := c.MustGet("user_id").(uint)

var deposits []models.RiderCODDeposit
if err := database.DB.
Where("delivery_partner_id = ?", partnerID).
Order("deposit_date desc").
Find(&deposits).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settlement history"})
return
}

c.JSON(http.StatusOK, gin.H{"settlements": deposits})
}
