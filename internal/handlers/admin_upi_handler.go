package handlers

import (
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// GET /api/v1/admin/upi-collections?status=unverified|verified|rejected|all (admin only)
func GetUPICollections(c *gin.Context) {
    status := c.DefaultQuery("status", "unverified")
    q := database.DB.Preload("DeliveryPartner").Where("collected_via = ?", "upi")
    if status != "all" {
        q = q.Where("upi_status = ?", status)
    }
    var orders []models.Order
    if err := q.Order("delivered_at DESC").Limit(200).Find(&orders).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load UPI collections"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"orders": orders})
}

type reviewUPIRequest struct {
    Action string `json:"action" binding:"required,oneof=verify reject"`
    UTR    string `json:"utr"`
}

// PUT /api/v1/admin/upi-collections/:id/review (admin only)
// verify: the money reached the bank. reject: it did not, so the amount
// counts as cash the rider is holding again.
func ReviewUPICollection(c *gin.Context) {
    var req reviewUPIRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "action must be verify or reject"})
        return
    }
    var order models.Order
    if err := database.DB.Where("id = ? AND collected_via = ?", c.Param("id"), "upi").First(&order).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "UPI collection not found"})
        return
    }
    if order.UPIStatus != nil && *order.UPIStatus == "rejected" {
        c.JSON(http.StatusConflict, gin.H{"error": "Already reviewed: " + *order.UPIStatus})
        return
    }

    updates := map[string]interface{}{"upi_verified_at": time.Now()}
    newStatus := "verified"
    if req.Action == "reject" {
        newStatus = "rejected"
    } else if utr := strings.TrimSpace(req.UTR); utr != "" {
        updates["upi_utr"] = utr
    }
    updates["upi_status"] = newStatus

    if err := database.DB.Model(&models.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update UPI status"})
        return
    }

    if newStatus == "rejected" && order.DeliveryPartnerID != nil {
        oid := order.ID
        services.CreateDeliveryNotification(*order.DeliveryPartnerID, "UPI payment not received",
            fmt.Sprintf("UPI payment for order #%d (Rs %.2f) was not received. It now counts as cash you hold.", order.ID, order.TotalAmount),
            "cod", &oid)
    }
    c.JSON(http.StatusOK, gin.H{"message": "UPI collection " + newStatus, "order_id": order.ID, "upi_status": newStatus})
}