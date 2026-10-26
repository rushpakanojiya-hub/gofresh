package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// gatewayFeeRate is a flat assumed rate (SRS 12.10) since no per-payment
// fee is returned by the gateway integration in this app. 2% matches
// Razorpay's typical published domestic card/UPI rate; this is a stated
// assumption, not a real fetched value.
const gatewayFeeRate = 0.02

// SettleGatewayPayment godoc
// POST /api/v1/admin/finance/payments/:id/settle-gateway
func SettleGatewayPayment(c *gin.Context) {
	id := c.Param("id")
	var payment models.Payment
	if err := database.DB.First(&payment, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}
	if payment.Gateway == "cod" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "COD payments have no gateway to settle"})
		return
	}
	if payment.Status != "paid" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only paid payments can be settled"})
		return
	}
	if payment.IsSettled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment is already settled"})
		return
	}

	// BUG-10: settle only what was not refunded.
	settleBase := payment.Amount - payment.RefundedAmount
	if settleBase < 0 {
		settleBase = 0
	}
	fee := settleBase * gatewayFeeRate
	payment.GatewayFee = fee
	payment.IsSettled = true
	now := time.Now()
	payment.SettledAt = &now
	if err := database.DB.Save(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to settle payment"})
		return
	}

	if err := services.PostGatewaySettlementLedgerEntry(payment.ID, fee); err != nil {
		database.DB.Model(&models.Payment{}).Where("id = ? AND is_settled = ?", payment.ID, true).Updates(map[string]interface{}{"is_settled": false, "settled_at": nil, "gateway_fee": 0}) // BUG-10: undo so settlement can be retried
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ledger posting failed, settlement was rolled back. Please retry: " + err.Error()})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "settle_gateway_payment", "payment", id, "settled")

	c.JSON(http.StatusOK, gin.H{
		"payment":        payment,
		"gross_amount":   payment.Amount,
		"gateway_fee":    fee,
		"net_settlement": settleBase - fee,
	})
}
