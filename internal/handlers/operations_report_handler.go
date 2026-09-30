package handlers

import (
"net/http"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// OperationsReport is the response for GET /admin/reports/operations.
// Every "avg minutes" field is nil when there is no data to compute it
// from (e.g. no orders in range have both AssignedAt and DeliveredAt) -
// this is intentional. Nothing here is estimated or backfilled; a nil
// value means "not enough real data yet", not zero.
type OperationsReport struct {
RangeFrom string `json:"range_from"`
RangeTo   string `json:"range_to"`

TotalOrders       int64 `json:"total_orders"`
DeliveredOrders   int64 `json:"delivered_orders"`
CancelledOrders   int64 `json:"cancelled_orders"`
ReturnedOrders    int64 `json:"returned_orders"`

CancellationRatePercent *float64 `json:"cancellation_rate_percent"`
RefundRatePercent       *float64 `json:"refund_rate_percent"`

// AvgAssignmentMinutes: order confirmed -> first delivery partner
// assigned. Nil if no orders in range have both timestamps.
AvgAssignmentMinutes *float64 `json:"avg_assignment_minutes"`
// AvgDeliveryMinutes: assigned -> delivered. Nil if no orders in range
// have both timestamps.
AvgDeliveryMinutes *float64 `json:"avg_delivery_minutes"`
AssignmentSampleSize int64  `json:"assignment_sample_size"`
DeliverySampleSize   int64  `json:"delivery_sample_size"`

// SLA: confirmed -> delivered, compared against the admin-configurable
// sla_target_minutes setting.
SLATargetMinutes     float64  `json:"sla_target_minutes"`
AvgConfirmToDeliverMinutes *float64 `json:"avg_confirm_to_deliver_minutes"`
SLAAdherencePercent  *float64 `json:"sla_adherence_percent"`
SLASampleSize        int64    `json:"sla_sample_size"`
DelayedOrders        int64    `json:"delayed_orders"`

AvgPickingMinutes *float64 `json:"avg_picking_minutes"`
AvgPackingMinutes *float64 `json:"avg_packing_minutes"`
PickingSampleSize int64    `json:"picking_sample_size"`
PackingSampleSize int64    `json:"packing_sample_size"`

OrdersPerMinute *float64 `json:"orders_per_minute"`

TotalStores   int64 `json:"total_stores"`
OpenStores    int64 `json:"open_stores"`
PausedStores  int64 `json:"paused_stores"`
ClosedStores  int64 `json:"closed_stores"`

TotalRiders  int64 `json:"total_riders"`
OnlineRiders int64 `json:"online_riders"`
}

// GetOperationsReport godoc
// GET /api/v1/admin/reports/operations?from=2026-08-01&to=2026-08-31 (admin only)
// Operational metrics the report calls out explicitly: assignment time,
// delivery time, SLA/ETA adherence, cancellation/refund rate, picker/packing
// time, orders/minute, and store/rider availability. Every timing metric is
// computed strictly from real Order/PickingTask/PackingTask timestamps -
// nothing here is estimated, and a nil average means "no qualifying orders
// in this range", not zero.
func GetOperationsReport(c *gin.Context) {
fromStr := c.Query("from")
toStr := c.Query("to")

var from, to time.Time
var err error
if fromStr != "" {
from, err = time.Parse("2006-01-02", fromStr)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'from' date, expected YYYY-MM-DD"})
return
}
} else {
from = time.Now().AddDate(0, 0, -7)
}
if toStr != "" {
to, err = time.Parse("2006-01-02", toStr)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'to' date, expected YYYY-MM-DD"})
return
}
to = to.Add(24 * time.Hour)
} else {
to = time.Now()
}

report := OperationsReport{
RangeFrom: from.Format("2006-01-02"),
RangeTo:   to.Add(-24 * time.Hour).Format("2006-01-02"),
}

database.DB.Model(&models.Order{}).Where("created_at >= ? AND created_at < ?", from, to).Count(&report.TotalOrders)
database.DB.Model(&models.Order{}).Where("created_at >= ? AND created_at < ? AND status = ?", from, to, models.OrderStatusDelivered).Count(&report.DeliveredOrders)
database.DB.Model(&models.Order{}).Where("created_at >= ? AND created_at < ? AND status = ?", from, to, models.OrderStatusCancelled).Count(&report.CancelledOrders)
database.DB.Model(&models.Order{}).Where("created_at >= ? AND created_at < ? AND status = ?", from, to, models.OrderStatusReturned).Count(&report.ReturnedOrders)

if report.TotalOrders > 0 {
rate := float64(report.CancelledOrders) / float64(report.TotalOrders) * 100
report.CancellationRatePercent = &rate
}
if report.DeliveredOrders > 0 {
rate := float64(report.ReturnedOrders) / float64(report.DeliveredOrders) * 100
report.RefundRatePercent = &rate
}

// Assignment time: confirmed (created_at, since orders go
// pending->confirmed near-instantly at checkout for COD/zero-total) ->
// assigned_at. Only orders that actually have both timestamps count.
type avgRow struct {
AvgMinutes *float64
SampleSize int64
}
var assignRow avgRow
database.DB.Model(&models.Order{}).
Where("created_at >= ? AND created_at < ? AND assigned_at IS NOT NULL", from, to).
Select("COALESCE(AVG(EXTRACT(EPOCH FROM (assigned_at - created_at))), NULL) as avg_minutes, COUNT(*) as sample_size").
Scan(&assignRow)
if assignRow.AvgMinutes != nil {
m := *assignRow.AvgMinutes / 60
report.AvgAssignmentMinutes = &m
}
report.AssignmentSampleSize = assignRow.SampleSize

var deliverRow avgRow
database.DB.Model(&models.Order{}).
Where("created_at >= ? AND created_at < ? AND assigned_at IS NOT NULL AND delivered_at IS NOT NULL", from, to).
Select("COALESCE(AVG(EXTRACT(EPOCH FROM (delivered_at - assigned_at))), NULL) as avg_minutes, COUNT(*) as sample_size").
Scan(&deliverRow)
if deliverRow.AvgMinutes != nil {
m := *deliverRow.AvgMinutes / 60
report.AvgDeliveryMinutes = &m
}
report.DeliverySampleSize = deliverRow.SampleSize

// SLA: confirmed (created_at) -> delivered_at, vs sla_target_minutes.
report.SLATargetMinutes = utils.GetSettingFloat("sla_target_minutes", 30.0)

var slaRow avgRow
database.DB.Model(&models.Order{}).
Where("created_at >= ? AND created_at < ? AND delivered_at IS NOT NULL", from, to).
Select("COALESCE(AVG(EXTRACT(EPOCH FROM (delivered_at - created_at))), NULL) as avg_minutes, COUNT(*) as sample_size").
Scan(&slaRow)
if slaRow.AvgMinutes != nil {
m := *slaRow.AvgMinutes / 60
report.AvgConfirmToDeliverMinutes = &m
}
report.SLASampleSize = slaRow.SampleSize

if report.SLASampleSize > 0 {
var onTimeCount int64
database.DB.Model(&models.Order{}).
Where("created_at >= ? AND created_at < ? AND delivered_at IS NOT NULL AND EXTRACT(EPOCH FROM (delivered_at - created_at)) / 60 <= ?", from, to, report.SLATargetMinutes).
Count(&onTimeCount)
adherence := float64(onTimeCount) / float64(report.SLASampleSize) * 100
report.SLAAdherencePercent = &adherence
report.DelayedOrders = report.SLASampleSize - onTimeCount
}

// Picker/packing time - warehouse-wide average across all staff in
// range, reusing the same started_at/completed_at fields already
// proven out in the per-staff performance view.
var pickRow avgRow
database.DB.Model(&models.PickingTask{}).
Where("status = ? AND started_at IS NOT NULL AND completed_at IS NOT NULL AND created_at >= ? AND created_at < ?", "completed", from, to).
Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), NULL) as avg_minutes, COUNT(*) as sample_size").
Scan(&pickRow)
if pickRow.AvgMinutes != nil {
m := *pickRow.AvgMinutes / 60
report.AvgPickingMinutes = &m
}
report.PickingSampleSize = pickRow.SampleSize

var packRow avgRow
database.DB.Model(&models.PackingTask{}).
Where("status = ? AND started_at IS NOT NULL AND completed_at IS NOT NULL AND created_at >= ? AND created_at < ?", "completed", from, to).
Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), NULL) as avg_minutes, COUNT(*) as sample_size").
Scan(&packRow)
if packRow.AvgMinutes != nil {
m := *packRow.AvgMinutes / 60
report.AvgPackingMinutes = &m
}
report.PackingSampleSize = packRow.SampleSize

// Orders/minute over the range.
rangeMinutes := to.Sub(from).Minutes()
if rangeMinutes > 0 && report.TotalOrders > 0 {
opm := float64(report.TotalOrders) / rangeMinutes
report.OrdersPerMinute = &opm
}

// Store availability - current snapshot, not range-scoped (a store's
// status right now is what matters operationally).
database.DB.Model(&models.Warehouse{}).Count(&report.TotalStores)
database.DB.Model(&models.Warehouse{}).Where("status = ?", "open").Count(&report.OpenStores)
database.DB.Model(&models.Warehouse{}).Where("status = ?", "paused").Count(&report.PausedStores)
database.DB.Model(&models.Warehouse{}).Where("status = ?", "closed").Count(&report.ClosedStores)

// Rider availability - current snapshot.
database.DB.Model(&models.DeliveryPartner{}).Where("is_active = ?", true).Count(&report.TotalRiders)
database.DB.Model(&models.DeliveryPartner{}).Where("is_active = ? AND is_online = ?", true, true).Count(&report.OnlineRiders)

c.JSON(http.StatusOK, report)
}
