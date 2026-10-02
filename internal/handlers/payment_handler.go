package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
        "gorm.io/gorm"
        "gorm.io/gorm/clause"
)

// CreatePaymentOrder godoc
// POST /api/v1/orders/:id/payment (protected)
// Creates a Razorpay order for an existing app order (payment_method must
// be "online") and returns what the frontend needs to open Razorpay
// Checkout. Safe to call again to retry after a failed/abandoned attempt ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â
// it overwrites the same Payment row rather than creating duplicates.
func CreatePaymentOrder(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	orderID := c.Param("id")

	var order models.Order
	if err := database.DB.First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if order.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this order"})
		return
	}
	if order.PaymentMethod != models.PaymentMethodOnline {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This order was placed as Cash on Delivery, not online payment"})
		return
	}
	if order.PaymentStatus == models.OrderPaymentStatusPaid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This order is already paid"})
		return
	}
	if order.Status == models.OrderStatusCancelled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot pay for a cancelled order"})
		return
	}

	rzpOrderID, err := utils.CreateRazorpayOrder(order.TotalAmount, "order_receipt_"+orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var payment models.Payment
	if err := database.DB.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
		payment = models.Payment{OrderID: order.ID}
	}
	payment.RazorpayOrderID = rzpOrderID
	payment.Amount = order.TotalAmount
	payment.Currency = "INR"
	payment.Status = models.PaymentStatusCreated

	if err := database.DB.Save(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record payment order"})
		return
	}

	c.JSON(http.StatusOK, models.CreatePaymentOrderResponse{
		RazorpayOrderID: rzpOrderID,
		Amount:          int64(order.TotalAmount*100 + 0.5),
		Currency:        "INR",
		KeyID:           config.AppConfig.RazorpayKeyID,
		OrderID:         order.ID,
	})
}

// VerifyPayment godoc
// POST /api/v1/orders/:id/payment/verify (protected)
// Verifies the Razorpay signature returned by Checkout on the frontend.
// On success, marks the Payment + Order as paid and auto-advances a
// still-pending order to "confirmed". On signature mismatch, the payment
// is marked failed and the order is left untouched ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â the frontend can
// retry via CreatePaymentOrder.
func VerifyPayment(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	orderID := c.Param("id")

	var order models.Order
	if err := database.DB.First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if order.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this order"})
		return
	}

	var req models.VerifyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var payment models.Payment
	if err := database.DB.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No payment order found for this order. Call POST /orders/:id/payment first."})
		return
	}
	if payment.RazorpayOrderID != req.RazorpayOrderID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Razorpay order ID does not match the one created for this order"})
		return
	}

	if !utils.VerifyRazorpaySignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
		payment.Status = models.PaymentStatusFailed
		database.DB.Save(&payment)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment signature verification failed"})
		return
	}

    var orderWasCancelled bool
    txErr := database.DB.Transaction(func(tx *gorm.DB) error {
        // Re-fetch and lock the order inside the transaction so a concurrent
        // cancellation can't race with this payment verification.
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, order.ID).Error; err != nil {
            return err
        }

        payment.RazorpayPaymentID = req.RazorpayPaymentID
        payment.RazorpaySignature = req.RazorpaySignature
        payment.Status = models.PaymentStatusPaid
        if err := tx.Save(&payment).Error; err != nil {
            return err
        }

        if order.Status == models.OrderStatusCancelled || order.Status == models.OrderStatusReturned {
            // The order was cancelled/returned while the customer was on the
            // gateway page - money was captured, but we must NOT overwrite the
            // order back to paid/confirmed (that would silently resurrect a
            // cancelled order without re-reserving warehouse stock). Instead,
            // leave the order's status alone and refund the captured amount to
            // the customer's wallet, same as the gateway-refund path in
            // CancelOrder, since there's no live Razorpay refund integration.
            orderWasCancelled = true
            payment.RefundedAmount = payment.Amount
            payment.Status = models.PaymentStatusRefunded
            if err := tx.Save(&payment).Error; err != nil {
                return err
            }
            refID := order.ID
            if err := utils.CreditWallet(tx, order.UserID, payment.Amount, models.WalletReasonOrderRefund, "order", &refID, "Refund for payment captured after order was cancelled"); err != nil {
                return err
            }
            return nil
        }

        order.PaymentStatus = models.OrderPaymentStatusPaid
        if order.Status == models.OrderStatusPending {
            order.Status = models.OrderStatusConfirmed
        }
        if err := tx.Save(&order).Error; err != nil {
            return err
        }
        return nil
    })

    if txErr != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Payment was verified but failed to update the order - contact support"})
        return
    }

    if orderWasCancelled {
        if err := services.PostWalletRefundLedgerEntry(order.ID, payment.Amount); err != nil {
            log.Printf("failed to post refund ledger entry for cancelled order %d: %v", order.ID, err)
        }
        c.JSON(http.StatusOK, gin.H{
            "message": "Payment was captured, but this order was already cancelled. The amount has been refunded to your wallet.",
            "order":   order,
        })
        return
    }

    // Payment confirmed: this is the moment an online order is really placed,
    // so the customer is notified now, not at checkout time.
    sendOrderPlacedNotification(order.ID)

    // Auto-assign the nearest available delivery partner now that
    // payment is confirmed, same as the COD checkout-time flow.
    if order.Status == models.OrderStatusConfirmed {
        go services.AutoAssignDeliveryPartner(order.ID)
    }
    if order.WarehouseID != nil {
        services.NotifyWarehouse(*order.WarehouseID, models.WhNotifyNewOrder,
            "New order #"+orderID,
            "Payment received and order confirmed - ready to accept.", &order.ID, nil)
    }
    // Online payment just confirmed - this is the one moment the invoice
    // can legally be generated (payment proof exists now), so do it here
    // rather than waiting for someone to view it.
    if _, err := services.GenerateInvoiceIfNotExists(order.ID); err != nil {
        log.Printf("failed to generate invoice for order %s: %v", orderID, err)
    }

    // Revenue is recognized now (online payment just verified) - post the
    // double-entry sales ledger entry at this exact moment, not earlier.
    if err := services.PostSalesLedgerEntry(order.ID); err != nil {
        log.Printf("failed to post sales ledger entry for order %s: %v", orderID, err)
    }

    var addr models.Address
    database.DB.First(&addr, order.AddressID)
    message := "Payment received for order #" + orderID + ". Your order is now confirmed."
    utils.SendNotification(addr.Phone, message, "payment_received", &order.ID)
    services.SendPushToUserWithData(order.UserID, "Payment Received", message, services.OrderPushData(order.ID))

    c.JSON(http.StatusOK, gin.H{
        "message": "Payment verified successfully",
        "order":   order,
    })
}

// RazorpayWebhook handles asynchronous payment notifications from Razorpay.
func RazorpayWebhook(c *gin.Context) {
    body, err := io.ReadAll(c.Request.Body)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
        return
    }

    secret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
    if secret != "" {
        sig := c.GetHeader("X-Razorpay-Signature")
        if !utils.VerifyRazorpayWebhookSignature(body, sig, secret) {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
            return
        }
    }

    var event struct {
        Event   string `json:"event"`
        Payload struct {
            Payment struct {
                Entity struct {
                    ID      string  `json:"id"`
                    OrderID string  `json:"order_id"`
                    Amount  float64 `json:"amount"`
                    Status  string  `json:"status"`
                } `json:"entity"`
            } `json:"payment"`
        } `json:"payload"`
    }

    if err := json.Unmarshal(body, &event); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format"})
        return
    }

    if event.Event == "payment.captured" || event.Event == "order.paid" {
        rzpOrderID := event.Payload.Payment.Entity.OrderID
        rzpPaymentID := event.Payload.Payment.Entity.ID
        if rzpOrderID != "" {
            var payment models.Payment
            if err := database.DB.Where("razorpay_order_id = ?", rzpOrderID).First(&payment).Error; err == nil {
                if payment.Status != models.PaymentStatusPaid {
                    payment.RazorpayPaymentID = rzpPaymentID
                    payment.Status = models.PaymentStatusPaid
                    database.DB.Save(&payment)

                    var order models.Order
                    if err := database.DB.First(&order, payment.OrderID).Error; err == nil {
                        if order.Status == models.OrderStatusPending {
                            order.Status = models.OrderStatusConfirmed
                        }
                        order.PaymentStatus = models.OrderPaymentStatusPaid
                        database.DB.Save(&order)

                        _ = services.PostSalesLedgerEntry(order.ID)
                        _, _ = services.GenerateInvoiceIfNotExists(order.ID)
                        go services.AutoAssignDeliveryPartner(order.ID)
                    }
                }
            }
        }
    }

    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}


