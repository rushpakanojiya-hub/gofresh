package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------------------------------------------------------------------------
// Delivery Partners (admin only)
// ---------------------------------------------------------------------------

// CreateDeliveryPartner godoc
// POST /api/v1/admin/delivery-partners (admin only)
func CreateDeliveryPartner(c *gin.Context) {
	var req models.DeliveryPartnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	maxActiveOrders := 1 // safety net if config wasn't loaded (e.g. some tests)
	if config.AppConfig != nil && config.AppConfig.DefaultMaxActiveOrdersPerPartner > 0 {
		maxActiveOrders = config.AppConfig.DefaultMaxActiveOrdersPerPartner
	}
	if req.MaxActiveOrders != nil {
		maxActiveOrders = *req.MaxActiveOrders
	}

	partner := models.DeliveryPartner{
		Name:            req.Name,
		Phone:           req.Phone,
		VehicleNumber:   req.VehicleNumber,
		IsActive:        true,
		MaxActiveOrders: maxActiveOrders,
	}
	if req.IsActive != nil {
		partner.IsActive = *req.IsActive
	}

	if err := database.DB.Create(&partner).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Delivery partner already exists or could not be created"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"delivery_partner": partner})
}

// GetDeliveryPartners godoc
// GET /api/v1/admin/delivery-partners (admin only)
func GetDeliveryPartners(c *gin.Context) {
	var partners []models.DeliveryPartner
	if err := database.DB.Order("created_at DESC").Find(&partners).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load delivery partners"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"delivery_partners": buildPartnersWithRatings(partners)})
}

// UpdateDeliveryPartner godoc
// PUT /api/v1/admin/delivery-partners/:id (admin only)
func UpdateDeliveryPartner(c *gin.Context) {
	id := c.Param("id")

	var partner models.DeliveryPartner
	if err := database.DB.First(&partner, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}

	var req models.DeliveryPartnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	partner.Name = req.Name
	partner.Phone = req.Phone
	partner.VehicleNumber = req.VehicleNumber
	if req.IsActive != nil {
		partner.IsActive = *req.IsActive
	}
	if req.MaxActiveOrders != nil {
		partner.MaxActiveOrders = *req.MaxActiveOrders
	}

	if err := database.DB.Save(&partner).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update delivery partner"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"delivery_partner": partner})
}

// DeleteDeliveryPartner godoc
// DELETE /api/v1/admin/delivery-partners/:id (admin only)
func DeleteDeliveryPartner(c *gin.Context) {
	id := c.Param("id")

	// Refuse to delete a partner who still has an active, in-flight order
	// assigned to them - otherwise the order is silently stranded with a
	// delivery_partner_id that no longer resolves to anyone, and neither
	// the customer nor any other partner is ever notified to pick it up.
	var activeCount int64
	if err := database.DB.Model(&models.Order{}).
		Where("delivery_partner_id = ? AND status NOT IN (?)", id, []string{
			models.OrderStatusDelivered,
			models.OrderStatusCancelled,
			models.OrderStatusReturned,
		}).
		Count(&activeCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check active deliveries"})
		return
	}
	if activeCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "This delivery partner has active in-flight orders and cannot be deleted until they are reassigned or completed"})
		return
	}
	result := database.DB.Delete(&models.DeliveryPartner{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete delivery partner"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Delivery partner deleted"})
}

// ---------------------------------------------------------------------------
// Order assignment (admin only)
// ---------------------------------------------------------------------------

// errAssignConflict/errAssignValidation are internal sentinels used to
// short-circuit the transaction below with a specific, already-decided
// HTTP response, distinct from an unexpected DB error.
var (
	errAssignOrderNotFound     = errors.New("order not found")
	errAssignBadOrderStatus    = errors.New("order not eligible for assignment")
	errAssignPartnerNotFound   = errors.New("delivery partner not found")
	errAssignPartnerInactive   = errors.New("delivery partner is not active")
	errAssignPartnerOffline    = errors.New("delivery partner is offline")
	errAssignAlreadyActive     = errors.New("order already has an active delivery assignment")
	errAssignPartnerAtCapacity = errors.New("delivery partner is at maximum active order capacity")
)

// AssignDeliveryPartner godoc
// PUT /api/v1/admin/orders/:id/assign-delivery (admin only)
//
// Assigns (or re-assigns, after a rejection) a delivery partner to an
// order. Only ONLINE, active partners are eligible - offline partners
// must never receive new assignments. The whole check-then-write runs
// inside one transaction with the order row locked for its duration, so
// two concurrent assign requests for the same order can never both
// succeed (no double assignment).
//
// This also starts the assignment's acceptance-timeout clock
// (delivery_assignment_expires_at) and resets the attempted-partners
// history to just this partner, since a manual admin assignment starts a
// fresh offer cycle.
func AssignDeliveryPartner(c *gin.Context) {
	orderID := c.Param("id")

	var req models.AssignDeliveryPartnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var order models.Order
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the order row for the duration of this transaction so a
		// concurrent assignment (admin or auto-assign) for the same
		// order has to wait, rather than racing on the read-then-write
		// below.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Address").First(&order, orderID).Error; err != nil {
			return errAssignOrderNotFound
		}

		if order.Status != models.OrderStatusConfirmed && order.Status != models.OrderStatusReadyForDispatch && order.Status != models.OrderStatusShipped {
			return errAssignBadOrderStatus
		}

		// Only re-assignable if there is no partner yet, or the
		// previous partner rejected the delivery or their offer expired
		// unaccepted. An ASSIGNED or ACCEPTED order is already actively
		// owned by a partner and must not be silently reassigned here.
		if order.DeliveryPartnerID != nil &&
			(order.DeliveryAssignmentStatus == nil ||
				(*order.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusRejected &&
					*order.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusExpired)) {
			return errAssignAlreadyActive
		}

		var partner models.DeliveryPartner
		if err := tx.First(&partner, req.DeliveryPartnerID).Error; err != nil {
			return errAssignPartnerNotFound
		}
		if !partner.IsActive {
			return errAssignPartnerInactive
		}
		if !partner.IsOnline {
			return errAssignPartnerOffline
		}

		// Capacity check: a partner already handling as many active
		// (not yet delivered/cancelled/returned) orders as their configured
		// MaxActiveOrders must not receive another manual assignment - mirrors
		// the guard AutoAssignDeliveryPartner already applies.
		var activeCount int64
		if err := tx.Model(&models.Order{}).
			Where("delivery_partner_id = ? AND status NOT IN (?)", partner.ID, []string{
				models.OrderStatusDelivered,
				models.OrderStatusCancelled,
				models.OrderStatusReturned,
			}).
			Count(&activeCount).Error; err != nil {
			return fmt.Errorf("failed to check partner workload: %w", err)
		}
		maxActive := partner.MaxActiveOrders
		if maxActive <= 0 {
			maxActive = 5
		}
		if activeCount >= int64(maxActive) {
			return errAssignPartnerAtCapacity
		}

		// AssignedAt is set once, on the very first successful
		// assignment - later re-assignments (e.g. after a rejection)
		// must not overwrite the original confirmed-to-assigned
		// duration used for operations analytics.
		assignedAtValue := order.AssignedAt
		if assignedAtValue == nil {
			now := time.Now()
			assignedAtValue = &now
		}
		newStatus := models.DeliveryAssignmentStatusAssigned
		expiresAt := time.Now().Add(services.AssignmentTimeout())
		order.DeliveryPartnerID = &req.DeliveryPartnerID
		order.DeliveryAssignmentStatus = &newStatus
		order.DeliveryRejectionReason = nil

		if err := tx.Model(&models.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
			"delivery_partner_id":            req.DeliveryPartnerID,
			"delivery_assignment_status":     newStatus,
			"delivery_rejection_reason":      nil,
			"delivery_assignment_expires_at": expiresAt,
			"delivery_attempted_partner_ids": fmt.Sprint(req.DeliveryPartnerID),
			"delivery_status":                models.DeliveryStatusAssigned,
			"assigned_at":                    assignedAtValue,
		}).Error; err != nil {
			return fmt.Errorf("failed to assign delivery partner: %w", err)
		}
		return nil
	})

	if err != nil {
		switch {
		case errors.Is(err, errAssignOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		case errors.Is(err, errAssignBadOrderStatus):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Delivery partner can only be assigned to confirmed or shipped orders"})
		case errors.Is(err, errAssignAlreadyActive):
			c.JSON(http.StatusConflict, gin.H{"error": "Order already has an active delivery assignment"})
		case errors.Is(err, errAssignPartnerNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		case errors.Is(err, errAssignPartnerInactive):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Delivery partner is not active"})
		case errors.Is(err, errAssignPartnerOffline):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Delivery partner is offline and cannot receive new assignments"})
		case errors.Is(err, errAssignPartnerAtCapacity):
			c.JSON(http.StatusConflict, gin.H{"error": "Delivery partner already has the maximum number of active orders and cannot receive another assignment"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign delivery partner"})
		}
		return
	}

	database.DB.Preload("DeliveryPartner").First(&order, order.ID)

	// Notify the partner's device(s) that a new order has been assigned.
	go services.SendPushToPartnerWithData(req.DeliveryPartnerID, "New Order Assigned", "You have a new delivery order. Tap to view and accept it.", map[string]string{"type": "new_assignment", "order_id": fmt.Sprint(order.ID)})
	services.CreateDeliveryNotification(
		req.DeliveryPartnerID,
		"New delivery assigned",
		fmt.Sprintf("Order #%d has been assigned to you", order.ID),
		"new_assignment",
		&order.ID,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Delivery partner assigned", "order": order})
}

// ---------------------------------------------------------------------------
// Delivery Partner order actions (delivery partner only)
// ---------------------------------------------------------------------------

// AssignedOrderSummary is the shape returned by GET /delivery/orders. It
// deliberately surfaces only what a courier needs to fulfil the delivery -
// it does not include the customer's account/user record, other saved
// addresses, product cost/margin fields, etc.
type OrderItemSummary struct {
	ImageURL    string  `json:"image_url"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
}

type AssignedOrderSummary struct {
	StoreStaffName      string             `json:"store_staff_name"`
	StoreStaffPhone     string             `json:"store_staff_phone"`
	OrderID             uint               `json:"order_id"`
	Status              string             `json:"status"`
	AssignmentStatus    *string            `json:"assignment_status,omitempty"`
	RejectionReason     *string            `json:"rejection_reason,omitempty"`
	AssignmentExpiresAt *time.Time         `json:"assignment_expires_at,omitempty"`
	DeliveryStatus      *string            `json:"delivery_status,omitempty"`
	DeliveryAddress     string             `json:"delivery_address"`
	CustomerName        string             `json:"customer_name"`
	PickupName          string             `json:"pickup_name"`
	PickupAddress       string             `json:"pickup_address"`
	CustomerPhone       string             `json:"customer_phone"`
	TotalAmount         float64            `json:"total_amount"`
	PaymentMethod       string             `json:"payment_method"`
	ItemCount           int                `json:"item_count"`
	CollectedVia        *string            `json:"collected_via,omitempty"`
	Items               []OrderItemSummary `json:"items"`
	DeliveryLat         *float64           `json:"delivery_lat,omitempty"`
	DeliveryLng         *float64           `json:"delivery_lng,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
}

func toAssignedOrderSummary(o models.Order) AssignedOrderSummary {
	addr := fmt.Sprintf("%s, %s, %s - %s", o.Address.Line1, o.Address.City, o.Address.State, o.Address.Pincode)
	if o.Address.Line2 != "" {
		addr = fmt.Sprintf("%s, %s, %s, %s - %s", o.Address.Line1, o.Address.Line2, o.Address.City, o.Address.State, o.Address.Pincode)
	}
	itemSummaries := make([]OrderItemSummary, 0, len(o.Items))
	for _, it := range o.Items {
		itemSummaries = append(itemSummaries, OrderItemSummary{
			ProductName: it.Product.Name,
			ImageURL:    it.Product.ImageURL,
			Quantity:    it.Quantity,
			Price:       it.Price,
		})
	}
	pickupName, pickupAddr := "", ""
	if o.Warehouse != nil {
		pickupName = o.Warehouse.Name
		pickupAddr = o.Warehouse.Address
		if pickupAddr == "" {
			pickupAddr = o.Warehouse.City
		}
	}
	return AssignedOrderSummary{
		OrderID:             o.ID,
		Status:              o.Status,
		AssignmentStatus:    o.DeliveryAssignmentStatus,
		RejectionReason:     o.DeliveryRejectionReason,
		AssignmentExpiresAt: o.DeliveryAssignmentExpiresAt,
		DeliveryStatus:      o.DeliveryStatus,
		DeliveryAddress:     addr,
		CustomerName:        o.Address.FullName,
		PickupName:          pickupName,
		PickupAddress:       pickupAddr,
		CustomerPhone:       o.Address.Phone,
		TotalAmount:         o.TotalAmount,
		PaymentMethod:       o.PaymentMethod,
		ItemCount:           len(o.Items),
		CollectedVia:        o.CollectedVia,
		Items:               itemSummaries,
		CreatedAt:           o.CreatedAt,
		DeliveryLat:         o.Address.Lat,
		DeliveryLng:         o.Address.Lng,
	}
}

// GetMyDeliveries godoc
// GET /api/v1/delivery/orders (delivery partner only)
// Returns orders assigned to the logged-in delivery partner, identified
// solely from the verified JWT ("user_id") - never from a client-supplied
// delivery_boy_id, so one partner can never list another's orders (IDOR).
// Optional ?status= filters by order status (e.g. confirmed, shipped,
// delivered); optional ?assignment_status= filters by
// assigned/accepted/rejected/expired.
func GetMyDeliveries(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	query := database.DB.
		Preload("Address").
		Preload("Warehouse").
		Preload("Items.Product").
		Where("delivery_partner_id = ?", partnerID)

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if assignmentStatus := c.Query("assignment_status"); assignmentStatus != "" {
		query = query.Where("delivery_assignment_status = ?", assignmentStatus)
	}

	var orders []models.Order
	if err := query.Order("created_at DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assigned orders"})
		return
	}

	summaries := make([]AssignedOrderSummary, 0, len(orders))
	for _, o := range orders {
		summaries = append(summaries, toAssignedOrderSummary(o))
	}
	attachStoreStaff(orders, summaries)

	c.JSON(http.StatusOK, gin.H{"orders": summaries})
}

// ---------------------------------------------------------------------------
// Assignment accept / reject (delivery partner only)
// ---------------------------------------------------------------------------

// AcceptAssignment godoc
// PUT /api/v1/delivery/orders/:id/accept (delivery partner only)
// Moves a pending assignment ASSIGNED -> ACCEPTED. Only the partner the
// order is currently assigned to can accept it (IDOR/BOLA protection via
// services.RespondToAssignment). A conditional UPDATE guards against a
// double-accept race, and against racing an in-flight timeout expiry.
func AcceptAssignment(c *gin.Context) {
	respondToAssignment(c, models.DeliveryAssignmentStatusAccepted, "")
}

// RejectAssignment godoc
// PUT /api/v1/delivery/orders/:id/reject (delivery partner only)
// Moves a pending assignment ASSIGNED -> REJECTED, then automatically
// tries the next eligible partner who hasn't already been offered this
// order (services.TryAssignNextPartner). The order is never deleted.
func RejectAssignment(c *gin.Context) {
	var req models.RejectAssignmentRequest
	// Body is optional - an empty/absent body just means no reason given.
	_ = c.ShouldBindJSON(&req)
	respondToAssignment(c, models.DeliveryAssignmentStatusRejected, req.Reason)
}

func respondToAssignment(c *gin.Context, newStatus string, reason string) {
	partnerID := c.MustGet("user_id").(uint)

	orderID64, convErr := strconv.ParseUint(c.Param("id"), 10, 64)
	if convErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}

	order, err := services.RespondToAssignment(uint(orderID64), partnerID, newStatus, reason)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAssignmentOrderNotOwned):
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		case errors.Is(err, services.ErrAssignmentNotPending):
			c.JSON(http.StatusBadRequest, gin.H{"error": "This assignment has already been responded to or is not in a pending state"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update assignment"})
		}
		return
	}

	msg := "Delivery accepted"
	if newStatus == models.DeliveryAssignmentStatusRejected {
		msg = "Delivery rejected"
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "order": toAssignedOrderSummary(*order)})
}

// UpdateDeliveryStatus godoc
// PUT /api/v1/delivery/orders/:id/delivery-status (delivery partner only)
// Advances the granular delivery lifecycle one step at a time:
// ACCEPTED -> PICKED_UP -> OUT_FOR_DELIVERY -> ARRIVED -> DELIVERED.
// ASSIGNED and ACCEPTED are set automatically by the existing
// assign/accept flow, so the earliest status a partner can set here is
// PICKED_UP (see models.UpdateDeliveryStatusRequest's oneof binding).
// Only the partner the order is currently assigned to can advance it
// (IDOR/BOLA protection via services.UpdateDeliveryStatus), and an
// invalid/out-of-order transition is rejected.
//
// The ARRIVED -> DELIVERED step additionally requires a valid "otp" in
// the request body (matching the OTP generated when the order entered
// OUT_FOR_DELIVERY) and the partner's last-known GPS to be within the
// configured delivery geofence of the address - see
// services.UpdateDeliveryStatus for the exact checks and error cases.
func UpdateDeliveryStatus(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	orderID64, convErr := strconv.ParseUint(c.Param("id"), 10, 64)
	if convErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}

	var req models.UpdateDeliveryStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The second return value (the plaintext OTP, only non-empty on the
	// OUT_FOR_DELIVERY step) is intentionally discarded here - it must
	// never be included in a delivery-partner-facing API response.
	order, _, err := services.UpdateDeliveryStatus(uint(orderID64), partnerID, req.Status, req.OTP)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeliveryStatusOrderNotOwned):
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		case errors.Is(err, services.ErrDeliveryStatusInvalidTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delivery status transition"})
		case errors.Is(err, services.ErrDeliveryOTPRequired),
			errors.Is(err, services.ErrDeliveryOTPInvalid),
			errors.Is(err, services.ErrDeliveryOTPExpired),
			errors.Is(err, services.ErrDeliveryOTPLocked):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrDeliveryGPSMissing),
			errors.Is(err, services.ErrDeliveryAddressLocationMissing),
			errors.Is(err, services.ErrDeliveryOutsideGeofence):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update delivery status"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Delivery status updated", "order": toAssignedOrderSummary(*order)})
}

// ResolveFailedDelivery godoc
// PUT /api/v1/delivery/orders/:id/resolve-failed (delivery partner only)
// Explicitly resolves an order stuck in FAILED_DELIVERY - either "retry"
// (back out for delivery, with a fresh OTP) or "return" (terminal, back to
// the store). Always requires a reason, recorded for admin/ops visibility.
func ResolveFailedDelivery(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	orderID64, convErr := strconv.ParseUint(c.Param("id"), 10, 64)
	if convErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}
	var req models.ResolveFailedDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	order, _, err := services.ResolveFailedDelivery(uint(orderID64), partnerID, req.Action, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeliveryStatusOrderNotOwned):
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		case errors.Is(err, services.ErrDeliveryStatusInvalidTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order is not in a failed-delivery state, or the requested action is invalid"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve delivery"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Delivery resolved", "order": toAssignedOrderSummary(*order)})
}

// UpdateDeliveryOrderStatus godoc
// PUT /api/v1/delivery/orders/:id/status (delivery partner only)
// Lets the assigned partner move an order from confirmed -> shipped
// (picked up and out for delivery). Partners cannot set "delivered"
// here - see ConfirmDelivery below, which also handles COD collection.
func UpdateDeliveryOrderStatus(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	orderID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required,oneof=shipped"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var order models.Order
	if err := database.DB.Preload("Address").Preload("Items.Product").Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}

	if order.Status != models.OrderStatusHandedOver {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order must be handed over by the warehouse before it can be marked shipped"})
		return
	}

	order.Status = req.Status
	if err := database.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	// Notify the customer that their order is out for delivery.
	go services.SendPushToUserWithData(order.UserID, "Order out for delivery", fmt.Sprintf("Your order #%d is on its way", order.ID), services.OrderPushData(order.ID))

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated", "order": order})
}

// UploadDeliveryProof godoc
// PUT /api/v1/delivery/orders/:id/delivery-proof (delivery partner only)
// Accepts a multipart/form-data "image" field, uploads it via the same
// Cloudinary/local-disk path as UploadImage, and stores the resulting URL
// on the order as delivery_proof_url. This is mandatory before
// ConfirmDelivery will succeed - see the check there.
func UploadDeliveryProof(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	orderID := c.Param("id")

	var order models.Order
	if err := database.DB.Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}

	// Photo upload is only allowed when the partner is at the delivery address.
	if err := services.VerifyDeliveryGeofenceForOrder(&order, partnerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image file provided (expected form field 'image')"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type. Allowed: jpg, jpeg, png, webp"})
		return
	}

	var proofURL string
	cfg := config.AppConfig
	if cfg != nil && cfg.CloudinaryCloudName != "" && cfg.CloudinaryAPIKey != "" && cfg.CloudinaryAPISecret != "" {
		url, err := uploadToCloudinary(file, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image: " + err.Error()})
			return
		}
		proofURL = url
	} else {
		filename := fmt.Sprintf("delivery-proof-%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join("uploads", filename)
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
			return
		}
		proofURL = cfg.PublicBaseURL + "/uploads/" + filename
	}

	if err := database.DB.Model(&models.Order{}).Where("id = ?", order.ID).Update("delivery_proof_url", proofURL).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save delivery proof"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"delivery_proof_url": proofURL})
}

// ConfirmDelivery godoc
// PUT /api/v1/delivery/orders/:id/deliver (delivery partner only)
// Marks the order delivered. For COD orders this also marks payment as
// paid, since cash is collected at the same time.
func ConfirmDelivery(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	orderID := c.Param("id")
	// Optional body from the rider app: how the COD amount was collected (cash|upi).
	var body struct {
		CollectedVia string `json:"collected_via"`
	}
	_ = c.ShouldBindJSON(&body)

	var order models.Order
	if err := database.DB.Preload("Address").Preload("Items.Product").Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}

	if order.Status == models.OrderStatusDelivered {
		c.JSON(http.StatusOK, gin.H{"message": "Order is already marked delivered", "order": order})
		return
	}
	// Bypass fix: require secure geofence delivery-status flow to have already completed.
	if order.DeliveryStatus == nil || *order.DeliveryStatus != models.DeliveryStatusDelivered {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order must complete geofence verification via the delivery-status endpoint before it can be confirmed delivered"})
		return
	}
	// Delivery proof photo (uploaded at ARRIVED_AT_CUSTOMER via the
	// delivery-proof endpoint) is mandatory before delivery can be confirmed.
	if order.DeliveryProofURL == nil || *order.DeliveryProofURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Delivery proof photo is required before confirming delivery"})
		return
	}
	// The granular delivery_status chain (going_to_store -> ... -> delivered) is
	// now the source of truth for pickup/shipment progress under the new flow,
	// so promote order.Status to Shipped here if the older manual "mark as
	// shipped" step was skipped, before finalizing as Delivered.
	if order.Status != models.OrderStatusShipped {
		order.Status = models.OrderStatusShipped
	}
	order.Status = models.OrderStatusDelivered
	deliveredAt := time.Now()
	order.DeliveredAt = &deliveredAt
	if order.PaymentMethod == models.PaymentMethodCOD {
		order.PaymentStatus = models.OrderPaymentStatusPaid
	}
	if order.PaymentMethod == models.PaymentMethodCOD {
		via := "cash"
		if body.CollectedVia == "upi" {
			via = "upi"
			st := "verified"
			order.UPIStatus = &st
		}
		order.CollectedVia = &via
	}

	if err := database.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm delivery"})
		return
	}

	// Revenue is recognized now (COD delivery just confirmed) - post the
	// double-entry sales ledger entry at this exact moment, not earlier.
	// GenerateInvoiceIfNotExists is idempotent (safe re-call) - this is a
	// safety net in case the checkout-time COD invoice generation
	// (order_handler.go) failed or was skipped, so ledger posting below
	// is never silently blocked by a missing invoice.
	if _, err := services.GenerateInvoiceIfNotExists(order.ID); err != nil {
		log.Printf("CRITICAL: failed to generate invoice for delivered order %s, ledger will not post: %v", orderID, err)
	}
	if err := services.PostSalesLedgerEntry(order.ID); err != nil {
		log.Printf("CRITICAL: failed to post sales ledger entry for order %s - revenue untracked: %v", orderID, err)
	}

	// COD cash alerts at 80% / 100% of the limit. UPI is not cash held by the rider.
	if order.PaymentMethod == models.PaymentMethodCOD && body.CollectedVia != "upi" {
		go services.NotifyCODThresholds(partnerID, order.TotalAmount, order.ID)
	}

	// Notify the customer that their order has been delivered.
	go services.SendPushToUserWithData(order.UserID, "Order delivered", fmt.Sprintf("Your order #%d has been delivered. Enjoy!", order.ID), services.OrderPushData(order.ID))

	// Rider must scan the store QR again before getting another
	// auto-assigned order, so clear the store check-in now.
	if err := services.ClearPartnerCheckin(partnerID); err != nil {
		log.Printf("failed to clear store check-in for partner %d: %v", partnerID, err)
	}

	// This partner just freed up - immediately try to backfill any
	// orders that were left unassigned because every partner was busy.
	go services.TryAssignPendingOrdersToPartner(partnerID)

	c.JSON(http.StatusOK, gin.H{"message": "Delivery confirmed", "order": order})
}

// GetMyEarnings godoc
// GET /api/v1/delivery/earnings (delivery partner only)
// Returns delivery earnings summary: today's earnings, total earnings,
// total delivered count, and a list of delivered orders with the flat
// per-delivery payout. There is no separate earnings/payout table yet,
// so this is computed on the fly at a fixed rate per delivered order.
const perDeliveryEarning = 30.0

// GetDeliveryPartnerLocation godoc
// GET /api/v1/admin/delivery-partners/:id/location (admin only)
// Returns the partner's last known location, pushed via PUT /delivery/location.
func GetDeliveryPartnerLocation(c *gin.Context) {
	id := c.Param("id")
	var partner models.DeliveryPartner
	if err := database.DB.Select("id", "name", "current_lat", "current_lng", "last_location_update").First(&partner, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                   partner.ID,
		"name":                 partner.Name,
		"current_lat":          partner.CurrentLat,
		"current_lng":          partner.CurrentLng,
		"last_location_update": partner.LastLocationUpdate,
	})
}

func GetMyEarnings(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	var deliveredOrders []models.Order
	if err := database.DB.
		Where("delivery_partner_id = ? AND (status = ? OR delivered_at IS NOT NULL OR delivery_status = ?)", partnerID, models.OrderStatusDelivered, models.DeliveryStatusDelivered).
		Order("COALESCE(delivered_at, updated_at) DESC").
		Find(&deliveredOrders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load earnings"})
		return
	}

	var handedReturns []models.ReturnRequest
	if err := database.DB.
		Where("delivery_partner_id = ? AND pickup_status = ? AND handed_over_at IS NOT NULL", partnerID, models.PickupStatusHandedOver).
		Find(&handedReturns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load earnings"})
		return
	}
	totalCount := len(deliveredOrders) + len(handedReturns)
	totalEarnings := float64(totalCount) * perDeliveryEarning

	todayStart := time.Now().Truncate(24 * time.Hour)
	todayCount := 0
	todayEarnings := 0.0
	type EarningEntry struct {
		OrderID     uint      `json:"order_id"`
		Amount      float64   `json:"amount"`
		DeliveredAt time.Time `json:"delivered_at"`
		Type        string    `json:"type"`
	}
	entries := make([]EarningEntry, 0, len(deliveredOrders)+len(handedReturns))
	weekStart := todayStart.AddDate(0, 0, -6)
	weekCount := 0

	for _, o := range deliveredOrders {
		if deliveredTime(o).After(todayStart) {
			todayCount++
			todayEarnings += perDeliveryEarning
		}
		entries = append(entries, EarningEntry{
			OrderID:     o.ID,
			Amount:      perDeliveryEarning,
			DeliveredAt: deliveredTime(o),
			Type:        "delivery",
		})
	}

	for _, o := range deliveredOrders {
		if !deliveredTime(o).Before(weekStart) {
			weekCount++
		}
	}
	for _, r := range handedReturns {
		if r.HandedOverAt == nil {
			continue
		}
		at := *r.HandedOverAt
		if at.After(todayStart) {
			todayCount++
			todayEarnings += perDeliveryEarning
		}
		if !at.Before(weekStart) {
			weekCount++
		}
		entries = append(entries, EarningEntry{
			OrderID:     r.OrderID,
			Amount:      perDeliveryEarning,
			DeliveredAt: at,
			Type:        "return",
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].DeliveredAt.After(entries[j].DeliveredAt) })
	weekEarnings := float64(weekCount) * perDeliveryEarning

	c.JSON(http.StatusOK, gin.H{
		"per_delivery_rate": perDeliveryEarning,
		"today_earnings":    todayEarnings,
		"today_deliveries":  todayCount,
		"total_earnings":    totalEarnings,
		"total_deliveries":  totalCount,
		"week_earnings":     weekEarnings,
		"week_deliveries":   weekCount,
		"entries":           entries,
	})
}

// deliveredTime is the real delivery time when recorded, else UpdatedAt
// (orders delivered before delivered_at existed).
func deliveredTime(o models.Order) time.Time {
	if o.DeliveredAt != nil {
		return *o.DeliveredAt
	}
	return o.UpdatedAt
}

// attachStoreStaff fills the store contact (packer, else picker) on each
// summary so the rider knows who to collect the order from.
func attachStoreStaff(orders []models.Order, summaries []AssignedOrderSummary) {
	if len(orders) == 0 {
		return
	}
	ids := make([]uint, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	staffByOrder := make(map[uint]uint)
	var picks []models.PickingTask
	database.DB.Where("order_id IN ?", ids).Find(&picks)
	for _, pk := range picks {
		if pk.PickerID != nil {
			staffByOrder[pk.OrderID] = *pk.PickerID
		}
	}
	var packs []models.PackingTask
	database.DB.Where("order_id IN ?", ids).Find(&packs)
	for _, pk := range packs {
		if pk.PackerID != nil {
			staffByOrder[pk.OrderID] = *pk.PackerID
		}
	}
	if len(staffByOrder) == 0 {
		return
	}
	staffIDs := make([]uint, 0, len(staffByOrder))
	for _, id := range staffByOrder {
		staffIDs = append(staffIDs, id)
	}
	var staff []models.WarehouseStaff
	database.DB.Where("id IN ?", staffIDs).Find(&staff)
	byID := make(map[uint]models.WarehouseStaff, len(staff))
	for _, s := range staff {
		byID[s.ID] = s
	}
	for i := range summaries {
		if sid, ok := staffByOrder[summaries[i].OrderID]; ok {
			if s, ok2 := byID[sid]; ok2 {
				summaries[i].StoreStaffName = s.Name
				summaries[i].StoreStaffPhone = s.Phone
			}
		}
	}
}
