package services

import (
	"log"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UnpaidWalletOrderTimeout is how long an online order that used wallet
// balance may stay unpaid before the wallet portion is refunded (BUG-15).
const UnpaidWalletOrderTimeout = 30 * time.Minute

// ExpireUnpaidWalletOrders cancels pending online orders whose gateway payment
// never completed, restocks the items, reinstates the coupon and refunds the
// wallet amount that was debited at checkout. A payment captured later is
// already handled by VerifyPayment (refunded to wallet for cancelled orders).
func ExpireUnpaidWalletOrders() {
	var ids []uint
	cutoff := time.Now().Add(-UnpaidWalletOrderTimeout)
	if err := database.DB.Model(&models.Order{}).
		Where("payment_method = ? AND status = ? AND payment_status = ? AND wallet_amount_used > 0 AND created_at < ?",
			models.PaymentMethodOnline, models.OrderStatusPending, models.OrderPaymentStatusPending, cutoff).
		Pluck("id", &ids).Error; err != nil {
		log.Printf("[unpaid-orders] scan failed: %v", err)
		return
	}

	for _, id := range ids {
		var refunded float64
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			var order models.Order
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, id).Error; err != nil {
				return err
			}
			if order.Status != models.OrderStatusPending || order.PaymentStatus != models.OrderPaymentStatusPending || order.WalletAmountUsed <= 0 {
				return nil
			}
			var items []models.OrderItem
			if err := tx.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
				return err
			}
			for _, item := range items {
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
						return err
					}
					movement := models.StockMovement{
						ProductID:    item.ProductID,
						WarehouseID:  inventory.WarehouseID,
						PreviousQty:  previousQty,
						Change:       item.Quantity,
						NewQty:       inventory.Stock,
						MovementType: models.MovementReturn,
						Reason:       "Order cancelled - online payment not completed",
						ReferenceID:  &order.ID,
					}
					if err := tx.Create(&movement).Error; err != nil {
						return err
					}
				}
			}
			refID := order.ID
			if err := utils.CreditWallet(tx, order.UserID, order.WalletAmountUsed, models.WalletReasonOrderRefund, "order", &refID, "Refund for unpaid online order"); err != nil {
				return err
			}
			var orderCoupon models.OrderCoupon
			if err := tx.Where("order_id = ?", order.ID).First(&orderCoupon).Error; err == nil {
				if err := tx.Model(&models.Coupon{}).Where("id = ? AND used_count > 0", orderCoupon.CouponID).
					UpdateColumn("used_count", gorm.Expr("used_count - 1")).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&models.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
				"status":         models.OrderStatusCancelled,
				"payment_status": models.OrderPaymentStatusRefunded,
			}).Error; err != nil {
				return err
			}
			refunded = order.WalletAmountUsed
			return nil
		})
		if err != nil {
			log.Printf("[unpaid-orders] failed to expire order %d: %v", id, err)
			continue
		}
		if refunded > 0 {
			if err := PostWalletRefundLedgerEntry(id, refunded); err != nil {
				log.Printf("[unpaid-orders] failed to post refund ledger entry for order %d: %v", id, err)
			}
			log.Printf("[unpaid-orders] order %d cancelled, Rs.%.2f refunded to wallet", id, refunded)
		}
	}
}
