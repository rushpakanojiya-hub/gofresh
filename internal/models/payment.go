package models

import "time"

// Payment lifecycle for a single Razorpay order attempt.
const (
PaymentStatusCreated           = "created"
PaymentStatusPaid              = "paid"
PaymentStatusFailed            = "failed"
PaymentStatusRefunded          = "refunded"
PaymentStatusPartiallyRefunded = "partially_refunded"
)

// Payment tracks the gateway side of an order's payment, and doubles as the
// reconciliation record once an admin marks a COD order refunded (a Payment
// row is created for it at that point via upsert).
// One row per Order (re-creating a payment order overwrites the same row,
// so a customer can retry after a failed attempt without piling up rows).
type Payment struct {
ID                uint      `gorm:"primaryKey" json:"id"`
OrderID           uint      `gorm:"uniqueIndex;not null" json:"order_id"`
Order             Order     `gorm:"foreignKey:OrderID" json:"-"`
RazorpayOrderID   string    `json:"razorpay_order_id"`
RazorpayPaymentID string    `json:"razorpay_payment_id,omitempty"`
RazorpaySignature string    `json:"-"`
Amount            float64   `json:"amount"`
Currency          string    `gorm:"default:INR" json:"currency"`
Status            string    `gorm:"default:created" json:"status"` // created/paid/failed/refunded/partially_refunded
Gateway           string    `gorm:"default:razorpay" json:"gateway"`
RefundedAmount    float64   `gorm:"default:0" json:"refunded_amount"`
GatewayFee        float64    `gorm:"default:0" json:"gateway_fee,omitempty"`
IsSettled         bool       `gorm:"default:false" json:"is_settled"`
SettledAt         *time.Time `json:"settled_at,omitempty"`
CreatedAt         time.Time `json:"created_at"`
UpdatedAt         time.Time `json:"updated_at"`
}

// CreatePaymentOrderResponse is returned by POST /orders/:id/payment so the
// frontend (web/Flutter Razorpay SDK) has everything needed to open Checkout.
type CreatePaymentOrderResponse struct {
RazorpayOrderID string `json:"razorpay_order_id"`
Amount          int64  `json:"amount"` // in paise, as Razorpay Checkout expects
Currency        string `json:"currency"`
KeyID           string `json:"key_id"`
OrderID         uint   `json:"order_id"`
}

// VerifyPaymentRequest is the body for POST /orders/:id/payment/verify.
type VerifyPaymentRequest struct {
RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
RazorpaySignature string `json:"razorpay_signature" binding:"required"`
}

// AdminPaymentRow is one row in the admin Payments reconciliation list.
// It's a synthesized view: rows with a real Payment record use its data,
// COD orders with no Payment record yet have their fields derived from
// the Order itself.
type AdminPaymentRow struct {
OrderID        uint      `json:"order_id" gorm:"column:order_id"`
TransactionID  string    `json:"transaction_id" gorm:"column:transaction_id"`
CustomerName   string    `json:"customer_name" gorm:"column:customer_name"`
CustomerPhone  string    `json:"customer_phone" gorm:"column:customer_phone"`
Amount         float64   `json:"amount" gorm:"column:amount"`
RefundedAmount float64   `json:"refunded_amount" gorm:"column:refunded_amount"`
PaymentMethod  string    `json:"payment_method" gorm:"column:payment_method"`
Gateway        string    `json:"gateway" gorm:"column:gateway"`
Status         string    `json:"status" gorm:"column:status"`
	CollectedVia  *string   `json:"collected_via,omitempty" gorm:"column:collected_via"`
	CreatedAt      time.Time `json:"created_at" gorm:"column:created_at"`
}

// AdminPaymentListResponse wraps paginated payment rows.
type AdminPaymentListResponse struct {
Payments   []AdminPaymentRow `json:"payments"`
Page       int               `json:"page"`
Limit      int               `json:"limit"`
Total      int64             `json:"total"`
TotalPages int               `json:"total_pages"`
}

// AdminPaymentStatusUpdateRequest is the body for
// PUT /admin/payments/:order_id/status (admin only).
type AdminPaymentStatusUpdateRequest struct {
Status         string   `json:"status" binding:"required,oneof=created paid failed refunded partially_refunded"`
RefundedAmount *float64 `json:"refunded_amount"`
}

// AdminPaymentReconciliationSummary is returned by
// GET /admin/payments/reconciliation.
type AdminPaymentReconciliationSummary struct {
TotalCollected  float64 `json:"total_collected" gorm:"column:total_collected"`
TotalPending    float64 `json:"total_pending" gorm:"column:total_pending"`
TotalRefunded   float64 `json:"total_refunded" gorm:"column:total_refunded"`
CountPaid       int64   `json:"count_paid" gorm:"column:count_paid"`
CountPending    int64   `json:"count_pending" gorm:"column:count_pending"`
CountFailed     int64   `json:"count_failed" gorm:"column:count_failed"`
CountRefunded   int64   `json:"count_refunded" gorm:"column:count_refunded"`
OnlineCollected float64 `json:"online_collected" gorm:"column:online_collected"`
CODCollected    float64 `json:"cod_collected" gorm:"column:cod_collected"`
}
