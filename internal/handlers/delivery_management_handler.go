package handlers

import (
"net/http"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// ActiveDeliverySummary is one row in the admin's active-deliveries view -
// an order that has been assigned/accepted by a partner and is somewhere
// in the courier-driven delivery lifecycle, but not yet delivered.
type ActiveDeliverySummary struct {
OrderID            uint    `json:"order_id"`
OrderStatus        string  `json:"order_status"`
DeliveryStatus     string  `json:"delivery_status"`
DeliveryPartnerID  uint    `json:"delivery_partner_id"`
PartnerName        string  `json:"partner_name"`
PartnerPhone       string  `json:"partner_phone"`
CustomerArea       string  `json:"customer_area"`
AssignedAt         string  `json:"assigned_at"`
}

// GetActiveDeliveries godoc
// GET /api/v1/admin/delivery/active (admin only)
// Every order currently in the courier-driven delivery lifecycle
// (assigned through arrived, i.e. everything except delivered) - the
// live in-flight view the report calls "active deliveries".
func GetActiveDeliveries(c *gin.Context) {
activeStatuses := []string{
models.DeliveryStatusAssigned,
models.DeliveryStatusAccepted,
models.DeliveryStatusGoingToStore,
models.DeliveryStatusArrivedAtStore,
models.DeliveryStatusPickedUp,
models.DeliveryStatusOutForDelivery,
models.DeliveryStatusArrivedAtCustomer,
models.DeliveryStatusFailedDelivery,
models.DeliveryStatusArrived,
}

var orders []models.Order
if err := database.DB.
Preload("DeliveryPartner").
Preload("Address").
Where("delivery_status IN ? AND delivery_partner_id IS NOT NULL AND status NOT IN ?", activeStatuses, []string{models.OrderStatusDelivered, models.OrderStatusCancelled, models.OrderStatusReturned}).
Order("updated_at desc").
Find(&orders).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load active deliveries"})
return
}

summaries := make([]ActiveDeliverySummary, 0, len(orders))
for _, o := range orders {
s := ActiveDeliverySummary{
OrderID:     o.ID,
OrderStatus: o.Status,
}
if o.DeliveryStatus != nil {
s.DeliveryStatus = *o.DeliveryStatus
}
if o.DeliveryPartnerID != nil {
s.DeliveryPartnerID = *o.DeliveryPartnerID
}
if o.DeliveryPartner != nil {
s.PartnerName = o.DeliveryPartner.Name
s.PartnerPhone = o.DeliveryPartner.Phone
}
s.CustomerArea = o.Address.City
s.AssignedAt = o.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
summaries = append(summaries, s)
}

c.JSON(http.StatusOK, gin.H{
"active_count": len(summaries),
"deliveries":   summaries,
})
}

// RiderWorkloadSummary is one delivery partner's current load vs capacity.
type RiderWorkloadSummary struct {
PartnerID       uint   `json:"partner_id"`
Name            string `json:"name"`
Phone           string `json:"phone"`
IsOnline        bool   `json:"is_online"`
IsActive        bool   `json:"is_active"`
ActiveOrders    int64  `json:"active_orders"`
MaxActiveOrders int    `json:"max_active_orders"`
}

// GetRiderWorkload godoc
// GET /api/v1/admin/delivery/rider-workload (admin only)
// Every delivery partner with their current active-order count against
// their capacity, so admin can see who's overloaded vs free at a glance.
func GetRiderWorkload(c *gin.Context) {
var partners []models.DeliveryPartner
if err := database.DB.Order("is_online desc, name asc").Find(&partners).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load delivery partners"})
return
}

activeStatuses := []string{
models.DeliveryStatusAssigned,
models.DeliveryStatusAccepted,
models.DeliveryStatusGoingToStore,
models.DeliveryStatusArrivedAtStore,
models.DeliveryStatusPickedUp,
models.DeliveryStatusOutForDelivery,
models.DeliveryStatusArrivedAtCustomer,
models.DeliveryStatusFailedDelivery,
models.DeliveryStatusArrived,
}

summaries := make([]RiderWorkloadSummary, 0, len(partners))
for _, p := range partners {
var count int64
database.DB.Model(&models.Order{}).
Where("delivery_partner_id = ? AND delivery_status IN ?", p.ID, activeStatuses).
Count(&count)

summaries = append(summaries, RiderWorkloadSummary{
PartnerID:       p.ID,
Name:            p.Name,
Phone:           p.Phone,
IsOnline:        p.IsOnline,
IsActive:        p.IsActive,
ActiveOrders:    count,
MaxActiveOrders: p.MaxActiveOrders,
})
}

c.JSON(http.StatusOK, gin.H{"riders": summaries})
}

// FailedDeliverySummary is one order that reached the customer but was
// returned - the delivery equivalent of a failed outcome, since there is
// no separate "delivery_failed" state in the model (a rejected/expired
// *assignment* just goes back to the unassigned-orders pool, not here).
type FailedDeliverySummary struct {
OrderID      uint   `json:"order_id"`
PartnerID    uint   `json:"partner_id,omitempty"`
PartnerName  string `json:"partner_name,omitempty"`
CustomerArea string `json:"customer_area"`
ReturnedAt   string `json:"returned_at"`
}

// GetFailedDeliveries godoc
// GET /api/v1/admin/delivery/failed (admin only)
// Orders whose final outcome was "returned" rather than delivered - the
// admin-visible record of deliveries that didn't succeed.
func GetFailedDeliveries(c *gin.Context) {
var orders []models.Order
if err := database.DB.
Preload("DeliveryPartner").
Preload("Address").
Where("status = ?", models.OrderStatusReturned).
Order("updated_at desc").
Limit(200).
Find(&orders).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load failed deliveries"})
return
}

summaries := make([]FailedDeliverySummary, 0, len(orders))
for _, o := range orders {
s := FailedDeliverySummary{
OrderID:      o.ID,
CustomerArea: o.Address.City,
ReturnedAt:   o.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
}
if o.DeliveryPartnerID != nil {
s.PartnerID = *o.DeliveryPartnerID
}
if o.DeliveryPartner != nil {
s.PartnerName = o.DeliveryPartner.Name
}
summaries = append(summaries, s)
}

c.JSON(http.StatusOK, gin.H{
"failed_count": len(summaries),
"deliveries":   summaries,
})
}
