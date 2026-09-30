package models
import "time"
// SupportTicket represents a customer support request, optionally linked to
// an order. Status flows: open -> in_progress -> resolved -> closed.
type SupportTicket struct {
ID                 uint      `gorm:"primaryKey" json:"id"`
UserID             uint      `gorm:"not null" json:"user_id"`
OrderID            *uint     `json:"order_id,omitempty"`
Subject            string    `gorm:"not null" json:"subject"`
Status             string    `gorm:"default:open" json:"status"`
Priority           string    `gorm:"default:normal" json:"priority"`
IssueType          string    `gorm:"default:other" json:"issue_type"`
AssignedToStaffID  *uint     `json:"assigned_to_staff_id,omitempty"`
AssignedToStaff    *User     `gorm:"foreignKey:AssignedToStaffID" json:"assigned_to_staff,omitempty"`
CreatedAt          time.Time `json:"created_at"`
UpdatedAt          time.Time `json:"updated_at"`
}
// SupportMessage is a single message within a ticket's thread. SenderType
// distinguishes customer replies from admin/agent replies.
type SupportMessage struct {
ID         uint      `gorm:"primaryKey" json:"id"`
TicketID   uint      `gorm:"not null" json:"ticket_id"`
SenderID   uint      `gorm:"not null" json:"sender_id"`
SenderType string    `gorm:"not null" json:"sender_type"`
Message    string    `gorm:"not null" json:"message"`
CreatedAt  time.Time `json:"created_at"`
}
// CreateTicketRequest is the customer request body for POST /support/tickets.
type CreateTicketRequest struct {
OrderID   *uint  `json:"order_id"`
Subject   string `json:"subject" binding:"required"`
Message   string `json:"message" binding:"required"`
IssueType string `json:"issue_type"`
}
// ReplyRequest is the request body for POST /support/tickets/:id/messages
type ReplyRequest struct {
Message string `json:"message" binding:"required"`
}
// UpdateTicketRequest is the admin request body for PUT /admin/support/tickets/:id
type UpdateTicketRequest struct {
IssueType         *string `json:"issue_type"`
Priority          *string `json:"priority"`
AssignedToStaffID *uint   `json:"assigned_to_staff_id"`
}
// ValidIssueTypes is the allowed set for SupportTicket.IssueType.
var ValidIssueTypes = map[string]bool{
"missing_item":   true,
"wrong_item":     true,
"damaged_item":   true,
"expired_item":   true,
"delivery_issue": true,
"payment_issue":  true,
"refund_issue":   true,
"other":          true,
}
// ValidPriorities is the allowed set for SupportTicket.Priority.
var ValidPriorities = map[string]bool{
"low":    true,
"normal": true,
"high":   true,
"urgent": true,
}