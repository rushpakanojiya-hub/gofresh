package services

import (
	"log"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// returnedOrderRestockRefundTx (BUG-44) runs inside the ResolveFailedDelivery
// "return" transaction: it restocks the order's warehouse (with StockMovement
// rows), refunds wallet + paid gateway amount to the customer's wallet,
// reinstates the coupon and marks payment refunded. Returns the total
// credited to the wallet so the caller can post the ledger entry after commit.
func returnedOrderRestockRefundTx(tx *gorm.DB, order *models.Order) (float64, error) {
	var items []models.OrderItem
	if err := tx.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
		return 0, err
	}
	for _, item := range items {
		var inventory models.Inventory
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ?", item.ProductID)
		if order.WarehouseID != nil {
			q = q.Where("warehouse_id = ?", *order.WarehouseID)
		}
		if err := q.Order("id").First(&inventory).Error; err != nil {
			continue
		}
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
			Reason:       "Failed delivery returned to store",
			ReferenceID:  &order.ID,
		}
		if err := tx.Create(&movement).Error; err != nil {
			return 0, err
		}
	}

	refID := order.ID
	var total float64
	if order.WalletAmountUsed > 0 {
		if err := utils.CreditWallet(tx, order.UserID, order.WalletAmountUsed, models.WalletReasonOrderRefund, "order", &refID, "Refund for failed delivery returned to store"); err != nil {
			return 0, err
		}
		total += order.WalletAmountUsed
	}
	if order.PaymentMethod == models.PaymentMethodOnline {
		var payment models.Payment
		if err := tx.Where("order_id = ?", order.ID).First(&payment).Error; err == nil && payment.Status == models.PaymentStatusPaid {
			gw := payment.Amount - payment.RefundedAmount
			if gw > 0 {
				if err := utils.CreditWallet(tx, order.UserID, gw, models.WalletReasonOrderRefund, "order", &refID, "Gateway refund for failed delivery returned to store"); err != nil {
					return 0, err
				}
				payment.RefundedAmount = payment.Amount
				payment.Status = models.PaymentStatusRefunded
				if err := tx.Save(&payment).Error; err != nil {
					return 0, err
				}
				total += gw
			}
		}
	}

	var orderCoupon models.OrderCoupon
	if err := tx.Where("order_id = ?", order.ID).First(&orderCoupon).Error; err == nil {
		if err := tx.Model(&models.Coupon{}).Where("id = ? AND used_count > 0", orderCoupon.CouponID).
			UpdateColumn("used_count", gorm.Expr("used_count - 1")).Error; err != nil {
			return 0, err
		}
	}
	if total > 0 {
		if err := tx.Model(&models.Order{}).Where("id = ?", order.ID).Update("payment_status", models.OrderPaymentStatusRefunded).Error; err != nil {
			return 0, err
		}
	}
	return total, nil
}

func postReturnedOrderRefundLedger(orderID uint, total float64) {
	if total <= 0 {
		return
	}
	if err := PostWalletRefundLedgerEntry(orderID, total); err != nil {
		log.Printf("failed to post refund ledger entry for returned order %d: %v", orderID, err)
	}
}
