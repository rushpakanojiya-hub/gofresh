package handlers

import (
	"gorm.io/gorm/clause"
	"gorm.io/gorm"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ---------------------------------------------------------------------------
// Purchase Orders (admin only)
// ---------------------------------------------------------------------------

var validPOStatuses = map[string]bool{
	"draft": true, "sent": true, "confirmed": true,
	"partially_received": true, "received": true, "cancelled": true,
}

func generatePONumber() string {
	var seqVal int64
	if err := database.DB.Raw("SELECT nextval('public.po_number_seq')").Scan(&seqVal).Error; err == nil && seqVal > 0 {
		return fmt.Sprintf("PO-%06d", seqVal)
	}
	var count int64
	database.DB.Model(&models.PurchaseOrder{}).Count(&count)
	return fmt.Sprintf("PO-%06d", count+1)
}

func ListPurchaseOrders(c *gin.Context) {
	db := database.DB.Preload("Vendor").Preload("Warehouse").Preload("Items").Preload("Items.Product").Order("created_at DESC")

	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	if vendorID := c.Query("vendor_id"); vendorID != "" {
		db = db.Where("vendor_id = ?", vendorID)
	}
	if warehouseID := c.Query("warehouse_id"); warehouseID != "" {
		db = db.Where("warehouse_id = ?", warehouseID)
	}

	var orders []models.PurchaseOrder
	if err := db.Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load purchase orders"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"purchase_orders": orders})
}

func GetPurchaseOrder(c *gin.Context) {
	id := c.Param("id")

	var po models.PurchaseOrder
	if err := database.DB.Preload("Vendor").Preload("Warehouse").
		Preload("Items").Preload("Items.Product").First(&po, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase order not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"purchase_order": po})
}

func CreatePurchaseOrder(c *gin.Context) {
	var req models.PurchaseOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.MustGet("user_id").(uint)

	var expectedDate *time.Time
	if req.ExpectedDate != "" {
		if t, err := time.Parse("2006-01-02", req.ExpectedDate); err == nil {
			expectedDate = &t
		}
	}

	var total float64
	items := make([]models.PurchaseOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, models.PurchaseOrderItem{
			ProductID:       it.ProductID,
			QuantityOrdered: it.QuantityOrdered,
			UnitPrice:       it.UnitPrice,
		})
		total += it.UnitPrice * float64(it.QuantityOrdered)
	}

	po := models.PurchaseOrder{
		PONumber:     generatePONumber(),
		VendorID:     req.VendorID,
		WarehouseID:  req.WarehouseID,
		Status:       "draft",
		ExpectedDate: expectedDate,
		Notes:        req.Notes,
		TotalAmount:  total,
		CreatedByID:  adminID,
		Items:        items,
	}

	if err := database.DB.Create(&po).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create purchase order"})
		return
	}

	database.DB.Preload("Vendor").Preload("Warehouse").Preload("Items").Preload("Items.Product").First(&po, po.ID)
	c.JSON(http.StatusCreated, gin.H{"purchase_order": po})
}

// BUG-11: allowed manual status transitions. received / partially_received are
// deliberately absent as targets: they are only set by ReceivePurchaseOrderItems,
// which also adds stock. Received, partially received and cancelled orders are
// terminal here, so stock that was already received is never left behind a
// cancelled or draft PO.
var allowedPOTransitions = map[string]map[string]bool{
    "draft":     {"sent": true, "confirmed": true, "cancelled": true},
    "sent":      {"confirmed": true, "cancelled": true},
    "confirmed": {"cancelled": true},
}

func poTransitionAllowed(from, to string) bool {
    return allowedPOTransitions[from][to]
}

func UpdatePurchaseOrderStatus(c *gin.Context) {
    id := c.Param("id")

    var req models.PurchaseOrderStatusRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if !validPOStatuses[req.Status] {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
        return
    }
    if req.Status == "received" || req.Status == "partially_received" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "received and partially_received are set only by receiving items against the purchase order"})
        return
    }

    adminID := c.MustGet("user_id").(uint)
    statusCode := http.StatusInternalServerError
    var po models.PurchaseOrder
    txErr := database.DB.Transaction(func(tx *gorm.DB) error {
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&po, id).Error; err != nil {
            statusCode = http.StatusNotFound
            return errors.New("Purchase order not found")
        }
        if !poTransitionAllowed(po.Status, req.Status) {
            statusCode = http.StatusConflict
            return fmt.Errorf("cannot change purchase order status from %s to %s", po.Status, req.Status)
        }

        wasDraft := po.Status == "draft"
        po.Status = req.Status
        if wasDraft && req.Status != "cancelled" {
            now := time.Now()
            po.ApprovedByID = &adminID
            po.ApprovedAt = &now
        }
        if req.Status == "cancelled" && req.Reason != "" {
            po.CancelledReason = req.Reason
        }
        return tx.Save(&po).Error
    })
    if txErr != nil {
        c.JSON(statusCode, gin.H{"error": txErr.Error()})
        return
    }

    adminPhone, _ := c.Get("phone")
    utils.LogAudit(adminID, fmt.Sprint(adminPhone), "update_po_status", "purchase_order", id, "status: "+req.Status)

    c.JSON(http.StatusOK, gin.H{"purchase_order": po})
}

// BUG-58: whole receipt runs in one transaction with row locks, every DB error
// is checked, and over-receipt is rejected instead of silently clamped.
func ReceivePurchaseOrderItems(c *gin.Context) {
    id := c.Param("id")

    var req models.ReceivePurchaseOrderItemsRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    statusCode := http.StatusInternalServerError
    var poID uint
    txErr := database.DB.Transaction(func(tx *gorm.DB) error {
        var po models.PurchaseOrder
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&po, id).Error; err != nil {
            statusCode = http.StatusNotFound
            return errors.New("Purchase order not found")
        }
        poID = po.ID
        if po.Status == "cancelled" || po.Status == "received" {
            statusCode = http.StatusBadRequest
            return errors.New("Cannot receive against a " + po.Status + " purchase order")
        }

        var items []models.PurchaseOrderItem
        if err := tx.Where("purchase_order_id = ?", po.ID).Find(&items).Error; err != nil {
            return err
        }
        idx := make(map[uint]int, len(items))
        for i := range items {
            idx[items[i].ID] = i
        }

        for _, r := range req.Items {
            i, ok := idx[r.ItemID]
            if !ok {
                statusCode = http.StatusBadRequest
                return fmt.Errorf("item %d does not belong to this purchase order", r.ItemID)
            }
            item := &items[i]
            qty := r.QuantityReceived
            if qty <= 0 {
                continue
            }
            if qty > item.QuantityOrdered-item.QuantityReceived {
                statusCode = http.StatusBadRequest
                return fmt.Errorf("item %d: received quantity exceeds remaining quantity", r.ItemID)
            }

            item.QuantityReceived += qty
            if err := tx.Save(item).Error; err != nil {
                return err
            }

            var inv models.Inventory
            err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
                Where("product_id = ? AND warehouse_id = ?", item.ProductID, po.WarehouseID).First(&inv).Error
            previousQty := 0
            if err == nil {
                previousQty = inv.Stock
                inv.Stock += qty
                inv.InStock = true
                if err := tx.Save(&inv).Error; err != nil {
                    return err
                }
            } else if errors.Is(err, gorm.ErrRecordNotFound) {
                inv = models.Inventory{ProductID: item.ProductID, WarehouseID: po.WarehouseID, Stock: qty, InStock: true}
                if err := tx.Create(&inv).Error; err != nil {
                    return err
                }
            } else {
                return err
            }
            if err := tx.Create(&models.StockMovement{
                ProductID:    item.ProductID,
                WarehouseID:  po.WarehouseID,
                PreviousQty:  previousQty,
                Change:       qty,
                NewQty:       inv.Stock,
                MovementType: models.MovementReceive,
                Reason:       fmt.Sprintf("PO #%s receipt", po.PONumber),
                ReferenceID:  &po.ID,
            }).Error; err != nil {
                return err
            }
        }

        allReceived := true
        anyReceived := false
        for _, it := range items {
            if it.QuantityReceived > 0 {
                anyReceived = true
            }
            if it.QuantityReceived < it.QuantityOrdered {
                allReceived = false
            }
        }
        newStatus := po.Status
        if allReceived {
            newStatus = "received"
        } else if anyReceived {
            newStatus = "partially_received"
        }
        return tx.Model(&models.PurchaseOrder{}).Where("id = ?", po.ID).Update("status", newStatus).Error
    })
    if txErr != nil {
        c.JSON(statusCode, gin.H{"error": txErr.Error()})
        return
    }

    var po models.PurchaseOrder
    database.DB.Preload("Vendor").Preload("Warehouse").Preload("Items").Preload("Items.Product").First(&po, poID)
    c.JSON(http.StatusOK, gin.H{"purchase_order": po})
}

func GetReplenishmentSuggestions(c *gin.Context) {
	db := database.DB.Table("inventories").
		Select(`inventories.product_id, products.name as product_name,
inventories.warehouse_id, warehouses.name as warehouse_name,
inventories.stock as current_stock,
CASE WHEN products.reorder_level > 0 THEN products.reorder_level ELSE ? END as reorder_level`,
			lowStockThreshold).
		Joins("JOIN products ON products.id = inventories.product_id").
		Joins("JOIN warehouses ON warehouses.id = inventories.warehouse_id").
		Where(`inventories.stock <= CASE WHEN products.reorder_level > 0 THEN products.reorder_level ELSE ? END`,
			lowStockThreshold)

	if warehouseID := c.Query("warehouse_id"); warehouseID != "" {
		db = db.Where("inventories.warehouse_id = ?", warehouseID)
	}

	var rows []models.ReplenishmentSuggestion
	if err := db.Order("inventories.stock ASC").Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load replenishment suggestions"})
		return
	}

	for i := range rows {
		target := rows[i].ReorderLevel * 2
		if target <= rows[i].CurrentStock {
			target = rows[i].ReorderLevel + 10
		}
		rows[i].SuggestedOrder = target - rows[i].CurrentStock
		if rows[i].SuggestedOrder < 1 {
			rows[i].SuggestedOrder = 1
		}
	}

	c.JSON(http.StatusOK, gin.H{"suggestions": rows})
}

func UpdateProductReorderLevel(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		ReorderLevel int `json:"reorder_level" binding:"required,min=0"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Model(&models.Product{}).Where("id = ?", id).
		UpdateColumn("reorder_level", body.ReorderLevel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reorder level"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "product_id": id, "reorder_level": body.ReorderLevel})
}
