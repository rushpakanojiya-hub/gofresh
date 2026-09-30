package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// Complaint window rules for order-linked support tickets:
//
//   - The window starts at Order.DeliveredAt (falls back to UpdatedAt for old
//     orders that were delivered before that field existed).
//   - 48h if every item in the order is perishable, otherwise 72h.
//   - payment_issue and refund_issue have no window.
//   - Orders that are not delivered yet (delayed, cancelled, ...) have no window.
//   - Tickets that are NOT linked to an order (general chat) are never blocked,
//     so an agent can still decide case by case.
const (
	perishableComplaintWindow    = 48 * time.Hour
	nonPerishableComplaintWindow = 72 * time.Hour
)

// Issue types that are about money rather than the goods: no time limit.
var complaintWindowExempt = map[string]bool{
	"payment_issue": true,
	"refund_issue":  true,
}

// A category counts as perishable when its name contains any of these words
// (case-insensitive). Edit this list to match your category names.
var perishableCategoryKeywords = []string{
	"fruit", "vegetable", "dairy", "milk", "curd", "paneer",
	"egg", "bread", "bakery", "meat", "chicken", "fish", "seafood", "fresh",
}

func isPerishableCategoryName(name string) bool {
	n := strings.ToLower(name)
	if n == "" {
		return false
	}
	for _, kw := range perishableCategoryKeywords {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

type complaintWindowInfo struct {
	OrderID     uint       `json:"order_id"`
	Delivered   bool       `json:"delivered"`
	Open        bool       `json:"open"` // can an order-linked complaint be filed right now
	Perishable  bool       `json:"perishable"`
	WindowHours int        `json:"window_hours,omitempty"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	Deadline    *time.Time `json:"deadline,omitempty"`
}

// complaintLoadOwnedOrder returns the order only if it belongs to userID.
func complaintLoadOwnedOrder(userID uint, orderID uint) (*models.Order, error) {
	var order models.Order
	if err := database.DB.Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func computeOrderComplaintWindow(order *models.Order) (complaintWindowInfo, error) {
	info := complaintWindowInfo{OrderID: order.ID, Open: true}
	if order.Status != "delivered" {
		return info, nil
	}

	deliveredAt := order.UpdatedAt
	if order.DeliveredAt != nil {
		deliveredAt = *order.DeliveredAt
	}

	var items []models.OrderItem
	if err := database.DB.Preload("Product.Category").Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
		return info, err
	}
	allPerishable := len(items) > 0
	for _, it := range items {
		if !isPerishableCategoryName(it.Product.Category.Name) {
			allPerishable = false
			break
		}
	}

	window := nonPerishableComplaintWindow
	if allPerishable {
		window = perishableComplaintWindow
	}
	deadline := deliveredAt.Add(window)

	info.Delivered = true
	info.Perishable = allPerishable
	info.WindowHours = int(window / time.Hour)
	info.DeliveredAt = &deliveredAt
	info.Deadline = &deadline
	info.Open = time.Now().Before(deadline)
	return info, nil
}

// GetOrderComplaintWindow godoc
// GET /api/v1/support/orders/:id/window (authenticated customer, own order only)
func GetOrderComplaintWindow(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	order, err := complaintLoadOwnedOrder(userID, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	info, err := computeOrderComplaintWindow(order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check complaint window"})
		return
	}
	c.JSON(http.StatusOK, info)
}
