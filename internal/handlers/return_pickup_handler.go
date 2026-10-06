package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// ---------------------------------------------------------------------------
// Return pickups - assignment (admin-panel + store-app) and the delivery
// partner's own accept/reject/status flow.
// ---------------------------------------------------------------------------

func parseReturnID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid return id"})
		return 0, false
	}
	return uint(id), true
}

// AssignReturnPickup godoc
// PUT /api/v1/admin/returns/:id/assign-pickup   (admin)
// PUT /api/v1/warehouse/returns/:id/assign-pickup (store, warehouse-scoped)
func AssignReturnPickup(c *gin.Context) {
	returnReqID, ok := parseReturnID(c)
	if !ok {
		return
	}

	var body struct {
		DeliveryPartnerID uint `json:"delivery_partner_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store-app route is warehouse-scoped - make sure this return's order
	// actually belongs to that warehouse, same guard as ApproveReturn.
	if wid, exists := c.Get("warehouse_id"); exists {
		var returnReq models.ReturnRequest
		if err := database.DB.First(&returnReq, returnReqID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Return request not found"})
			return
		}
		var order models.Order
		if err := database.DB.First(&order, returnReq.OrderID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if order.WarehouseID == nil || *order.WarehouseID != wid.(uint) {
			c.JSON(http.StatusForbidden, gin.H{"error": "This return does not belong to your warehouse"})
			return
		}
	}

	if err := services.AssignReturnPickup(returnReqID, body.DeliveryPartnerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pickup assigned"})
}

// --- Delivery partner's own actions ----------------------------------------
// NOTE: assumes the partner's JWT middleware sets "delivery_partner_id" in
// context, matching how "user_id"/"warehouse_id" are set for admin/store
// auth. If your delivery-partner middleware uses a different key, update
// partnerIDFromContext below - it's the only place that reads it.

func partnerIDFromContext(c *gin.Context) (uint, bool) {
	v, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "delivery partner not authenticated"})
		return 0, false
	}
	return v.(uint), true
}

// AcceptReturnPickup godoc
// PUT /api/v1/delivery/returns/:id/accept
func AcceptReturnPickup(c *gin.Context) {
	returnReqID, ok := parseReturnID(c)
	if !ok {
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	if err := services.AcceptReturnPickup(returnReqID, partnerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pickup accepted"})
}

// RejectReturnPickup godoc
// PUT /api/v1/delivery/returns/:id/reject
func RejectReturnPickup(c *gin.Context) {
	returnReqID, ok := parseReturnID(c)
	if !ok {
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	if err := services.RejectReturnPickup(returnReqID, partnerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pickup rejected, open for reassignment"})
}

// MarkReturnPickupEnRoute godoc
// PUT /api/v1/delivery/returns/:id/en-route
func MarkReturnPickupEnRoute(c *gin.Context) {
	returnReqID, ok := parseReturnID(c)
	if !ok {
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	if err := services.MarkReturnPickupEnRoute(returnReqID, partnerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "marked en route"})
}

// MarkReturnPickupArrived godoc
// PUT /api/v1/delivery/returns/:id/arrived
func MarkReturnPickupArrived(c *gin.Context) {
	returnReqID, ok := parseReturnID(c)
	if !ok {
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	if err := services.MarkReturnPickupArrived(returnReqID, partnerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "marked arrived"})
}

// ConfirmReturnPickedUp godoc
// PUT /api/v1/delivery/returns/:id/picked-up
// body: { "condition_notes": "...", "condition_photo_url": "..." }
func ConfirmReturnPickedUp(c *gin.Context) {
	returnReqID, ok := parseReturnID(c)
	if !ok {
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}

	var body struct {
		ConditionNotes    string `json:"condition_notes"`
		ConditionPhotoURL string `json:"condition_photo_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.ConfirmReturnPickedUp(returnReqID, partnerID, body.ConditionNotes, body.ConditionPhotoURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pickup confirmed"})
}

// HandoverReturnToWarehouse godoc
// PUT /api/v1/delivery/returns/:id/handover
func HandoverReturnToWarehouse(c *gin.Context) {
	returnReqID, ok := parseReturnID(c)
	if !ok {
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	if err := services.HandoverReturnToWarehouse(returnReqID, partnerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "handed over to warehouse"})
}

// ---------------------------------------------------------------------------
// Return pickups - list for delivery partner (mirrors GetMyDeliveries' shape
// so the Flutter app can render both in one merged list).
// ---------------------------------------------------------------------------

// ReturnPickupItemSummary mirrors OrderItemSummary for a return line item.
type ReturnPickupItemSummary struct {
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
}

// ReturnPickupSummary is the shape returned by GET /delivery/returns. Field
// names deliberately echo AssignedOrderSummary (order_id, pickup_status,
// delivery_address, customer_name, customer_phone, items, created_at) so
// the Flutter app can treat both as one polymorphic list, distinguished by
// a "type": "return_pickup" tag added by the handler below.
type ReturnPickupSummary struct {
	ReturnRequestID           uint                      `json:"return_request_id"`
	OrderID                   uint                      `json:"order_id"`
	Type                      string                    `json:"type"` // always "return_pickup"
	PickupStatus              *string                   `json:"pickup_status,omitempty"`
	PickupAssignmentExpiresAt *time.Time                `json:"pickup_assignment_expires_at,omitempty"`
	DeliveryAddress           string                    `json:"delivery_address"`
	CustomerName              string                    `json:"customer_name"`
	CustomerPhone             string                    `json:"customer_phone"`
	DeliveryLat               *float64                  `json:"delivery_lat,omitempty"`
	DeliveryLng               *float64                  `json:"delivery_lng,omitempty"`
	PickupName                string                    `json:"pickup_name,omitempty"`
	PickupAddress             string                    `json:"pickup_address,omitempty"`
	PickupLat                 *float64                  `json:"pickup_lat,omitempty"`
	PickupLng                 *float64                  `json:"pickup_lng,omitempty"`
	Reason                    string                    `json:"reason"`
	RefundAmount              float64                   `json:"refund_amount"`
	ItemCount                 int                       `json:"item_count"`
	Items                     []ReturnPickupItemSummary `json:"items"`
	CreatedAt                 time.Time                 `json:"created_at"`
}

func toReturnPickupSummary(rr models.ReturnRequest) ReturnPickupSummary {
	addr := fmt.Sprintf("%s, %s, %s - %s",
		rr.Order.Address.Line1, rr.Order.Address.City, rr.Order.Address.State, rr.Order.Address.Pincode)
	if rr.Order.Address.Line2 != "" {
		addr = fmt.Sprintf("%s, %s, %s, %s - %s",
			rr.Order.Address.Line1, rr.Order.Address.Line2, rr.Order.Address.City, rr.Order.Address.State, rr.Order.Address.Pincode)
	}
	itemSummaries := make([]ReturnPickupItemSummary, 0, len(rr.Items))
	for _, ri := range rr.Items {
		itemSummaries = append(itemSummaries, ReturnPickupItemSummary{
			ProductName: ri.OrderItem.Product.Name,
			Quantity:    ri.Quantity,
		})
	}
	return ReturnPickupSummary{
		ReturnRequestID:           rr.ID,
		OrderID:                   rr.OrderID,
		Type:                      "return_pickup",
		PickupStatus:              rr.PickupStatus,
		PickupAssignmentExpiresAt: rr.PickupAssignmentExpiresAt,
		DeliveryAddress:           addr,
		CustomerName:              rr.Order.Address.FullName,
		CustomerPhone:             rr.Order.Address.Phone,
		DeliveryLat:               rr.Order.Address.Lat,
		DeliveryLng:               rr.Order.Address.Lng,
		Reason:                    rr.Reason,
		RefundAmount:              rr.RefundAmount,
		ItemCount:                 len(rr.Items),
		Items:                     itemSummaries,
		CreatedAt:                 rr.CreatedAt,
	}
}

// GetMyReturnPickups godoc
// GET /api/v1/delivery/returns (delivery partner only)
// Returns return-pickups assigned to the logged-in delivery partner,
// identified solely from the verified JWT ("user_id") - same IDOR
// protection as GetMyDeliveries. Optional ?pickup_status= filters by
// assigned/accepted/en_route/arrived/picked_up/handed_over.
func GetMyReturnPickups(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)

	query := database.DB.
		Preload("Order.Address").
		Preload("Items.OrderItem.Product").
		Where("delivery_partner_id = ?", partnerID)

	if pickupStatus := c.Query("pickup_status"); pickupStatus != "" {
		query = query.Where("pickup_status = ?", pickupStatus)
	}

	var returnReqs []models.ReturnRequest
	if err := query.Order("created_at DESC").Find(&returnReqs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assigned return pickups"})
		return
	}

	whByID := map[uint]models.Warehouse{}
	{
		ids := []uint{}
		for _, rr := range returnReqs {
			if rr.Order.WarehouseID != nil {
				ids = append(ids, *rr.Order.WarehouseID)
			}
		}
		if len(ids) > 0 {
			var whs []models.Warehouse
			database.DB.Where("id IN ?", ids).Find(&whs)
			for _, w := range whs {
				whByID[w.ID] = w
			}
		}
	}

	summaries := make([]ReturnPickupSummary, 0, len(returnReqs))
	for _, rr := range returnReqs {
		s := toReturnPickupSummary(rr)
		if rr.Order.WarehouseID != nil {
			if w, ok := whByID[*rr.Order.WarehouseID]; ok {
				lat, lng := w.Lat, w.Lng
				s.PickupName = w.Name
				s.PickupAddress = w.Address
				s.PickupLat = &lat
				s.PickupLng = &lng
			}
		}
		summaries = append(summaries, s)
	}

	c.JSON(http.StatusOK, gin.H{"returns": summaries})
}
