package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// ReceiveStockTransfer PUT /warehouse/stock-transfers/:id/receive
// Truck arrived: no stock is added yet. A Receiving record is created for the
// destination store; stock and bin are updated only when putaway completes.
func ReceiveStockTransfer(c *gin.Context) {
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

		supplier := "Warehouse"
		var from models.Warehouse
		if err := tx.First(&from, transfer.FromWarehouseID).Error; err == nil {
			supplier = from.Name
		}

		now := time.Now()
		rec := models.Receiving{
			WarehouseID:       transfer.ToWarehouseID,
			SupplierName:      supplier,
			ReferenceNumber:   fmt.Sprintf("Transfer #%d", transfer.ID),
			ProductID:         transfer.ProductID,
			ExpectedQuantity:  transfer.Quantity,
			ReceivedQuantity:  transfer.Quantity,
			Status:            models.ReceivingStatusReceived,
			CreatedByStaffID:  staffID,
			ReceivedByStaffID: &staffID,
			ReceivedAt:        &now,
			Notes:             fmt.Sprintf("Truck arrived for stock transfer #%d", transfer.ID),
		}
		if err := tx.Create(&rec).Error; err != nil {
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
