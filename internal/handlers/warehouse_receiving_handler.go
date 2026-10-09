package handlers

import (
	"errors"
	"fmt"
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

// CreateReceiving godoc
// POST /api/v1/warehouse/receiving (warehouse staff only)
func CreateReceiving(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)

	var req models.CreateReceivingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var product models.Product
	if err := database.DB.First(&product, req.ProductID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found"})
		return
	}

	rec := models.Receiving{
		WarehouseID:      warehouseID,
		SupplierName:     req.SupplierName,
		ReferenceNumber:  req.ReferenceNumber,
		ProductID:        req.ProductID,
		ExpectedQuantity: req.ExpectedQuantity,
		Status:           models.ReceivingStatusPending,
		CreatedByStaffID: staffID,
		Notes:            req.Notes,
	}
	if err := database.DB.Create(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create receiving record"})
		return
	}

	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "receiving_created", "receiving", strconv.Itoa(int(rec.ID)),
		"", fmt.Sprintf("expected=%d product=%s", rec.ExpectedQuantity, product.Name))

	services.NotifyWarehouse(warehouseID, models.WhNotifyReceiving,
		"New receiving logged",
		fmt.Sprintf("Expecting %d units of %s from %s.", rec.ExpectedQuantity, product.Name, rec.SupplierName),
		nil, &rec.ProductID)

	c.JSON(http.StatusCreated, rec)
}

// GetWarehouseReceivings godoc
// GET /api/v1/warehouse/receiving?status=&page=&limit= (warehouse staff only)
func GetWarehouseReceivings(c *gin.Context) {
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

	db := database.DB.Model(&models.Receiving{}).Where("warehouse_id = ?", warehouseID)
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}

	var total int64
	db.Count(&total)

	var receivings []models.Receiving
	offset := (page - 1) * limit
	if err := db.Preload("Product").Preload("Bin.Rack.Zone").
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&receivings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch receiving records"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"receivings":  receivings,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int((total + int64(limit) - 1) / int64(limit)),
	})
}

// GetReceiving godoc
// GET /api/v1/warehouse/receiving/:id (warehouse staff only)
func GetReceiving(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	id := c.Param("id")

	var rec models.Receiving
	if err := database.DB.Where("id = ? AND warehouse_id = ?", id, warehouseID).
		Preload("Product").Preload("Bin.Rack.Zone").First(&rec).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Receiving record not found for your warehouse"})
		return
	}
	c.JSON(http.StatusOK, rec)
}

// MarkReceiving godoc
// PUT /api/v1/warehouse/receiving/:id/receive (warehouse staff only)
// Records the physically received + damaged quantities. Does not touch
// inventory yet - that only happens after QC + put-away.
func MarkReceiving(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	id := c.Param("id")

	var req models.MarkReceivedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var rec models.Receiving
	statusCode := http.StatusInternalServerError
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", id, warehouseID).First(&rec).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("Receiving record not found for your warehouse")
		}
		if rec.Status != models.ReceivingStatusPending {
			statusCode = http.StatusBadRequest
			return errors.New("Only pending receiving records can be marked received, current status: " + rec.Status)
		}

		now := time.Now()
		rec.ReceivedQuantity = req.ReceivedQuantity
		rec.DamagedQuantity = req.DamagedQuantity
		rec.Status = models.ReceivingStatusReceived
		rec.ReceivedByStaffID = &staffID
		rec.ReceivedAt = &now
		if req.Notes != "" {
			rec.Notes = req.Notes
		}
		return tx.Save(&rec).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	staffName, _ := c.Get("staff_name")
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "receiving_marked_received", "receiving", id,
		"status=pending", fmt.Sprintf("received=%d damaged=%d", req.ReceivedQuantity, req.DamagedQuantity))

	c.JSON(http.StatusOK, rec)
}

// QCReceiving godoc
// PUT /api/v1/warehouse/receiving/:id/qc (warehouse staff only)
func QCReceiving(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	id := c.Param("id")

	var req models.QCReceivingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var rec models.Receiving
	statusCode := http.StatusInternalServerError
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", id, warehouseID).First(&rec).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("Receiving record not found for your warehouse")
		}
		if rec.Status != models.ReceivingStatusReceived {
			statusCode = http.StatusBadRequest
			return errors.New("Only received records can go through QC, current status: " + rec.Status)
		}

		if req.Action != "accept" && req.Action != "reject" {
			statusCode = http.StatusBadRequest
			return errors.New("action must be either \"accept\" or \"reject\"")
		}

		if req.Action == "accept" && (req.AcceptedQuantity <= 0 || req.AcceptedQuantity > rec.ReceivedQuantity) {
			statusCode = http.StatusBadRequest
			return errors.New("accepted_quantity must be between 1 and received_quantity")
		}
		if req.Action == "reject" && req.RejectionReason == "" {
			statusCode = http.StatusBadRequest
			return errors.New("rejection_reason is required when rejecting")
		}

		now := time.Now()
		rec.QCByStaffID = &staffID
		rec.QCAt = &now
		if req.Action == "accept" {
			rec.AcceptedQuantity = req.AcceptedQuantity
			rec.Status = models.ReceivingStatusAccepted
		} else {
			rec.Status = models.ReceivingStatusRejected
			rec.RejectionReason = req.RejectionReason
		}
		return tx.Save(&rec).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": txErr.Error()})
		return
	}

	staffName, _ := c.Get("staff_name")
	opsCreatePutawayTask(rec)
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "receiving_qc", "receiving", id,
		"status=received", "status="+rec.Status)

	c.JSON(http.StatusOK, rec)
}

// PutAwayReceiving godoc
// PUT /api/v1/warehouse/receiving/:id/putaway (warehouse staff only)
// Adds accepted_quantity to the warehouse's inventory for this product,
// optionally assigns a bin, and writes a StockMovement(receive) record -
// all inside one transaction with the inventory row locked.
func PutAwayReceiving(c *gin.Context) {
	warehouseID := c.MustGet("warehouse_id").(uint)
	staffID := c.MustGet("staff_id").(uint)
	id := c.Param("id")

	var req models.PutAwayReceivingRequest
	_ = c.ShouldBindJSON(&req)

	// Existence/ownership pre-check only - the authoritative status check
	// happens inside the transaction below, under a row lock, so two
	// concurrent put-away calls for the same receiving record can't both
	// pass and both add stock (duplicate receiving / double addition).
	var rec models.Receiving
	if err := database.DB.Where("id = ? AND warehouse_id = ?", id, warehouseID).First(&rec).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Receiving record not found for your warehouse"})
		return
	}
	if rec.Status != models.ReceivingStatusAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only accepted records can be put away, current status: " + rec.Status})
		return
	}

	statusCode := http.StatusInternalServerError
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		// Re-fetch and lock the receiving row itself, then re-check its status.
		// This closes the race where two simultaneous requests both read
		// status=accepted before either commits.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND warehouse_id = ?", id, warehouseID).
			First(&rec).Error; err != nil {
			statusCode = http.StatusNotFound
			return errors.New("receiving record not found for your warehouse")
		}
		if rec.Status != models.ReceivingStatusAccepted {
			statusCode = http.StatusBadRequest
			return errors.New("only accepted records can be put away - it may have already been put away, current status: " + rec.Status)
		}

		var inv models.Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", rec.ProductID, warehouseID).
			First(&inv).Error
		if err == gorm.ErrRecordNotFound {
			inv = models.Inventory{ProductID: rec.ProductID, WarehouseID: warehouseID, Stock: 0, InStock: false}
			if err := tx.Create(&inv).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		previousQty := inv.Stock
		inv.Stock += rec.AcceptedQuantity
		inv.InStock = inv.Stock > 0
		if req.BinID != nil {
			inv.BinID = req.BinID
		}
		if err := tx.Save(&inv).Error; err != nil {
			return err
		}

		movement := models.StockMovement{
			ProductID:    rec.ProductID,
			WarehouseID:  warehouseID,
			PreviousQty:  previousQty,
			Change:       rec.AcceptedQuantity,
			NewQty:       inv.Stock,
			MovementType: models.MovementReceive,
			StaffID:      &staffID,
			ReferenceID:  &rec.ID,
			Notes:        fmt.Sprintf("Receiving #%d from %s", rec.ID, rec.SupplierName),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		now := time.Now()
		rec.Status = models.ReceivingStatusPutAway
		rec.PutAwayByStaffID = &staffID
		rec.PutAwayAt = &now
		rec.BinID = req.BinID
		return tx.Save(&rec).Error
	})

	if txErr != nil {
		c.JSON(statusCode, gin.H{"error": "Failed to put away: " + txErr.Error()})
		return
	}

	staffName, _ := c.Get("staff_name")
	opsClosePutawayTask(rec.ID, staffID, rec.BinID)
	services.LogWarehouseAction(warehouseID, staffID, fmt.Sprint(staffName), "receiving_put_away", "receiving", id,
		"status=accepted", "status=put_away")

	c.JSON(http.StatusOK, rec)
}
