package handlers

import (
	"log"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// adminCancelOrderTx cancels an order on behalf of an admin (BUG-29). It locks
// the order, restocks the warehouse the order was fulfilled from (with a
// StockMovement row), refunds wallet + paid gateway amount to the customer's
// wallet, reinstates the coupon and clears delivery assignment. It returns the
// gateway amount refunded so the caller can post the ledger entry.
func adminCancelOrderTx(tx *gorm.DB, order *models.Order) (float64, error) {
	var locked models.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, order.ID).Error; err != nil {
		return 0, err
	}
	if !adminCancellableStatus[locked.Status] {
		return 0, errOrderAlreadyCancelled
	}

	for _, item := range order.Items {
		var inventory models.Inventory
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ?", item.ProductID)
		if order.WarehouseID != nil {
			q = q.Where("warehouse_id = ?", *order.WarehouseID)
		}
		if err := q.Order("id").First(&inventory).Error; err == nil {
			previousQty := inventory.Stock
			inventory.Stock += item.Quantity
			inventory.InStock = true
			if err := tx.Save(&inventory).Error; err != nil {
				return 0, err
			}
			movement := models.StockMovement{
				ProductID:    item.ProductID,
				WarehouseID:  inventory.WarehouseID,
				PreviousQty:  previousQty,
				Change:       item.Quantity,
				NewQty:       inventory.Stock,
				MovementType: models.MovementReturn,
				Reason:       "Order cancelled by admin",
				ReferenceID:  &order.ID,
			}
			if err := tx.Create(&movement).Error; err != nil {
				return 0, err
			}
		}
	}

	refID := order.ID
	if order.WalletAmountUsed > 0 {
		if err := utils.CreditWallet(tx, order.UserID, order.WalletAmountUsed, models.WalletReasonOrderRefund, "order", &refID, "Refund for order cancelled by admin"); err != nil {
			return 0, err
		}
	}

	var orderCoupon models.OrderCoupon
	if err := tx.Where("order_id = ?", order.ID).First(&orderCoupon).Error; err == nil {
		if err := tx.Model(&models.Coupon{}).Where("id = ? AND used_count > 0", orderCoupon.CouponID).
			UpdateColumn("used_count", gorm.Expr("used_count - 1")).Error; err != nil {
			return 0, err
		}
	}

	var gatewayRefund float64
	if order.PaymentMethod == models.PaymentMethodOnline {
		var payment models.Payment
		if err := tx.Where("order_id = ?", order.ID).First(&payment).Error; err == nil && payment.Status == models.PaymentStatusPaid {
			gatewayRefund = payment.Amount - payment.RefundedAmount
			if gatewayRefund > 0 {
				if err := utils.CreditWallet(tx, order.UserID, gatewayRefund, models.WalletReasonOrderRefund, "order", &refID, "Gateway refund for order cancelled by admin"); err != nil {
					return 0, err
				}
				payment.RefundedAmount = payment.Amount
				payment.Status = models.PaymentStatusRefunded
				if err := tx.Save(&payment).Error; err != nil {
					return 0, err
				}
			}
		}
	}

	updates := map[string]interface{}{
		"status":                         models.OrderStatusCancelled,
		"delivery_partner_id":            nil,
		"delivery_assignment_status":     nil,
		"delivery_status":                nil,
		"delivery_assignment_expires_at": nil,
	}
	if gatewayRefund > 0 || order.WalletAmountUsed > 0 {
		updates["payment_status"] = models.OrderPaymentStatusRefunded
	}
	if err := tx.Model(&models.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
		return 0, err
	}
	return gatewayRefund, nil
}

func adminPostCancelRefund(orderID uint, total float64) {
	if total <= 0 {
		return
	}
	if err := services.PostWalletRefundLedgerEntry(orderID, total); err != nil {
		log.Printf("failed to post refund ledger entry for order %d: %v", orderID, err)
	}
}

// BUG-30: statuses from which an admin may cancel an order.
var adminCancellableStatus = map[string]bool{
	models.OrderStatusPending: true, models.OrderStatusConfirmed: true, models.OrderStatusPicking: true,
	models.OrderStatusPicked: true, models.OrderStatusPacking: true, models.OrderStatusPacked: true,
	models.OrderStatusReadyForDispatch: true,
}
