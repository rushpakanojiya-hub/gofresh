package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetWarehouseOrders godoc
// GET /api/v1/warehouse/orders (warehouse staff only)
// Lists orders for the staff member's own warehouse, filterable by status.
func GetWarehouseOrders(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)

	status := c.Query("status")
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		if v, err := parsePositiveInt(p); err == nil {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := parsePositiveInt(l); err == nil && v <= 100 {
			limit = v
		}
	}

	db := database.DB.Model(&models.Order{}).Where("warehouse_id = ?", warehouseID)
	if status != "" {
		db = db.Where("status = ?", status)
	}

	var total int64
	db.Count(&total)

	var orders []models.Order
	offset := (page - 1) * limit
	if err := db.Preload("Items.Product").Preload("Address").Preload("DeliveryPartner").
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": opsEnrichOrders(warehouseID, orders),
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int((total + int64(limit) - 1) / int64(limit)),
	})
}

// AcceptOrder godoc
// PUT /api/v1/warehouse/orders/:id/accept (warehouse staff only)
// Moves a confirmed order into picking and creates its PickingTask.
// This is the entry point into the warehouse fulfillment workflow.
func AcceptOrder(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	orderID := c.Param("id")
	staffID := c.MustGet("staff_id").(uint)
	staffName, _ := c.Get("staff_name")

	var order models.Order
	if err := database.DB.Where("warehouse_id = ?", warehouseID).First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found for your warehouse"})
		return
	}

	statusCode := http.StatusInternalServerError
	var orderItems []models.OrderItem
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, order.ID).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("order not found")
		}
		if order.Status != models.OrderStatusConfirmed {
			statusCode = http.StatusBadRequest
			return errors.New("only confirmed orders can be accepted into picking, current status: " + order.Status)
		}
		if err := tx.Where("order_id = ?", order.ID).Find(&orderItems).Error; err != nil {
			return err
		}
		if len(orderItems) == 0 {
			statusCode = http.StatusBadRequest
			return errors.New("order has no items")
		}
		task := models.PickingTask{
			OrderID:     order.ID,
			WarehouseID: warehouseID,
			Status:      "pending",
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		for _, item := range orderItems {
			taskItem := models.PickingTaskItem{
				PickingTaskID:  task.ID,
				OrderItemID:    item.ID,
				ProductID:      item.ProductID,
				QuantityNeeded: item.Quantity,
				Status:         models.PickItemPending,
			}
			if err := tx.Create(&taskItem).Error; err != nil {
				return err
			}
		}
		order.Status = models.OrderStatusPicking
		return tx.Save(&order).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "accept_order", "order", orderID,
		"status=confirmed", "status="+order.Status)

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": order.ID, "status": order.Status})
}

func parsePositiveInt(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return 0, gorm.ErrInvalidData
	}
	return v, nil
}
