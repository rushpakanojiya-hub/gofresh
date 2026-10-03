package handlers

import (
    "net/http"
    "strconv"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// GET /admin/delivery-partners/:id/onboarding (admin only)
// Full onboarding submission incl. documents and payout details.
func GetPartnerOnboardingAdmin(c *gin.Context) {
    id, err := strconv.ParseUint(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid partner id"})
        return
    }
    var p models.DeliveryPartner
    if err := database.DB.First(&p, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
        return
    }
    view := onboardingView(p)
    view["id"] = p.ID
    view["name"] = p.Name
    view["phone"] = p.Phone
    view["is_active"] = p.IsActive
    view["upi_id"] = p.UPIID
    view["bank_account_holder"] = p.BankAccountHolder
    view["bank_account_no"] = p.BankAccountNo
    view["bank_ifsc"] = p.BankIFSC
    c.JSON(http.StatusOK, view)
}

// PUT /admin/delivery-partners/:id/onboarding/review (admin only)
// {action: "approve"|"reject", reason?: string}. Only a "pending" submission
// can be reviewed. Approve activates the account.
func ReviewPartnerOnboarding(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid partner id"})
        return
    }
    var req struct {
        Action string `json:"action" binding:"required,oneof=approve reject"`
        Reason string `json:"reason"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    reason := strings.TrimSpace(req.Reason)
    if req.Action == "reject" && reason == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required to reject"})
        return
    }

    var updates map[string]interface{}
    title, body := "Application approved", "You are approved. You can go online and start taking orders."
    if req.Action == "approve" {
        updates = map[string]interface{}{"approval_status": approvalApproved, "is_active": true}
    } else {
        updates = map[string]interface{}{"approval_status": approvalRejected}
        title, body = "Application needs changes", "Your application was rejected: "+reason
    }

    res := database.DB.Model(&models.DeliveryPartner{}).
        Where("id = ? AND approval_status = ?", id, approvalPending).
        Updates(updates)
    if res.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update partner"})
        return
    }
    if res.RowsAffected == 0 {
        c.JSON(http.StatusConflict, gin.H{"error": "Partner has no pending submission"})
        return
    }

    adminID := c.MustGet("user_id").(uint)
    adminPhone := c.MustGet("phone").(string)
    utils.LogAudit(adminID, adminPhone, "review_partner_onboarding", "delivery_partner", idStr, req.Action)
    go services.SendPushToPartnerWithData(uint(id), title, body, map[string]string{"type": "onboarding_" + req.Action})

    c.JSON(http.StatusOK, gin.H{"message": "Partner " + req.Action + "d"})
}