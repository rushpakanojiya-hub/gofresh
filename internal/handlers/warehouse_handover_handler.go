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

// HandoverOrderRequest is the body for PUT /warehouse/orders/:id/handover
type HandoverOrderRequest struct {
	PackageCount      int  `json:"package_count" binding:"required,min=1"`
	DeliveryPartnerID uint `json:"delivery_partner_id" binding:"required"`
}

// HandoverOrder godoc
// PUT /api/v1/warehouse/orders/:id/handover (warehouse staff only)
// Verifies a ready_for_dispatch order has an assigned delivery partner,
// records the handover, and moves the order to handed_over. This is what
// unblocks the delivery partner's own "mark shipped" action.
func HandoverOrder(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	orderID := c.Param("id")

	var req HandoverOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var order models.Order
	if err := database.DB.Where("warehouse_id = ?", warehouseID).First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found for your warehouse"})
		return
	}

	statusCode := http.StatusInternalServerError
	now := time.Now()
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, order.ID).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("order not found")
		}
		if order.Status != models.OrderStatusReadyForDispatch {
			statusCode = http.StatusBadRequest
			return errors.New("only ready_for_dispatch orders can be handed over, current status: " + order.Status)
		}
		if order.DeliveryPartnerID == nil {
			statusCode = http.StatusBadRequest
			return errors.New("no delivery partner assigned to this order yet")
		}
		if *order.DeliveryPartnerID != req.DeliveryPartnerID {
			statusCode = http.StatusForbidden
			return errors.New("delivery partner does not match the partner assigned to this order")
		}

		// BUG-66: only an assignment the partner has accepted can be handed over.
		var acceptedCnt int64
		if err := tx.Model(&models.Order{}).Where("id = ? AND delivery_assignment_status = ?", order.ID, "accepted").Count(&acceptedCnt).Error; err != nil {
			return err
		}
		if acceptedCnt == 0 {
			statusCode = http.StatusBadRequest
			return errors.New("delivery partner has not accepted this assignment yet")
		}
        // BUG-66b: partner must be checked in at THIS store (distance was verified at check-in).
        var hoPartner models.DeliveryPartner
        if err := tx.Select("id", "checked_in_warehouse_id", "checked_in_at").First(&hoPartner, *order.DeliveryPartnerID).Error; err != nil {
            statusCode = http.StatusNotFound
            return errors.New("delivery partner not found")
        }
        if hoPartner.CheckedInWarehouseID == nil || *hoPartner.CheckedInWarehouseID != warehouseID ||
            hoPartner.CheckedInAt == nil || time.Since(*hoPartner.CheckedInAt) > services.StoreCheckinMaxAge {
            statusCode = http.StatusBadRequest
            return errors.New("delivery partner is not checked in at this store")
        }
		var existing models.OrderHandover
		if err := tx.Where("order_id = ?", order.ID).First(&existing).Error; err == nil {
			statusCode = http.StatusConflict
			return errors.New("this order has already been handed over")
		}
		handover := models.OrderHandover{
			OrderID:           order.ID,
			WarehouseID:       warehouseID,
			WarehouseStaffID:  staffID,
			DeliveryPartnerID: *order.DeliveryPartnerID,
			PackageCount:      req.PackageCount,
			HandedOverAt:      now,
		}
		if err := tx.Create(&handover).Error; err != nil {
			return err
		}
		order.Status = models.OrderStatusHandedOver
		return tx.Save(&order).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "handover_order", "order", orderID,
		"status=ready_for_dispatch", "status=handed_over")

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": order.ID, "status": order.Status})
}

// GetHandover godoc
// GET /api/v1/warehouse/orders/:id/handover (warehouse staff only)
func GetHandover(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	orderID := c.Param("id")

	var handover models.OrderHandover
	if err := database.DB.Where("order_id = ? AND warehouse_id = ?", orderID, warehouseID).First(&handover).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No handover record for this order"})
		return
	}
	c.JSON(http.StatusOK, handover)
}
