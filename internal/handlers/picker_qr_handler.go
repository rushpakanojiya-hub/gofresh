package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// GetPickerHandoverQR: GET /warehouse/picker/orders/:id/handover-qr
// Picker shows this token as a QR for the delivery partner to scan.
func GetPickerHandoverQR(c *gin.Context) {
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
	var task models.PickingTask
	if err := database.DB.Where("order_id = ? AND picker_id = ?", orderID, staffID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Picking task not found for you"})
		return
	}
	if task.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Complete picking first"})
		return
	}
	var order models.Order
	database.DB.Preload("Address").First(&order, orderID)
	c.JSON(http.StatusOK, gin.H{
		"token":         services.GeneratePickerQRToken(uint(orderID)),
		"order_id":      orderID,
		"customer_name": order.Address.FullName,
	})
}

// VerifyPickerQR: POST /delivery/orders/:id/verify-picker-qr {"token": "..."}
func VerifyPickerQR(c *gin.Context) {
	partnerID := c.MustGet("user_id").(uint)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	tokOrder, err := services.ParsePickerQRToken(req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if uint64(tokOrder) != orderID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This QR belongs to a different order"})
		return
	}
	var order models.Order
	if err := database.DB.Preload("Address").
		Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not assigned to you"})
		return
	}
	pickerName := ""
	var st models.WarehouseStaff
	sub := database.DB.Model(&models.PickingTask{}).Select("picker_id").Where("order_id = ?", orderID).Limit(1)
	if e := database.DB.Where("id = (?)", sub).First(&st).Error; e == nil {
		pickerName = st.Name
	}
	c.JSON(http.StatusOK, gin.H{
		"verified":      true,
		"order_id":      orderID,
		"customer_name": order.Address.FullName,
		"picker_name":   pickerName,
	})
}
