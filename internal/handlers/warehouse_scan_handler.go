package handlers

import (
"net/http"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// ScanPickItemRequest is the body for PUT /warehouse/picking/items/:item_id/scan
type ScanPickItemRequest struct {
Barcode string `json:"barcode" binding:"required"`
}

// ScanPickItem godoc
// PUT /api/v1/warehouse/picking/items/:item_id/scan (warehouse staff only)
// Verifies a scanned barcode against the item's product before allowing it
// to be marked picked. Does NOT mark the item picked itself - frontend
// calls MarkPickItem separately once this returns match=true, or after an
// explicit manual override.
func ScanPickItem(c *gin.Context) {
warehouseID := c.MustGet("warehouse_id").(uint)
itemID := c.Param("item_id")

var req ScanPickItemRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var item models.PickingTaskItem
if err := database.DB.Preload("Product").First(&item, itemID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Picking item not found"})
return
}

var task models.PickingTask
if err := database.DB.Where("id = ? AND warehouse_id = ?", item.PickingTaskID, warehouseID).First(&task).Error; err != nil {
c.JSON(http.StatusForbidden, gin.H{"error": "This item does not belong to your warehouse"})
return
}

scanned := req.Barcode
match := item.Product.Barcode != "" && item.Product.Barcode == scanned

c.JSON(http.StatusOK, gin.H{
"match":        match,
"item_id":      item.ID,
"product_id":   item.ProductID,
"product_name": item.Product.Name,
})
}
