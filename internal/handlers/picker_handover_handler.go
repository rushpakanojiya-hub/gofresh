package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PickerHandover POST /warehouse/picker/orders/:id/handover {"package_count":1}
// Call after PUT /warehouse/picking/:id/complete. Finishes packing on behalf of
// the picker, makes sure a delivery partner is assigned, records the handover
// and moves the order to handed_over (which unlocks the partner's "Order picked").
func PickerHandover(c *gin.Context) {
	database.EnsurePickerTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order id"})
		return
	}
	var req struct {
		PackageCount int `json:"package_count"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.PackageCount < 1 {
		req.PackageCount = 1
	}

	var task models.PickingTask
	if err := database.DB.Where("order_id = ? AND picker_id = ?", orderID, staffID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Picking task not found for you"})
		return
	}
	if task.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Complete picking first"})
		return
	}

	statusCode := http.StatusInternalServerError
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, orderID).Error; e != nil {
			statusCode = http.StatusNotFound
			return errors.New("order not found")
		}
		if order.WarehouseID == nil || *order.WarehouseID != task.WarehouseID {
			statusCode = http.StatusForbidden
			return errors.New("order does not belong to your store")
		}
		switch order.Status {
		case models.OrderStatusPicked, models.OrderStatusPacking, models.OrderStatusPacked:
			now := time.Now()
			var pt models.PackingTask
			if e := tx.Where("order_id = ?", order.ID).First(&pt).Error; e == nil && pt.Status != "completed" {
				if pt.StartedAt == nil {
					pt.StartedAt = &now
				}
				pt.PackerID = &staffID
				pt.Status = "completed"
				pt.CompletedAt = &now
				if e := tx.Save(&pt).Error; e != nil {
					return e
				}
			}
			return tx.Model(&models.Order{}).Where("id = ?", order.ID).
				Update("status", models.OrderStatusReadyForDispatch).Error
		case models.OrderStatusReadyForDispatch, models.OrderStatusHandedOver,
			models.OrderStatusShipped, models.OrderStatusDelivered:
			return nil
		default:
			statusCode = http.StatusBadRequest
			return errors.New("order cannot be handed over, status: " + order.Status)
		}
	})
	if err != nil {
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	var order models.Order
	database.DB.First(&order, orderID)
	if order.Status == models.OrderStatusReadyForDispatch && order.DeliveryPartnerID == nil {
		services.AutoAssignDeliveryPartner(order.ID)
		database.DB.First(&order, orderID)
	}

	handedOver := false
	if order.Status == models.OrderStatusReadyForDispatch {
		if order.DeliveryPartnerID == nil {
			c.JSON(http.StatusConflict, gin.H{
				"error":               "Waiting for a delivery partner to be assigned",
				"waiting_for_partner": true,
				"status":              order.Status,
			})
			return
		}
		statusCode = http.StatusInternalServerError
		err = database.DB.Transaction(func(tx *gorm.DB) error {
			var o models.Order
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&o, orderID).Error; e != nil {
				return e
			}
			if o.Status != models.OrderStatusReadyForDispatch {
				return nil
			}
			if o.DeliveryPartnerID == nil {
				statusCode = http.StatusConflict
				return errors.New("Waiting for a delivery partner to be assigned")
			}
			var existing models.OrderHandover
			if e := tx.Where("order_id = ?", o.ID).First(&existing).Error; e != nil {
				h := models.OrderHandover{
					OrderID:           o.ID,
					WarehouseID:       task.WarehouseID,
					WarehouseStaffID:  staffID,
					DeliveryPartnerID: *o.DeliveryPartnerID,
					PackageCount:      req.PackageCount,
					HandedOverAt:      time.Now(),
				}
				if e := tx.Create(&h).Error; e != nil {
					return e
				}
			}
			if e := tx.Model(&models.Order{}).Where("id = ?", o.ID).
				Update("status", models.OrderStatusHandedOver).Error; e != nil {
				return e
			}
			handedOver = true
			return nil
		})
		if err != nil {
			c.JSON(statusCode, gin.H{"error": err.Error(), "waiting_for_partner": statusCode == http.StatusConflict})
			return
		}
		database.DB.First(&order, orderID)
	}

	if handedOver {
		var st models.WarehouseStaff
		database.DB.Select("id", "name").First(&st, staffID)
		services.LogWarehouseAction(task.WarehouseID, staffID, st.Name, "handover_order", "order",
			strconv.FormatUint(orderID, 10), "status=ready_for_dispatch", "status=handed_over")
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": order.ID, "status": order.Status})
}
