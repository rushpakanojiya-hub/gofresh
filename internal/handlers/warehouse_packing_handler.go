package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetPackingTask godoc
// GET /api/v1/warehouse/packing/:order_id (warehouse staff only)
func GetPackingTask(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	orderID := c.Param("id")

	var task models.PackingTask
	if err := database.DB.Where("order_id = ? AND warehouse_id = ?", orderID, warehouseID).
		Preload("Order.Address").First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Packing task not found for your warehouse"})
		return
	}

	// Include the picking outcome so the packer can see what was actually
	// picked (including any short/unavailable items) before packing.
	var pickingTask models.PickingTask
	database.DB.Where("order_id = ?", orderID).Preload("Items.Product").First(&pickingTask)

	c.JSON(http.StatusOK, gin.H{
		"packing_task": task,
		"picked_items": pickingTask.Items,
	})
}

// StartPacking godoc
// PUT /api/v1/warehouse/packing/:order_id/start (warehouse staff only)
func StartPacking(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	orderID := c.Param("id")

	var task models.PackingTask
	statusCode := http.StatusInternalServerError

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_id = ? AND warehouse_id = ?", orderID, warehouseID).
			First(&task).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("Packing task not found for your warehouse")
		}
		if task.Status == "completed" {
			statusCode = http.StatusBadRequest
			return errors.New("Packing already completed for this order")
		}
		if task.PackerID != nil && *task.PackerID != staffID {
			statusCode = http.StatusConflict
			return errors.New("This order is already being packed by another staff member")
		}

		// Move the order into the "packing" state so the lifecycle reflects
		// picked -> packing -> packed -> ready_for_dispatch, rather than
		// jumping from picked straight to ready_for_dispatch on completion.
		if task.Status != "in_progress" {
			var order models.Order
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND warehouse_id = ?", orderID, warehouseID).First(&order).Error; err != nil {
				statusCode = http.StatusNotFound
				return errors.New("Order not found for your warehouse")
			}
			if order.Status != models.OrderStatusPicked {
				statusCode = http.StatusBadRequest
				return errors.New("only picked orders can be packed, current status: " + order.Status)
			}
			if err := tx.Model(&models.Order{}).Where("id = ?", orderID).
				Update("status", models.OrderStatusPacking).Error; err != nil {
				return err
			}
		}

		now := time.Now()
		task.PackerID = &staffID
		task.Status = "in_progress"
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
		return tx.Save(&task).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// CompletePacking godoc
// PUT /api/v1/warehouse/packing/:order_id/complete (warehouse staff only)
// Marks the order packed and ready for dispatch. Prevents double-packing by
// checking task.Status == "completed" up front (idempotency guard).
func CompletePacking(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	orderID := c.Param("id")

	var task models.PackingTask
	statusCode := http.StatusInternalServerError

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_id = ? AND warehouse_id = ?", orderID, warehouseID).
			First(&task).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("Packing task not found for your warehouse")
		}
		if task.Status == "completed" {
			statusCode = http.StatusConflict
			return errors.New("This order has already been packed")
		}
		if task.Status != "in_progress" {
			statusCode = http.StatusBadRequest
			return errors.New("Packing must be started before it can be completed")
		}

		now := time.Now()
		task.Status = "completed"
		task.CompletedAt = &now
		if err := tx.Save(&task).Error; err != nil {
			return err
		}

		// Pass through "packed" before "ready_for_dispatch" so the order
		// lifecycle records the distinct milestone, matching
		// picked -> packing -> packed -> ready_for_dispatch.
		if err := tx.Model(&models.Order{}).Where("id = ?", orderID).Update("status", models.OrderStatusPacked).Error; err != nil {
			return err
		}
		return tx.Model(&models.Order{}).Where("id = ?", orderID).Update("status", models.OrderStatusReadyForDispatch).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "complete_packing", "order", orderID,
		"status=packing", "status=ready_for_dispatch")

	// Fire-and-forget: as soon as the order is ready_for_dispatch, try to
	// auto-assign the nearest available delivery partner instead of waiting
	// for an admin to manually assign one (this was the missing Zepto-style
	// instant-assignment step).
	go services.AutoAssignDeliveryPartner(task.OrderID)

	services.NotifyWarehouse(warehouseID, models.WhNotifyHandoverRequired,
		"Order #"+orderID+" ready for handover",
		"Packing complete - this order is ready to hand over to a delivery partner.", &task.OrderID, nil)

	c.JSON(http.StatusOK, gin.H{"success": true, "packing_task": task, "order_status": models.OrderStatusReadyForDispatch})
}
