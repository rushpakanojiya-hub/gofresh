package handlers

import (
"net/http"
"strings"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// LiveOperationsResponse is the response for GET /admin/control-tower/operations.
// Delayed/Failed are intentionally nil (not 0) - there is no SLA threshold or
// failure definition in the system yet, so "we don't know" must be visually
// distinct from "we checked and found zero".
type LiveOperationsResponse struct {
ActiveOrders   int64  `json:"active_orders"`
Picking        int64  `json:"picking"`
Packed         int64  `json:"packed"`
ReadyForPickup int64  `json:"ready_for_pickup"`
OutForDelivery int64  `json:"out_for_delivery"`
ActiveRiders   int64  `json:"active_riders"`
Delayed        *int64 `json:"delayed"`
Failed         *int64 `json:"failed"`
}

// GetLiveOperations godoc
// GET /api/v1/admin/control-tower/operations (admin only)
// Order-status and active-rider counts for the Control Tower's Live
// Operations panel. Every count here is a direct read off an existing
// column - no derived/estimated numbers. Delayed and Failed are left null:
// there's no SLA threshold or failure definition in the system to compute
// them from, and inventing one here would be a business decision made by
// the wrong person.
func GetLiveOperations(c *gin.Context) {
var resp LiveOperationsResponse

database.DB.Model(&models.Order{}).
Where("status NOT IN ?", []string{models.OrderStatusDelivered, models.OrderStatusCancelled, models.OrderStatusReturned}).
Count(&resp.ActiveOrders)

database.DB.Model(&models.Order{}).
Where("status IN ?", []string{models.OrderStatusPicking, models.OrderStatusPicked}).
Count(&resp.Picking)

database.DB.Model(&models.Order{}).
Where("status IN ?", []string{models.OrderStatusPacking, models.OrderStatusPacked}).
Count(&resp.Packed)

database.DB.Model(&models.Order{}).
Where("status IN ?", []string{models.OrderStatusReadyForDispatch, models.OrderStatusHandedOver}).
Count(&resp.ReadyForPickup)

database.DB.Model(&models.Order{}).
Where("delivery_status IN ?", []string{models.DeliveryStatusPickedUp, models.DeliveryStatusOutForDelivery, models.DeliveryStatusArrivedAtCustomer, models.DeliveryStatusArrived}).
Count(&resp.OutForDelivery)

database.DB.Model(&models.DeliveryPartner{}).
Where("is_online = ?", true).
Count(&resp.ActiveRiders)

// Not available yet - no SLA threshold / failure definition exists.
resp.Delayed = nil
resp.Failed = nil

c.JSON(http.StatusOK, resp)
}

// UnassignedOrderItem is one row in the GetUnassignedOrders response - an
// order that exhausted every eligible delivery partner and needs manual
// admin assignment.
type UnassignedOrderItem struct {
OrderID                  uint       `json:"order_id"`
OrderStatus               string     `json:"order_status"`
DeliveryAssignmentStatus  string     `json:"delivery_assignment_status"`
WarehouseName              string     `json:"warehouse,omitempty"`
CustomerCity                string     `json:"customer_area,omitempty"`
AttemptedPartnerCount       int        `json:"attempted_partner_count"`
AssignmentExpiry             *time.Time `json:"assignment_expiry,omitempty"`
CreatedAt                     time.Time  `json:"created_at"`
}

// UnassignedOrdersResponse is the response for GET /admin/delivery/unassigned-orders.
type UnassignedOrdersResponse struct {
UnassignedCount int64                 `json:"unassigned_count"`
Orders          []UnassignedOrderItem `json:"orders"`
}

// GetUnassignedOrders godoc
// GET /api/v1/admin/delivery/unassigned-orders (admin only)
// Orders that have exhausted every eligible delivery partner (rejected or
// expired, see TryAssignNextPartner in services/delivery_acceptance.go) and
// are sitting with delivery_partner_id = NULL, needing manual admin
// assignment.
//
// Deliberately NOT just "delivery_partner_id IS NULL" - that alone also
// matches orders that simply haven't reached the assignment stage yet. The
// rejected/expired assignment-status filter is what isolates genuine
// assignment failures from normal in-flight orders.
func GetUnassignedOrders(c *gin.Context) {
var orders []models.Order
if err := database.DB.
Preload("Warehouse").
Preload("Address").
Where("delivery_partner_id IS NULL AND delivery_assignment_status IN ?",
[]string{models.DeliveryAssignmentStatusRejected, models.DeliveryAssignmentStatusExpired}).
Order("created_at ASC").
Find(&orders).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load unassigned orders"})
return
}

items := make([]UnassignedOrderItem, 0, len(orders))
for _, o := range orders {
item := UnassignedOrderItem{
OrderID:               o.ID,
OrderStatus:           o.Status,
AttemptedPartnerCount: countAttemptedPartners(o.DeliveryAttemptedPartnerIDs),
AssignmentExpiry:      o.DeliveryAssignmentExpiresAt,
CreatedAt:             o.CreatedAt,
}
if o.DeliveryAssignmentStatus != nil {
item.DeliveryAssignmentStatus = *o.DeliveryAssignmentStatus
}
if o.Warehouse != nil {
item.WarehouseName = o.Warehouse.Name
}
item.CustomerCity = o.Address.City
items = append(items, item)
}

c.JSON(http.StatusOK, UnassignedOrdersResponse{
UnassignedCount: int64(len(items)),
Orders:          items,
})
}

// countAttemptedPartners returns how many distinct partner IDs are recorded
// in a DeliveryAttemptedPartnerIDs CSV string. Kept local to this handler
// since services.parseAttemptedIDs is unexported.
func countAttemptedPartners(csv string) int {
csv = strings.TrimSpace(csv)
if csv == "" {
return 0
}
count := 0
for _, p := range strings.Split(csv, ",") {
if strings.TrimSpace(p) != "" {
count++
}
}
return count
}
// StuckOrderItem is one order that has been sitting in an active,
// partner-assigned state for longer than the stuck-order threshold without
// reaching delivered/cancelled/returned. Surfaced for manual admin review -
// see GetStuckOrders.
type StuckOrderItem struct {
    OrderID          uint      `json:"order_id"`
    OrderStatus      string    `json:"order_status"`
    DeliveryPartnerID uint     `json:"delivery_partner_id"`
    PartnerName      string    `json:"partner_name,omitempty"`
    CustomerCity     string    `json:"customer_area,omitempty"`
    LastUpdatedAt    time.Time `json:"last_updated_at"`
    HoursStuck       float64   `json:"hours_stuck"`
}

// StuckOrdersResponse is the response for GET /admin/delivery/stuck-orders.
type StuckOrdersResponse struct {
    StuckCount int64            `json:"stuck_count"`
    Orders     []StuckOrderItem `json:"orders"`
}

// stuckOrderThreshold is how long an order can sit in an active,
// partner-assigned state before it's flagged for manual review. This
// mirrors the cutoff AutoAssignDeliveryPartner uses to stop counting an
// order against a partner's active-order capacity (see
// internal/services/delivery_assignment.go) - past this point, the order is
// either genuinely stuck (partner app crashed, status never updated) or
// data-hygiene needs a look, either way an admin should see it.
const stuckOrderThreshold = 12 * time.Hour

// GetStuckOrders godoc
// GET /api/v1/admin/delivery/stuck-orders (admin only)
// Orders assigned to a delivery partner that have been sitting in an
// active (confirmed/picking/.../shipped) status for longer than
// stuckOrderThreshold without moving to delivered/cancelled/returned. These
// no longer count against the partner's capacity for new auto-assignments
// (see AutoAssignDeliveryPartner), but the order itself still needs a human
// to check what actually happened and correct its status.
func GetStuckOrders(c *gin.Context) {
    cutoff := time.Now().Add(-stuckOrderThreshold)

    var orders []models.Order
    if err := database.DB.
        Preload("DeliveryPartner").
        Preload("Address").
        Where("delivery_partner_id IS NOT NULL AND status IN ? AND updated_at < ?",
            []string{models.OrderStatusConfirmed, models.OrderStatusPicking, models.OrderStatusPicked, models.OrderStatusPacking, models.OrderStatusPacked, models.OrderStatusReadyForDispatch, models.OrderStatusHandedOver, models.OrderStatusShipped},
            cutoff).
        Order("updated_at ASC").
        Find(&orders).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load stuck orders"})
        return
    }

    items := make([]StuckOrderItem, 0, len(orders))
    for _, o := range orders {
        item := StuckOrderItem{
            OrderID:       o.ID,
            OrderStatus:   o.Status,
            LastUpdatedAt: o.UpdatedAt,
            HoursStuck:    time.Since(o.UpdatedAt).Hours(),
            CustomerCity:  o.Address.City,
        }
        if o.DeliveryPartnerID != nil {
            item.DeliveryPartnerID = *o.DeliveryPartnerID
        }
        if o.DeliveryPartner != nil {
            item.PartnerName = o.DeliveryPartner.Name
        }
        items = append(items, item)
    }

    c.JSON(http.StatusOK, StuckOrdersResponse{
        StuckCount: int64(len(items)),
        Orders:     items,
    })
}
