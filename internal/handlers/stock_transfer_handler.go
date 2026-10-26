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

// ---------------------------------------------------------------------------
// Stock Transfers - Warehouse Staff side
// ---------------------------------------------------------------------------

// RequestStockTransfer godoc
// POST /api/v1/warehouse/stock-transfers (warehouse staff only)
// The requesting staff member's own warehouse is always the source.
func RequestStockTransfer(c *gin.Context) {
	staffID := c.MustGet("user_id").(uint)

	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
		return
	}

	var req models.StockTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ToWarehouseID == staff.WarehouseID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Source and destination warehouse cannot be the same"})
		return
	}

	var destWarehouse models.Warehouse
	if err := database.DB.First(&destWarehouse, req.ToWarehouseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplying warehouse not found"})
		return
	}

	var product models.Product
	if err := database.DB.First(&product, req.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	transfer := models.StockTransfer{
		ProductID:       req.ProductID,
		FromWarehouseID: req.ToWarehouseID,
		ToWarehouseID:   staff.WarehouseID,
		Quantity:        req.Quantity,
		Status:          models.StockTransferPending,
		RequestedBy:     staffID,
	}

	if err := database.DB.Create(&transfer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create stock transfer request"})
		return
	}

	services.LogWarehouseAction(staff.WarehouseID, staffID, staff.Name, "request_stock_transfer", "stock_transfer",
		fmt.Sprint(transfer.ID), "", fmt.Sprintf("qty=%d to_warehouse=%d", transfer.Quantity, transfer.ToWarehouseID))
	services.NotifyWarehouse(req.ToWarehouseID, models.WhNotifyStockTransfer,
		"Incoming stock transfer request",
		fmt.Sprintf("Warehouse #%d requested a transfer of %d units - awaiting your approval.", staff.WarehouseID, transfer.Quantity),
		nil, &req.ProductID)

	c.JSON(http.StatusCreated, gin.H{"stock_transfer": transfer})
}

// GetMyStockTransfers godoc
// GET /api/v1/warehouse/stock-transfers?status=&page=&limit= (warehouse staff only)
// Returns transfers where the staff's warehouse is either the source or destination.
func GetMyStockTransfers(c *gin.Context) {
	staffID := c.MustGet("user_id").(uint)

	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
		return
	}

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

	db := database.DB.Model(&models.StockTransfer{}).
		Where("from_warehouse_id = ? OR to_warehouse_id = ?", staff.WarehouseID, staff.WarehouseID)
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}

	var total int64
	db.Count(&total)

	var transfers []models.StockTransfer
	offset := (page - 1) * limit
	if err := db.
		Preload("Product").Preload("FromWarehouse").Preload("ToWarehouse").
		Order("created_at DESC").Offset(offset).Limit(limit).
		Find(&transfers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load stock transfers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stock_transfers": transfers,
		"page":            page,
		"limit":           limit,
		"total":           total,
		"total_pages":     int((total + int64(limit) - 1) / int64(limit)),
	})
}

// ReceiveStockTransfer godoc
// PUT /api/v1/warehouse/stock-transfers/:id/receive (warehouse staff only)
// Only staff at the DESTINATION warehouse may confirm receipt. Adds the
// quantity to the destination's inventory and marks the transfer received.
func receiveStockTransferLegacy(c *gin.Context) {
	staffID := c.MustGet("user_id").(uint)
	id := c.Param("id")

	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
		return
	}

	var transfer models.StockTransfer
	if err := database.DB.First(&transfer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock transfer not found"})
		return
	}

	if transfer.ToWarehouseID != staff.WarehouseID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the destination warehouse can receive this transfer"})
		return
	}

	statusCode := http.StatusInternalServerError
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&transfer, id).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("stock transfer not found")
		}
		if transfer.Status != models.StockTransferInTransit {
			statusCode = http.StatusBadRequest
			return errors.New("only in-transit transfers can be received - it may have already been received")
		}

		var inventory models.Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ? AND warehouse_id = ?", transfer.ProductID, transfer.ToWarehouseID).
			First(&inventory).Error
		if err != nil {
			inventory = models.Inventory{
				ProductID:   transfer.ProductID,
				WarehouseID: transfer.ToWarehouseID,
			}
		}
		previousQty := inventory.Stock
		inventory.Stock += transfer.Quantity
		inventory.InStock = inventory.Stock > 0
		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		movement := models.StockMovement{
			ProductID:    transfer.ProductID,
			WarehouseID:  transfer.ToWarehouseID,
			PreviousQty:  previousQty,
			Change:       transfer.Quantity,
			NewQty:       inventory.Stock,
			MovementType: models.MovementTransfer,
			StaffID:      &staffID,
			ReferenceID:  &transfer.ID,
			Notes:        fmt.Sprintf("Stock transfer #%d received from warehouse #%d", transfer.ID, transfer.FromWarehouseID),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		transfer.Status = models.StockTransferReceived
		return tx.Save(&transfer).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	services.LogWarehouseAction(staff.WarehouseID, staffID, staff.Name, "receive_stock_transfer", "stock_transfer",
		fmt.Sprint(transfer.ID), "status=in_transit", "status=received")

	c.JSON(http.StatusOK, gin.H{"stock_transfer": transfer})
}

// ApproveStockTransferByWarehouseStaff godoc
// PUT /api/v1/warehouse/stock-transfers/:id/approve (warehouse staff only)
// Only staff at the SUPPLYING (source) warehouse may approve a transfer.
// Same effect as the admin ApproveStockTransfer: deducts stock from the
// source warehouse immediately and marks the transfer in_transit.
func ApproveStockTransferByWarehouseStaff(c *gin.Context) {
	staffID := c.MustGet("user_id").(uint)
	id := c.Param("id")

	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
		return
	}

	var transfer models.StockTransfer
	if err := database.DB.First(&transfer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock transfer not found"})
		return
	}

	if transfer.FromWarehouseID != staff.WarehouseID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the supplying warehouse can approve this transfer"})
		return
	}

	statusCode := http.StatusInternalServerError
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&transfer, id).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("stock transfer not found")
		}
		if transfer.Status != models.StockTransferPending {
			statusCode = http.StatusBadRequest
			return errors.New("only pending transfers can be approved - it may have already been processed")
		}

		var inventory models.Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", transfer.ProductID, transfer.FromWarehouseID).
			First(&inventory).Error; err != nil {
			statusCode = http.StatusBadRequest
			return errors.New("source warehouse has no inventory record for this product")
		}

		if inventory.Stock-transferReservedQty(tx, transfer.ProductID, transfer.FromWarehouseID) < transfer.Quantity { // BUG-61
			statusCode = http.StatusBadRequest
			return errors.New("insufficient stock at source warehouse to approve this transfer")
		}

		previousQty := inventory.Stock
		inventory.Stock -= transfer.Quantity
		if inventory.Stock <= 0 {
			inventory.InStock = false
		}
		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		movement := models.StockMovement{
			ProductID:    transfer.ProductID,
			WarehouseID:  transfer.FromWarehouseID,
			PreviousQty:  previousQty,
			Change:       -transfer.Quantity,
			NewQty:       inventory.Stock,
			MovementType: models.MovementTransfer,
			StaffID:      &staffID,
			ReferenceID:  &transfer.ID,
			Notes:        fmt.Sprintf("Stock transfer #%d approved, dispatched to warehouse #%d", transfer.ID, transfer.ToWarehouseID),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		transfer.Status = models.StockTransferInTransit
		transfer.ApprovedBy = &staffID
		return tx.Save(&transfer).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	services.LogWarehouseAction(staff.WarehouseID, staffID, staff.Name, "approve_stock_transfer", "stock_transfer",
		fmt.Sprint(transfer.ID), "status=pending", "status=in_transit")
	services.NotifyWarehouse(transfer.ToWarehouseID, models.WhNotifyStockTransfer, // BUG-61: notify the requesting warehouse
		"Stock transfer approved",
		fmt.Sprintf("Your transfer #%d to warehouse #%d was approved and is now in transit.", transfer.ID, transfer.ToWarehouseID),
		nil, &transfer.ProductID)

	c.JSON(http.StatusOK, gin.H{"stock_transfer": transfer})
}

// RejectStockTransferByWarehouseStaff godoc
// PUT /api/v1/warehouse/stock-transfers/:id/reject (warehouse staff only)
// Only staff at the DESTINATION warehouse may reject an incoming transfer.
func RejectStockTransferByWarehouseStaff(c *gin.Context) {
	staffID := c.MustGet("user_id").(uint)
	id := c.Param("id")

	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
		return
	}

	var transfer models.StockTransfer
	if err := database.DB.First(&transfer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock transfer not found"})
		return
	}

	if transfer.ToWarehouseID != staff.WarehouseID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the destination warehouse can reject this transfer"})
		return
	}

	if transfer.Status != models.StockTransferPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending transfers can be rejected"})
		return
	}

	transfer.Status = models.StockTransferRejected
	transfer.ApprovedBy = &staffID
	if err := database.DB.Save(&transfer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject stock transfer"})
		return
	}

	services.LogWarehouseAction(staff.WarehouseID, staffID, staff.Name, "reject_stock_transfer", "stock_transfer",
		fmt.Sprint(transfer.ID), "status=pending", "status=rejected")
	services.NotifyWarehouse(transfer.FromWarehouseID, models.WhNotifyStockTransfer,
		"Stock transfer rejected",
		fmt.Sprintf("Your transfer #%d to warehouse #%d was rejected.", transfer.ID, transfer.ToWarehouseID),
		nil, &transfer.ProductID)

	c.JSON(http.StatusOK, gin.H{"stock_transfer": transfer})
}

// ---------------------------------------------------------------------------
// Stock Transfers - Admin side
// ---------------------------------------------------------------------------

// GetStockTransfers godoc
// GET /api/v1/admin/stock-transfers (admin only)
// Optional ?status= query param to filter.
func GetStockTransfers(c *gin.Context) {
	var transfers []models.StockTransfer
	query := database.DB.
		Preload("Product").Preload("FromWarehouse").Preload("ToWarehouse").
		Order("created_at DESC")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&transfers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load stock transfers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stock_transfers": transfers})
}

// ApproveStockTransfer godoc
// PUT /api/v1/admin/stock-transfers/:id/approve (admin only)
// Deducts stock from the source warehouse immediately and marks the
// transfer in_transit. The destination only gets the stock once it
// confirms receipt via ReceiveStockTransfer.
func ApproveStockTransfer(c *gin.Context) {
	adminID := c.MustGet("user_id").(uint)
	id := c.Param("id")

	var transfer models.StockTransfer
	if err := database.DB.First(&transfer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock transfer not found"})
		return
	}

	statusCode := http.StatusInternalServerError
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&transfer, id).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("stock transfer not found")
		}
		if transfer.Status != models.StockTransferPending {
			statusCode = http.StatusBadRequest
			return errors.New("only pending transfers can be approved - it may have already been processed")
		}

		var inventory models.Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", transfer.ProductID, transfer.FromWarehouseID).
			First(&inventory).Error; err != nil {
			statusCode = http.StatusBadRequest
			return errors.New("source warehouse has no inventory record for this product")
		}

		if inventory.Stock-transferReservedQty(tx, transfer.ProductID, transfer.FromWarehouseID) < transfer.Quantity { // BUG-61
			statusCode = http.StatusBadRequest
			return errors.New("insufficient stock at source warehouse to approve this transfer")
		}

		previousQty := inventory.Stock
		inventory.Stock -= transfer.Quantity
		if inventory.Stock <= 0 {
			inventory.InStock = false
		}
		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		movement := models.StockMovement{
			ProductID:    transfer.ProductID,
			WarehouseID:  transfer.FromWarehouseID,
			PreviousQty:  previousQty,
			Change:       -transfer.Quantity,
			NewQty:       inventory.Stock,
			MovementType: models.MovementTransfer,
			StaffID:      &adminID,
			ReferenceID:  &transfer.ID,
			Notes:        fmt.Sprintf("Stock transfer #%d approved by admin, dispatched to warehouse #%d", transfer.ID, transfer.ToWarehouseID),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		transfer.Status = models.StockTransferInTransit
		transfer.ApprovedBy = &adminID
		return tx.Save(&transfer).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stock_transfer": transfer})
}

// RejectStockTransfer godoc
// PUT /api/v1/admin/stock-transfers/:id/reject (admin only)
func RejectStockTransfer(c *gin.Context) {
	adminID := c.MustGet("user_id").(uint)
	id := c.Param("id")

	var transfer models.StockTransfer
	if err := database.DB.First(&transfer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock transfer not found"})
		return
	}

	if transfer.Status != models.StockTransferPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending transfers can be rejected"})
		return
	}

	transfer.Status = models.StockTransferRejected
	transfer.ApprovedBy = &adminID
	if err := database.DB.Save(&transfer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject stock transfer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stock_transfer": transfer})
}

// CancelStockTransfer godoc
// PUT /api/v1/admin/stock-transfers/:id/cancel (admin only)
// Cancels an in-transit transfer and restores the deducted stock back to
// the source warehouse. Use this when goods are lost, damaged, or the
// transfer needs to be undone after approval.
func CancelStockTransfer(c *gin.Context) {
	id := c.Param("id")

	var transfer models.StockTransfer
	if err := database.DB.First(&transfer, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock transfer not found"})
		return
	}

	statusCode := http.StatusInternalServerError
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&transfer, id).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("stock transfer not found")
		}
		if transfer.Status != models.StockTransferInTransit {
			statusCode = http.StatusBadRequest
			return errors.New("only in-transit transfers can be cancelled - it may have already been processed")
		}

		var inventory models.Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ? AND warehouse_id = ?", transfer.ProductID, transfer.FromWarehouseID).
			First(&inventory).Error
		if err != nil {
			inventory = models.Inventory{
				ProductID:   transfer.ProductID,
				WarehouseID: transfer.FromWarehouseID,
			}
		}
		previousQty := inventory.Stock
		inventory.Stock += transfer.Quantity
		inventory.InStock = inventory.Stock > 0
		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		movement := models.StockMovement{
			ProductID:    transfer.ProductID,
			WarehouseID:  transfer.FromWarehouseID,
			PreviousQty:  previousQty,
			Change:       transfer.Quantity,
			NewQty:       inventory.Stock,
			MovementType: models.MovementCorrection,
			ReferenceID:  &transfer.ID,
			Notes:        fmt.Sprintf("Stock transfer #%d cancelled - stock restored to source warehouse", transfer.ID),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		transfer.Status = models.StockTransferCancelled
		return tx.Save(&transfer).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stock_transfer": transfer})
}
