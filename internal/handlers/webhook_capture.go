package handlers

import (
	"log"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// processWebhookCapture applies a captured payment from the Razorpay webhook
// (BUG-31). It runs in one transaction with row locks on the payment and order,
// and if the order was cancelled/returned meanwhile it refunds the captured
// amount to the wallet instead of resurrecting the order.
func processWebhookCapture(rzpOrderID, rzpPaymentID string) {
	var orderID uint
	var cancelled bool
	var refundAmount float64
	var applied bool

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var payment models.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("razorpay_order_id = ?", rzpOrderID).First(&payment).Error; err != nil {
			return err
		}
		if payment.Status == models.PaymentStatusPaid || payment.Status == models.PaymentStatusRefunded {
			return nil
		}
		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, payment.OrderID).Error; err != nil {
			return err
		}
		orderID = order.ID
		payment.RazorpayPaymentID = rzpPaymentID
		payment.Status = models.PaymentStatusPaid

		if order.Status == models.OrderStatusCancelled || order.Status == models.OrderStatusReturned {
			cancelled = true
			refundAmount = payment.Amount
			payment.RefundedAmount = payment.Amount
			payment.Status = models.PaymentStatusRefunded
			if err := tx.Save(&payment).Error; err != nil {
				return err
			}
			refID := order.ID
			return utils.CreditWallet(tx, order.UserID, payment.Amount, models.WalletReasonOrderRefund, "order", &refID, "Refund for payment captured after order was cancelled")
		}

		if err := tx.Save(&payment).Error; err != nil {
			return err
		}
		if order.Status == models.OrderStatusPending {
			order.Status = models.OrderStatusConfirmed
		}
		order.PaymentStatus = models.OrderPaymentStatusPaid
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		log.Printf("[webhook] capture for razorpay order %s failed: %v", rzpOrderID, err)
		return
	}
	if cancelled {
		if err := services.PostWalletRefundLedgerEntry(orderID, refundAmount); err != nil {
			log.Printf("[webhook] refund ledger entry failed for order %d: %v", orderID, err)
		}
		return
	}
	if applied {
		_ = services.PostSalesLedgerEntry(orderID)
		_, _ = services.GenerateInvoiceIfNotExists(orderID)
		go services.AutoAssignDeliveryPartner(orderID)
	}
}
