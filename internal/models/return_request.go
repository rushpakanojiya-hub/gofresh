package models

import "time"

// Return request status
const (
	ReturnStatusPending    = "pending"
	ReturnStatusApproved   = "approved"
	ReturnStatusRejected   = "rejected"
	PickupStatusAssigned   = "assigned"
	PickupStatusAccepted   = "accepted"
	PickupStatusEnRoute    = "en_route"
	PickupStatusArrived    = "arrived"
	PickupStatusPickedUp   = "picked_up"
	PickupStatusHandedOver = "handed_over"
	ReturnStatusCompleted  = "completed"
)

// ReturnRequest is a customer-initiated request to return one or more items
// from a delivered order. Approval restores stock for the returned items,
// refunds the calculated amount to the customer's wallet, and - only if
// every item on the order has now been returned across all approved
// requests - marks the order "returned".
type ReturnRequest struct {
	ID                        uint                `gorm:"primaryKey" json:"id"`
	OrderID                   uint                `gorm:"not null;index" json:"order_id"`
	Order                     Order               `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	UserID                    uint                `gorm:"not null;index" json:"user_id"`
	Reason                    string              `gorm:"not null" json:"reason"`
	Status                    string              `gorm:"default:pending" json:"status"`
	RefundAmount              float64             `gorm:"not null;default:0" json:"refund_amount"`
	ImageURL                  *string             `json:"image_url,omitempty"`
	DeliveryPartnerID         *uint               `json:"delivery_partner_id,omitempty"`
	DeliveryPartner           *DeliveryPartner    `gorm:"foreignKey:DeliveryPartnerID" json:"delivery_partner,omitempty"`
	PickupStatus              *string             `json:"pickup_status,omitempty"`
	PickupAssignmentExpiresAt *time.Time          `json:"pickup_assignment_expires_at,omitempty"`
	PickedUpAt                *time.Time          `json:"picked_up_at,omitempty"`
	HandedOverAt              *time.Time          `json:"handed_over_at,omitempty"`
	ConditionNotes            *string             `json:"condition_notes,omitempty"`
	ConditionPhotoURL         *string             `json:"condition_photo_url,omitempty"`
	Items                     []ReturnRequestItem `gorm:"foreignKey:ReturnRequestID" json:"items,omitempty"`
	ProcessedBy               *uint               `json:"processed_by,omitempty"`
	RejectionReason           *string             `json:"rejection_reason,omitempty"`
	CreatedAt                 time.Time           `json:"created_at"`
	UpdatedAt                 time.Time           `json:"updated_at"`
}

// ReturnRequestItem is one line within a return request: a specific order
// item and how many units of it are being returned (may be less than the
// quantity originally purchased).
type ReturnRequestItem struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ReturnRequestID uint      `gorm:"not null;index" json:"return_request_id"`
	OrderItemID     uint      `gorm:"not null;index" json:"order_item_id"`
	OrderItem       OrderItem `gorm:"foreignKey:OrderItemID" json:"order_item,omitempty"`
	Quantity        int       `gorm:"not null" json:"quantity"`
	RefundAmount    float64   `gorm:"not null;default:0" json:"refund_amount"` // item.Price * Quantity at request time
}

// ReturnRequestBody is the body for POST /orders/:id/return
type ReturnRequestBody struct {
	Reason   string                  `json:"reason" binding:"required"`
	ImageURL string                  `json:"image_url"`
	Items    []ReturnRequestItemBody `json:"items" binding:"required,min=1,dive"`
}

type ReturnRequestItemBody struct {
	OrderItemID uint `json:"order_item_id" binding:"required"`
	Quantity    int  `json:"quantity" binding:"required,min=1"`
}
