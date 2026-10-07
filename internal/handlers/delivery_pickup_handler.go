package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PickupOrder: PUT /api/v1/delivery/orders/:id/pickup (delivery partner only)
// The partner swipes "Order picked" at the store. Works only once the store
// has marked the order ready_for_dispatch. Records the handover (no store
// staff confirmation needed), moves the order to handed_over and the
// delivery_status to picked_up.
func PickupOrder(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	orderID, convErr := strconv.ParseUint(c.Param("id"), 10, 64)
	if convErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}

	statusCode := http.StatusInternalServerError
	var order models.Order
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).
			First(&order).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("order not found or not assigned to you")
		}
		cur := ""
		if order.DeliveryStatus != nil {
			cur = *order.DeliveryStatus
		}
		if cur == models.DeliveryStatusPickedUp {
			return nil
		}
		if cur != models.DeliveryStatusAccepted &&
			cur != models.DeliveryStatusGoingToStore &&
			cur != models.DeliveryStatusArrivedAtStore {
			statusCode = http.StatusBadRequest
			return errors.New("accept the order before picking it up")
		}
		if order.Status != models.OrderStatusReadyForDispatch && order.Status != models.OrderStatusHandedOver {
			statusCode = http.StatusBadRequest
			return errors.New("the store has not marked this order ready for dispatch yet")
		}
		if order.WarehouseID == nil {
			statusCode = http.StatusBadRequest
			return errors.New("order has no warehouse")
		}
		var existing models.OrderHandover
		if err := tx.Where("order_id = ?", order.ID).First(&existing).Error; err != nil {
			h := models.OrderHandover{
				OrderID:           order.ID,
				WarehouseID:       *order.WarehouseID,
				WarehouseStaffID:  0,
				DeliveryPartnerID: partnerID,
				PackageCount:      1,
				HandedOverAt:      time.Now(),
			}
			if err := tx.Create(&h).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
			"status":          models.OrderStatusHandedOver,
			"delivery_status": models.DeliveryStatusPickedUp,
		}).Error
	})
	if err != nil {
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	database.DB.Preload("Address").Preload("Items").First(&order, order.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Order picked", "order": toAssignedOrderSummary(order)})
}
