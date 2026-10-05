package models

import "time"

// Valid order status transitions: pending -> confirmed -> shipped -> delivered
// A pending or confirmed order can also move to cancelled.
const (
	OrderStatusPending          = "pending"
	OrderStatusConfirmed        = "confirmed"
	OrderStatusPicking          = "picking"
	OrderStatusPicked           = "picked"
	OrderStatusPacking          = "packing"
	OrderStatusPacked           = "packed"
	OrderStatusReadyForDispatch = "ready_for_dispatch"
	OrderStatusHandedOver       = "handed_over"
	OrderStatusShipped          = "shipped"
	OrderStatusDelivered        = "delivered"
	OrderStatusReturned         = "returned"
	OrderStatusCancelled        = "cancelled"
)

// Payment method chosen at checkout, and the resulting payment lifecycle.
const (
	PaymentMethodCOD    = "cod"
	PaymentMethodOnline = "online"

	OrderPaymentStatusPending = "pending"
	OrderPaymentStatusPaid    = "paid"
	OrderPaymentStatusFailed  = "failed"
    OrderPaymentStatusRefunded = "refunded"
    OrderPaymentStatusPartiallyRefunded = "partially_refunded"
)

// Delivery assignment lifecycle. This tracks the state of the *courier's*
// response to being handed an order, independent of the order's own
// fulfillment Status above. It only ever applies while DeliveryPartnerID
// is set:
//
//	ASSIGNED - a partner has just been offered the delivery (admin action,
//	           auto-assign, or automatic reassignment) and hasn't responded
//	           yet. Only the ASSIGNED state is open to Accept/Reject, and it
//	           carries an acceptance deadline in DeliveryAssignmentExpiresAt.
//	ACCEPTED - the assigned partner accepted the delivery. Terminal (happy
//	           path) - no further automatic reassignment.
//	REJECTED - the assigned partner declined. Automatically triggers an
//	           attempt to offer the order to the next eligible partner who
//	           hasn't already been tried (see services.TryAssignNextPartner).
//	EXPIRED  - the partner never responded within the acceptance window.
//	           Set by the periodic expiry sweep (services.ExpireStaleAssignments),
//	           which also triggers the same automatic reassignment as REJECTED.
const (
	DeliveryAssignmentStatusAssigned = "assigned"
	DeliveryAssignmentStatusAccepted = "accepted"
	DeliveryAssignmentStatusRejected = "rejected"
	DeliveryAssignmentStatusExpired  = "expired"
)

// Delivery status lifecycle. This is a finer-grained, courier-driven
// progression of an *accepted* delivery than DeliveryAssignmentStatus
// above (which only covers the assign/accept/reject/expire offer). It is
// additive and does not replace DeliveryAssignmentStatus or Order.Status:
//
//	ASSIGNED -> ACCEPTED are set automatically by the existing
//	            assign/auto-assign and accept flows (AssignDeliveryPartner,
//	            TryAssignNextPartner, RespondToAssignment), so they always
//	            stay in lockstep with DeliveryAssignmentStatus.
//	PICKED_UP, OUT_FOR_DELIVERY, ARRIVED, DELIVERED are set by the
//	            partner via PUT /delivery/orders/:id/delivery-status, one
//	            step at a time, once they've accepted the delivery.
const (
	DeliveryStatusAssigned       = "assigned"
	DeliveryStatusAccepted       = "accepted"
	DeliveryStatusPickedUp       = "picked_up"
	DeliveryStatusOutForDelivery = "out_for_delivery"
	DeliveryStatusArrived        = "arrived"
	DeliveryStatusDelivered      = "delivered"
// New granular pickup/delivery states (added for operations analytics -
// distance/time between accept, store arrival, pickup, and customer
// arrival). DeliveryStatusArrived above is kept for backward
// compatibility with orders created before this change; new code should
// use DeliveryStatusArrivedAtCustomer instead.
DeliveryStatusGoingToStore      = "going_to_store"
DeliveryStatusArrivedAtStore    = "arrived_at_store"
DeliveryStatusArrivedAtCustomer = "arrived_at_customer"
// DeliveryStatusFailedDelivery is a terminal-ish state reached from
// ArrivedAtCustomer when delivery could not be completed (customer
// refused COD, unavailable, etc.). Resolution is an explicit operation,
// not an automatic status change:
//   - "retry" moves the order back to DeliveryStatusOutForDelivery
//   - "return" moves the order to DeliveryStatusReturned
DeliveryStatusFailedDelivery = "failed_delivery"
// DeliveryStatusReturned is the final state for a failed delivery that
// was returned to the store rather than retried.
DeliveryStatusReturned = "returned"
)

type Order struct {
	ID                uint             `gorm:"primaryKey" json:"id"`
	UserID            uint             `gorm:"not null;index" json:"user_id"`
	User              User             `gorm:"foreignKey:UserID" json:"-"`
	AddressID         uint             `gorm:"not null" json:"address_id"`
	Address           Address          `gorm:"foreignKey:AddressID" json:"address,omitempty"`
	WarehouseID       *uint            `gorm:"index;index:idx_orders_warehouse_status,priority:1" json:"warehouse_id,omitempty"`
	Warehouse         *Warehouse       `gorm:"foreignKey:WarehouseID" json:"warehouse,omitempty"`
	ItemsAmount       float64          `gorm:"not null" json:"items_amount"`
	DeliveryCharge    float64          `gorm:"not null;default:0" json:"delivery_charge"`
	PlatformFee       float64          `gorm:"not null;default:0" json:"platform_fee"`
	WalletAmountUsed  float64          `gorm:"not null;default:0" json:"wallet_amount_used"`
// CouponDiscount is not a DB column - it's populated from the OrderCoupon
// table (if one exists for this order) by GetOrders/GetOrderByID so the
// customer app can display/account for the coupon discount without a
// separate API call.
CouponDiscount    float64          `gorm:"-" json:"coupon_discount"`
	TotalAmount       float64          `gorm:"not null" json:"total_amount"`
	Status            string           `gorm:"default:pending;index:idx_orders_warehouse_status,priority:2" json:"status"` // pending/confirmed/shipped/delivered/cancelled
	PaymentMethod     string           `gorm:"default:cod" json:"payment_method"`                                          // cod/online
	PaymentStatus     string           `gorm:"default:pending" json:"payment_status"`                                      // pending/paid/failed
	DeliveryPartnerID *uint            `gorm:"index" json:"delivery_partner_id,omitempty"`
	DeliveryPartner   *DeliveryPartner `gorm:"foreignKey:DeliveryPartnerID" json:"delivery_partner,omitempty"`
	// DeliveryAssignmentStatus is one of the DeliveryAssignmentStatus*
	// constants above, or empty/nil when no partner has ever been assigned.
	DeliveryAssignmentStatus *string `gorm:"index;size:20" json:"delivery_assignment_status,omitempty"`
	DeliveryRejectionReason  *string `json:"delivery_rejection_reason,omitempty"`
	// DeliveryAssignmentExpiresAt is when the current ASSIGNED offer expires
	// if the partner doesn't respond in time. Nil when there's no pending
	// offer (before first assignment, or after accept/reject/expiry).
	DeliveryAssignmentExpiresAt *time.Time `gorm:"index" json:"delivery_assignment_expires_at,omitempty"`
 // AssignedAt is when a delivery partner was FIRST successfully assigned
 // to this order (manual or auto-assign). Set once and never overwritten
 // by later re-assignments, so it reflects the order's original
 // confirmed-to-assigned duration for operations analytics. Nil until
 // assigned at least once.
 AssignedAt *time.Time `gorm:"index" json:"assigned_at,omitempty"`
 // DeliveredAt is when this order was marked DELIVERED (courier-confirmed,
 // OTP-verified). Nil until delivered. Used for delivery-time and SLA
 // analytics - never backfilled or estimated for orders delivered before
 // this field existed.
 DeliveredAt *time.Time `gorm:"index" json:"delivered_at,omitempty"`
	// DeliveryAttemptedPartnerIDs is a comma-separated list of every
	// partner ID already offered this order (assigned, then
	// rejected/expired), so automatic reassignment never offers the same
	// order to the same partner twice.
	DeliveryAttemptedPartnerIDs string `gorm:"default:''" json:"-"`
	// DeliveryStatus is the granular courier-driven delivery lifecycle
	// state (see the DeliveryStatus* constants above). Nil until a partner
	// is first assigned.
	DeliveryStatus *string `gorm:"index;size:20" json:"delivery_status,omitempty"`
	DeliveryProofURL *string `json:"delivery_proof_url,omitempty"`
        // UPI collection (COD orders paid by QR at the door). collected_via: cash|upi
        CollectedVia  *string    `gorm:"column:collected_via;size:10" json:"collected_via,omitempty"`
        UPIStatus     *string    `gorm:"column:upi_status;size:12" json:"upi_status,omitempty"` // unverified|verified|rejected
        UPIVerifiedAt *time.Time `gorm:"column:upi_verified_at" json:"upi_verified_at,omitempty"`
        UPIUTR        *string    `gorm:"column:upi_utr;size:40" json:"upi_utr,omitempty"`
	// Delivery-completion OTP fields. Only the bcrypt hash is ever
	// persisted - the plaintext code is never stored and is never
	// serialized to JSON (json:"-"), so it can never leak through any
	// delivery-partner or admin API response, including
	// toAssignedOrderSummary and every other order payload.
	// DeliveryOTPHash is set when the order moves to OUT_FOR_DELIVERY and
	// cleared once DELIVERED succeeds (or a fresh OTP is issued).
	DeliveryOTPHash *string `json:"-"`
	// DeliveryOTPExpiresAt is when the current OTP stops being valid -
	// config.DeliveryOTPExpiryMinutes after it was generated.
	DeliveryOTPExpiresAt *time.Time `json:"-"`
	// DeliveryOTPAttempts counts consecutive wrong-OTP guesses against the
	// current OTP. Reset to 0 whenever a fresh OTP is generated. Once it
	// reaches config.DeliveryOTPMaxAttempts, the OTP is locked and DELIVERED
	// is refused until a fresh OTP is issued (i.e. the order would need to
	// re-enter OUT_FOR_DELIVERY).
	DeliveryOTPAttempts int         `gorm:"not null;default:0" json:"-"`
	Items               []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"not null;index" json:"order_id"`
	ProductID uint      `gorm:"not null" json:"product_id"`
	Product   Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	Price     float64   `gorm:"not null" json:"price"` // price at time of order
	CreatedAt time.Time `json:"created_at"`
}

// CheckoutRequest is the body for POST /orders/checkout.
// AddressID is optional - if omitted, the user's default address is used.
// PaymentMethod is optional - defaults to "cod" if omitted; "online" starts
// the Razorpay flow (see POST /orders/:id/payment).
type CheckoutRequest struct {
	AddressID     uint   `json:"address_id"`
	PaymentMethod string `json:"payment_method" binding:"omitempty,oneof=cod online"`
	CouponCode    string `json:"coupon_code"`
	UseWallet     bool   `json:"use_wallet"`
}

// OrderStatusUpdateRequest is the body for PUT /admin/orders/:id/status (admin only).
type OrderStatusUpdateRequest struct {
	Status string `json:"status" binding:"required,oneof=confirmed shipped delivered cancelled"`
}

// RejectAssignmentRequest is the body for PUT /delivery/orders/:id/reject
// (delivery partner only). Reason is optional.
type RejectAssignmentRequest struct {
	Reason string `json:"reason"`
}

// UpdateDeliveryStatusRequest is the body for
// PUT /delivery/orders/:id/delivery-status (delivery partner only).
// Restricted to the states the partner drives themselves after accepting -
// ASSIGNED and ACCEPTED are set automatically by the existing
// assign/accept flow and can't be set through this endpoint.
// ResolveFailedDeliveryRequest is the body for
// PUT /delivery/orders/:id/resolve-failed (delivery partner only).
// Failure resolution is always an explicit choice, never automatic - the
// partner must say whether they are retrying delivery or returning the
// order to the store, and why the delivery failed in the first place.
type ResolveFailedDeliveryRequest struct {
// Action is "retry" (order goes back to DeliveryStatusOutForDelivery)
// or "return" (order goes to DeliveryStatusReturned).
Action string `json:"action" binding:"required,oneof=retry return"`
Reason string `json:"reason" binding:"required"`
}
type UpdateDeliveryStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=going_to_store arrived_at_store picked_up out_for_delivery arrived arrived_at_customer delivered failed_delivery"`
	// OTP is required only when Status is "delivered" - validated in
	// services.UpdateDeliveryStatus, not by a binding tag here, so the same
	// request struct still works for every other step.
	OTP string `json:"otp"`
}

// OrderListResponse wraps paginated order results.
type OrderListResponse struct {
	Orders     []Order `json:"orders"`
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	Total      int64   `json:"total"`
	TotalPages int     `json:"total_pages"`
}

