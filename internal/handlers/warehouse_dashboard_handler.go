package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetWarehouseDashboard godoc
// GET /api/v1/warehouse/dashboard (warehouse staff only)
func GetWarehouseDashboard(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	todayStart := opsDayStart()

	var todayOrders, newOrders, picking, packed, readyForDispatch, completed, cancelled int64
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND created_at >= ?", warehouseID, todayStart).Count(&todayOrders)
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND status = ?", warehouseID, models.OrderStatusConfirmed).Count(&newOrders)
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND status IN ?", warehouseID, []string{models.OrderStatusPicking, models.OrderStatusPicked}).Count(&picking)
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND status IN ?", warehouseID, []string{models.OrderStatusPacking, models.OrderStatusPacked}).Count(&packed)
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND status = ?", warehouseID, models.OrderStatusReadyForDispatch).Count(&readyForDispatch)
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND status = ? AND created_at >= ?", warehouseID, models.OrderStatusDelivered, todayStart).Count(&completed)
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND status = ? AND created_at >= ?", warehouseID, models.OrderStatusCancelled, todayStart).Count(&cancelled)

	var lowStock, outOfStock int64
	database.DB.Model(&models.Inventory{}).Where("warehouse_id = ? AND stock > 0 AND stock < ?", warehouseID, lowStockThreshold).Count(&lowStock)
	database.DB.Model(&models.Inventory{}).Where("warehouse_id = ? AND stock <= 0", warehouseID).Count(&outOfStock)

	var pendingTransfers int64
	database.DB.Model(&models.StockTransfer{}).
		Where("(from_warehouse_id = ? OR to_warehouse_id = ?) AND status = ?", warehouseID, warehouseID, "pending").
		Count(&pendingTransfers)

	var pendingReceivings int64
	database.DB.Model(&models.Receiving{}).
		Where("warehouse_id = ? AND status = ?", warehouseID, models.ReceivingStatusPending).
		Count(&pendingReceivings)

	var openExceptions int64
	database.DB.Model(&models.WarehouseException{}).
		Where("warehouse_id = ? AND status IN ?", warehouseID, []string{models.ExceptionStatusOpen, models.ExceptionStatusInvestigating}).
		Count(&openExceptions)

	// Pending handovers = orders already packed and ready to leave the
	// warehouse but not yet physically handed to a delivery partner.
	var pendingHandovers int64
	database.DB.Model(&models.Order{}).Where("warehouse_id = ? AND status = ?", warehouseID, models.OrderStatusReadyForDispatch).Count(&pendingHandovers)

	// Delayed = confirmed but still not accepted into picking after 2 hours,
	// or already in picking/packing/ready_for_dispatch for over 4 hours -
	// a rough SLA breach signal since there's no explicit expected-ship-by field.
	delayedCutoffNew := opsNewCutoff()
	delayedCutoffInProgress := opsProgressCutoff()
	var delayedOrders int64
	database.DB.Model(&models.Order{}).Where(
		"warehouse_id = ? AND ((status = ? AND created_at < ?) OR (status IN ? AND updated_at < ?))",
		warehouseID, models.OrderStatusConfirmed, delayedCutoffNew,
		[]string{models.OrderStatusPicking, models.OrderStatusPicked, models.OrderStatusPacking, models.OrderStatusPacked, models.OrderStatusReadyForDispatch}, delayedCutoffInProgress,
	).Count(&delayedOrders)

	var expiringBatches int64
	expiryCutoff := time.Now().AddDate(0, 0, 7)
	database.DB.Model(&models.Batch{}).
		Where("warehouse_id = ? AND quantity > 0 AND expiry_date <= ?", warehouseID, expiryCutoff).
		Count(&expiringBatches)

	var activeStaff int64
	database.DB.Model(&models.WarehouseStaff{}).Where("warehouse_id = ? AND is_active = ?", warehouseID, true).Count(&activeStaff)

	// Average picking time (minutes) for tasks completed today.
	var avgPickingSeconds float64
	database.DB.Model(&models.PickingTask{}).
		Where("warehouse_id = ? AND status = ? AND completed_at IS NOT NULL AND started_at IS NOT NULL AND completed_at >= ?", warehouseID, "completed", todayStart).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)").
		Scan(&avgPickingSeconds)

	var avgPackingSeconds float64
	database.DB.Model(&models.PackingTask{}).
		Where("warehouse_id = ? AND status = ? AND completed_at IS NOT NULL AND started_at IS NOT NULL AND completed_at >= ?", warehouseID, "completed", todayStart).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)").
		Scan(&avgPackingSeconds)

	var fulfillmentRate float64
	if todayOrders > 0 {
		fulfillmentRate = float64(completed) / float64(todayOrders) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"today_orders":            todayOrders,
		"new_orders":              newOrders,
		"picking":                 picking,
		"packed":                  packed,
		"ready_for_dispatch":      readyForDispatch,
		"completed_today":         completed,
		"cancelled_today":         cancelled,
		"low_stock_products":      lowStock,
		"out_of_stock_products":   outOfStock,
		"pending_stock_transfers": pendingTransfers,
		"pending_receivings":      pendingReceivings,
		"open_exceptions":         openExceptions,
		"pending_handovers":       pendingHandovers,
		"delayed_orders":          delayedOrders,
		"expiring_stock_batches":  expiringBatches,
		"active_staff":            activeStaff,
		"avg_picking_minutes":     avgPickingSeconds / 60,
		"avg_packing_minutes":     avgPackingSeconds / 60,
		"fulfillment_rate":        fulfillmentRate,
		"items_per_hour":          opsItemsPerHour(warehouseID, 0, todayStart),
		"online_pickers":          opsOnlinePickers(warehouseID),
"pending_putaway": opsPendingPutaway(warehouseID),
	})
}
