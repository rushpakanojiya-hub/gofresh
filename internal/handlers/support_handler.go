package handlers

import (
"net/http"
"strconv"
"strings"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// CreateTicket godoc
// POST /api/v1/support/tickets (authenticated customer)
func CreateTicket(c *gin.Context) {
userID := c.MustGet("user_id").(uint)

var req models.CreateTicketRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

issueType := req.IssueType
if issueType == "" || !models.ValidIssueTypes[issueType] {
issueType = "other"
}

    // Order-linked tickets: order must belong to this customer and, except for
    // payment/refund issues, must still be inside the complaint window.
    if req.OrderID != nil {
        order, err := complaintLoadOwnedOrder(userID, *req.OrderID)
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
            return
        }
        if !complaintWindowExempt[issueType] {
            info, err := computeOrderComplaintWindow(order)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check complaint window"})
                return
            }
            if !info.Open {
                c.JSON(http.StatusForbidden, gin.H{
                    "error":    "The time to report a problem with this order has ended.",
                    "code":     "complaint_window_closed",
                    "deadline": info.Deadline,
                })
                return
            }
        }
    }

ticket := models.SupportTicket{
UserID:   userID,
OrderID:  req.OrderID,
Subject:  req.Subject,
Status:   "open",
Priority: "normal",
IssueType: issueType,
}

if err := database.DB.Create(&ticket).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
return
}

firstMsg := models.SupportMessage{
TicketID:   ticket.ID,
SenderID:   userID,
SenderType: "customer",
Message:    req.Message,
}
if err := database.DB.Create(&firstMsg).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save ticket message"})
return
}

c.JSON(http.StatusCreated, gin.H{"ticket": ticket, "message": firstMsg})
}

// GetMyTickets godoc
// GET /api/v1/support/tickets (authenticated customer) - own tickets only
func GetMyTickets(c *gin.Context) {
userID := c.MustGet("user_id").(uint)

var tickets []models.SupportTicket
if err := database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&tickets).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tickets"})
return
}
c.JSON(http.StatusOK, tickets)
}

// GetTicketMessages godoc
// GET /api/v1/support/tickets/:id/messages (authenticated customer, own ticket only)
func GetTicketMessages(c *gin.Context) {
userID := c.MustGet("user_id").(uint)
id, err := strconv.Atoi(c.Param("id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
return
}

var ticket models.SupportTicket
if err := database.DB.First(&ticket, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
return
}
if ticket.UserID != userID {
c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this ticket"})
return
}

var messages []models.SupportMessage
if err := database.DB.Where("ticket_id = ?", id).Order("created_at asc").Find(&messages).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
return
}
c.JSON(http.StatusOK, gin.H{"ticket": ticket, "messages": messages})
}

// ReplyToTicket godoc
// POST /api/v1/support/tickets/:id/messages (authenticated customer, own ticket only)
func ReplyToTicket(c *gin.Context) {
userID := c.MustGet("user_id").(uint)
id, err := strconv.Atoi(c.Param("id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
return
}

var ticket models.SupportTicket
if err := database.DB.First(&ticket, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
return
}
if ticket.UserID != userID {
c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this ticket"})
return
}

var req models.ReplyRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

msg := models.SupportMessage{
TicketID:   uint(id),
SenderID:   userID,
SenderType: "customer",
Message:    req.Message,
}
if err := database.DB.Create(&msg).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
return
}

// Reopen ticket if customer replies after it was resolved/closed.
if ticket.Status == "resolved" || ticket.Status == "closed" {
ticket.Status = "open"
database.DB.Save(&ticket)
}

c.JSON(http.StatusCreated, msg)
}

// GetAllTickets godoc
// GET /api/v1/admin/support/tickets (admin only) - optional ?status= filter
func GetAllTickets(c *gin.Context) {
status := c.Query("status")

query := database.DB.Order("created_at desc")
if status != "" {
query = query.Where("status = ?", status)
}

var tickets []models.SupportTicket
if err := query.Find(&tickets).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tickets"})
return
}
c.JSON(http.StatusOK, tickets)
}

// GetTicketMessagesAdmin godoc
// GET /api/v1/admin/support/tickets/:id/messages (admin only) - any ticket
func GetTicketMessagesAdmin(c *gin.Context) {
id, err := strconv.Atoi(c.Param("id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
return
}

var ticket models.SupportTicket
if err := database.DB.First(&ticket, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
return
}

var messages []models.SupportMessage
if err := database.DB.Where("ticket_id = ?", id).Order("created_at asc").Find(&messages).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
return
}
c.JSON(http.StatusOK, gin.H{"ticket": ticket, "messages": messages})
}

// AdminReplyToTicket godoc
// POST /api/v1/admin/support/tickets/:id/messages (admin only)
func AdminReplyToTicket(c *gin.Context) {
adminID := c.MustGet("user_id").(uint)
id, err := strconv.Atoi(c.Param("id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
return
}

var ticket models.SupportTicket
if err := database.DB.First(&ticket, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
return
}

var req models.ReplyRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

msg := models.SupportMessage{
TicketID:   uint(id),
SenderID:   adminID,
SenderType: "admin",
Message:    req.Message,
}
if err := database.DB.Create(&msg).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
return
}

// Auto-move ticket to in_progress on first admin reply.
if ticket.Status == "open" {
ticket.Status = "in_progress"
database.DB.Save(&ticket)
}

c.JSON(http.StatusCreated, msg)
}

// UpdateTicketStatus godoc
// PUT /api/v1/admin/support/tickets/:id/status (admin only)
// body: { "status": "resolved" }
func UpdateTicketStatus(c *gin.Context) {
id, err := strconv.Atoi(c.Param("id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
return
}

var body struct {
Status string `json:"status" binding:"required"`
}
if err := c.ShouldBindJSON(&body); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

valid := map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}
if !valid[body.Status] {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status, must be one of: open, in_progress, resolved, closed"})
return
}

var ticket models.SupportTicket
if err := database.DB.First(&ticket, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
return
}
ticket.Status = body.Status
if err := database.DB.Save(&ticket).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket"})
return
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "update_ticket_status", "support_ticket", strconv.Itoa(id), "status: "+body.Status)

c.JSON(http.StatusOK, ticket)
}


// UpdateTicket godoc
// PUT /api/v1/admin/support/tickets/:id (admin only)
func UpdateTicket(c *gin.Context) {
id, err := strconv.Atoi(c.Param("id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
return
}

var ticket models.SupportTicket
if err := database.DB.First(&ticket, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
return
}

var req models.UpdateTicketRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

changes := []string{}

if req.IssueType != nil {
if !models.ValidIssueTypes[*req.IssueType] {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid issue_type"})
return
}
ticket.IssueType = *req.IssueType
changes = append(changes, "issue_type: "+*req.IssueType)
}

if req.Priority != nil {
if !models.ValidPriorities[*req.Priority] {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid priority, must be one of: low, normal, high, urgent"})
return
}
ticket.Priority = *req.Priority
changes = append(changes, "priority: "+*req.Priority)
}

if req.AssignedToStaffID != nil && *req.AssignedToStaffID == 0 {
ticket.AssignedToStaffID = nil
changes = append(changes, "assigned_to_staff_id: unassigned")
} else if req.AssignedToStaffID != nil {
var staff models.User
if err := database.DB.First(&staff, *req.AssignedToStaffID).Error; err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "assigned_to_staff_id does not match a valid user"})
return
}
if staff.Role != "admin" {
c.JSON(http.StatusBadRequest, gin.H{"error": "assigned_to_staff_id must be an admin staff member"})
return
}
ticket.AssignedToStaffID = req.AssignedToStaffID
changes = append(changes, "assigned_to_staff_id: "+strconv.Itoa(int(*req.AssignedToStaffID)))
}

if err := database.DB.Save(&ticket).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket"})
return
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "update_ticket", "support_ticket", strconv.Itoa(id), strings.Join(changes, ", "))

database.DB.Preload("AssignedToStaff").First(&ticket, id)
c.JSON(http.StatusOK, ticket)
}
