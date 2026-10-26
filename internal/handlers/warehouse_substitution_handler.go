package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateSubstitutionRequest godoc
// POST /api/v1/warehouse/substitutions (warehouse/store staff - picker/packer)
// A picker/packer requests to swap the original product for a substitute
// when the original is unavailable or short during picking/packing.
func CreateSubstitutionRequest(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)

	var req models.CreateSubstitutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Make sure the order actually belongs to this warehouse.
	var order models.Order
	if err := database.DB.Where("id = ? AND warehouse_id = ?", req.OrderID, warehouseID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found for your warehouse"})
		return
	}

	sub := models.SubstitutionRequest{
		OrderID:             req.OrderID,
		PickingTaskItemID:   req.PickingTaskItemID,
		OriginalProductID:   req.OriginalProductID,
		SubstituteProductID: req.SubstituteProductID,
		Quantity:            req.Quantity,
		Reason:              req.Reason,
		WarehouseID:         warehouseID,
		RequestedByID:       staffID,
		Status:              models.SubstitutionStatusPending,
	}

	if err := database.DB.Create(&sub).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create substitution request"})
		return
	}

	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "create_substitution", "substitution_request",
		strconv.Itoa(int(sub.ID)), "", "status=pending")

	database.DB.Preload("Order").Preload("OriginalProduct").Preload("SubstituteProduct").First(&sub, sub.ID)
	c.JSON(http.StatusCreated, sub)
}

// GetSubstitutionRequests godoc
// GET /api/v1/warehouse/substitutions?status=&order_id=&page=&limit= (warehouse/store staff)
func GetSubstitutionRequests(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)

	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	db := database.DB.Model(&models.SubstitutionRequest{}).Where("warehouse_id = ?", warehouseID)
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	if orderID := c.Query("order_id"); orderID != "" {
		db = db.Where("order_id = ?", orderID)
	}

	var total int64
	db.Count(&total)

	var subs []models.SubstitutionRequest
	offset := (page - 1) * limit
	if err := db.Preload("Order").Preload("OriginalProduct").Preload("SubstituteProduct").
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&subs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch substitution requests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"substitution_requests": subs,
		"page":                  page,
		"limit":                 limit,
		"total":                 total,
		"total_pages":           int((total + int64(limit) - 1) / int64(limit)),
	})
}

// GetSubstitutionRequest godoc
// GET /api/v1/warehouse/substitutions/:id (warehouse/store staff)
func GetSubstitutionRequest(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	id := c.Param("id")

	var sub models.SubstitutionRequest
	if err := database.DB.Where("id = ? AND warehouse_id = ?", id, warehouseID).
		Preload("Order").Preload("OriginalProduct").Preload("SubstituteProduct").First(&sub).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Substitution request not found for your warehouse"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

// ApproveSubstitutionRequest godoc
// PUT /api/v1/warehouse/substitutions/:id/approve (warehouse_manager only - enforce via route middleware)
func ApproveSubstitutionRequest(c *gin.Context) {
	decideSubstitution(c, models.SubstitutionStatusApproved, "approve_substitution")
}

// RejectSubstitutionRequest godoc
// PUT /api/v1/warehouse/substitutions/:id/reject (warehouse_manager only - enforce via route middleware)
func RejectSubstitutionRequest(c *gin.Context) {
	decideSubstitution(c, models.SubstitutionStatusRejected, "reject_substitution")
}

func decideSubstitution(c *gin.Context, newStatus string, auditAction string) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	id := c.Param("id")

	var req models.DecideSubstitutionRequest
	// Note field is optional, so ignore a bind error on an empty body.
	_ = c.ShouldBindJSON(&req)

	var sub models.SubstitutionRequest
	var previousStatus string
	var refundAmount float64
	statusCode := http.StatusInternalServerError

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", id, warehouseID).First(&sub).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("Substitution request not found for your warehouse")
		}
		if sub.Status != models.SubstitutionStatusPending {
			statusCode = http.StatusConflict
			return errors.New("Substitution request has already been decided")
		}
		previousStatus = sub.Status

		if newStatus == models.SubstitutionStatusApproved {
			var order models.Order
			if err := tx.First(&order, sub.OrderID).Error; err != nil {
				return err
			}

			var orderItem models.OrderItem
			if err := tx.Where("order_id = ? AND product_id = ?", sub.OrderID, sub.OriginalProductID).First(&orderItem).Error; err != nil {
				statusCode = http.StatusBadRequest
				return errors.New("Original product not found on this order")
			}

            // BUG-57: cannot substitute more than ordered, and not after picking is completed
            // (CompletePicking already restocked and refunded the unpicked original item).
            if sub.Quantity > orderItem.Quantity {
                statusCode = http.StatusBadRequest
                return errors.New("Substitution quantity exceeds the quantity ordered for this item")
            }
            if sub.PickingTaskItemID != nil {
                var subPickItem models.PickingTaskItem
                if perr := tx.First(&subPickItem, *sub.PickingTaskItemID).Error; perr == nil {
                    var subPickTask models.PickingTask
                    if terr := tx.First(&subPickTask, subPickItem.PickingTaskID).Error; terr == nil && subPickTask.Status == "completed" {
                        statusCode = http.StatusConflict
                        return errors.New("Picking is already completed for this order; the original item was already restocked and refunded")
                    }
                }
            }

			var substituteProduct models.Product
			if err := tx.First(&substituteProduct, sub.SubstituteProductID).Error; err != nil {
				return err
			}

			// Deduct the substitute product's stock at this warehouse.
			var subInventory models.Inventory
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("product_id = ? AND warehouse_id = ?", sub.SubstituteProductID, warehouseID).First(&subInventory).Error; err != nil {
				statusCode = http.StatusBadRequest
				return errors.New("Substitute product has no inventory record at this warehouse")
			}
			if subInventory.Stock < sub.Quantity {
				statusCode = http.StatusBadRequest
				return errors.New("Insufficient stock of substitute product to approve this request")
			}
			subPreviousQty := subInventory.Stock
			subInventory.Stock -= sub.Quantity
			if subInventory.Stock <= 0 {
				subInventory.InStock = false
			}
			if err := tx.Save(&subInventory).Error; err != nil {
				return err
			}
			subMovement := models.StockMovement{
				ProductID:    sub.SubstituteProductID,
				WarehouseID:  warehouseID,
				PreviousQty:  subPreviousQty,
				Change:       -sub.Quantity,
				NewQty:       subInventory.Stock,
				MovementType: models.MovementSale,
				Reason:       "Substitution approved",
				ReferenceID:  &sub.OrderID,
			}
			if err := tx.Create(&subMovement).Error; err != nil {
				return err
			}

			// Restore the original product's stock at this warehouse (it was
			// deducted at checkout but never actually picked/shipped).
			var origInventory models.Inventory
			q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ? AND warehouse_id = ?", sub.OriginalProductID, warehouseID)
			if err := q.First(&origInventory).Error; err == nil {
				origPreviousQty := origInventory.Stock
				origInventory.Stock += sub.Quantity
				origInventory.InStock = true
				if err := tx.Save(&origInventory).Error; err != nil {
					return err
				}
				origMovement := models.StockMovement{
					ProductID:    sub.OriginalProductID,
					WarehouseID:  warehouseID,
					PreviousQty:  origPreviousQty,
					Change:       sub.Quantity,
					NewQty:       origInventory.Stock,
					MovementType: models.MovementReturn,
					Reason:       "Substitution approved - original item not picked",
					ReferenceID:  &sub.OrderID,
				}
				if err := tx.Create(&origMovement).Error; err != nil {
					return err
				}
			}

			// Swap the order item onto the substitute product. If the substitute
			// is cheaper, refund the difference to the customer's wallet; if it's
			// pricier, absorb the difference rather than charging the customer
			// more than they agreed to pay.
			priceDiff := (orderItem.Price - substituteProduct.Price) * float64(sub.Quantity)
			originalPrice := orderItem.Price
			if sub.Quantity >= orderItem.Quantity {
				// Full-line substitution: every unit on this order line is being
				// substituted, so it's safe to swap the existing row in place.
				orderItem.ProductID = sub.SubstituteProductID
				if substituteProduct.Price < originalPrice {
					orderItem.Price = substituteProduct.Price
				}
				if err := tx.Save(&orderItem).Error; err != nil {
					return err
				}
			} else {
				// Partial substitution: only sub.Quantity of the line's units are
				// being substituted. Shrink the original line and add a separate
				// order item for the substituted units, instead of converting the
				// whole line (which would corrupt quantity/inventory accounting).
				orderItem.Quantity -= sub.Quantity
				if err := tx.Save(&orderItem).Error; err != nil {
					return err
				}
				substitutePrice := originalPrice
				if substituteProduct.Price < originalPrice {
					substitutePrice = substituteProduct.Price
				}
				newItem := models.OrderItem{
					OrderID:   order.ID,
					ProductID: sub.SubstituteProductID,
					Quantity:  sub.Quantity,
					Price:     substitutePrice,
				}
				if err := tx.Create(&newItem).Error; err != nil {
					return err
				}
			}

			if priceDiff > 0 {
				// BUG-55: coupon-adjusted refund; unpaid COD cash is never credited to the wallet.
				subRatio := 1.0
				if order.ItemsAmount > 0 {
					var subCoupon models.OrderCoupon
					if cerr := tx.Where("order_id = ?", order.ID).First(&subCoupon).Error; cerr == nil {
						eff := order.ItemsAmount - subCoupon.DiscountAmount
						if eff < 0 {
							eff = 0
						}
						subRatio = eff / order.ItemsAmount
					}
				}
				refundAmount = priceDiff * subRatio
				walletCredit := refundAmount
				if order.PaymentMethod == models.PaymentMethodCOD {
					walletCredit = refundAmount - order.TotalAmount
					if walletCredit < 0 {
						walletCredit = 0
					}
					if walletCredit > order.WalletAmountUsed {
						walletCredit = order.WalletAmountUsed
					}
				}
				newItemsAmount := order.ItemsAmount - priceDiff
				newTotalAmount := order.TotalAmount - refundAmount
				if newItemsAmount < 0 {
					newItemsAmount = 0
				}
				if newTotalAmount < 0 {
					newTotalAmount = 0
				}
				if err := tx.Model(&models.Order{}).Where("id = ?", order.ID).
					Updates(map[string]interface{}{"items_amount": newItemsAmount, "total_amount": newTotalAmount}).Error; err != nil {
					return err
				}
				refID := order.ID
				if walletCredit > 0 {
					if err := utils.CreditWallet(tx, order.UserID, walletCredit, models.WalletReasonOrderRefund, "order", &refID, "Refund for cheaper substitute item"); err != nil {
						return err
					}
				}
			}

			// Reflect the swap on the picking task item, if this substitution was
			// raised against one.
			if sub.PickingTaskItemID != nil {
				if err := tx.Model(&models.PickingTaskItem{}).Where("id = ?", *sub.PickingTaskItemID).
					Updates(map[string]interface{}{"product_id": sub.SubstituteProductID, "status": models.PickItemPicked, "quantity_picked": sub.Quantity}).Error; err != nil {
					return err
				}
			}
		}

		sub.Status = newStatus
		sub.DecisionNote = req.Note
		staffIDCopy := staffID
		sub.DecidedByID = &staffIDCopy
		now := time.Now()
		sub.DecidedAt = &now
		return tx.Save(&sub).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), auditAction, "substitution_request",
		strconv.Itoa(int(sub.ID)), "status="+previousStatus, "status="+sub.Status)

	database.DB.Preload("Order").Preload("OriginalProduct").Preload("SubstituteProduct").First(&sub, sub.ID)
	c.JSON(http.StatusOK, gin.H{"substitution_request": sub, "refund_amount": refundAmount})
}
