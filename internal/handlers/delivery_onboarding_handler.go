package handlers

import (
    "net/http"
    "regexp"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "gorm.io/gorm"
)

const (
    approvalOnboarding = "onboarding"
    approvalPending    = "pending"
    approvalApproved   = "approved"
    approvalRejected   = "rejected"
)

var (
    ifscRe            = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
    accountRe         = regexp.MustCompile(`^[0-9]{9,18}$`)
    validVehicleTypes = map[string]bool{"motorcycle": true, "bicycle": true, "electric_scooter": true}
)

// partnerLoginBlocked: only an admin-deactivated, previously approved account
// (or an unknown status) may not log in. Onboarding/pending/rejected may.
func partnerLoginBlocked(p models.DeliveryPartner) bool {
    if p.IsActive {
        return false
    }
    return p.ApprovalStatus != approvalOnboarding && p.ApprovalStatus != approvalPending && p.ApprovalStatus != approvalRejected
}

func onboardingView(p models.DeliveryPartner) gin.H {
    return gin.H{
        "onboarding_step": p.OnboardingStep,
        "approval_status": p.ApprovalStatus,
        "vehicle_type":    p.VehicleType,
        "vehicle_number":  p.VehicleNumber,
        "warehouse_id":    p.WarehouseID,
        "licence_url":     p.LicenceURL,
        "rc_url":          p.RCURL,
        "aadhaar_url":     p.AadhaarURL,
        "selfie_url":      p.SelfieURL,
        "id_doc_type":     p.IDDocType,
        "voter_url":       p.VoterURL,
        "pan_url":         p.PANURL,
        "payout_added":    p.UPIID != "" || p.BankAccountNo != "",
    }
}

func loadOnboardingPartner(c *gin.Context) (*models.DeliveryPartner, bool) {
    partnerID := c.MustGet("user_id").(uint)
    var p models.DeliveryPartner
    if err := database.DB.First(&p, partnerID).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Delivery partner not found"})
        return nil, false
    }
    return &p, true
}

// requireEditable: edits allowed only while onboarding or after a rejection,
// and only once the previous step is done (minStep = this step - 1).
func requireEditable(c *gin.Context, p *models.DeliveryPartner, minStep int) bool {
    if p.ApprovalStatus != approvalOnboarding && p.ApprovalStatus != approvalRejected {
        c.JSON(http.StatusConflict, gin.H{"error": "Onboarding already submitted"})
        return false
    }
    if p.OnboardingStep < minStep {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Complete the previous step first"})
        return false
    }
    return true
}

func saveOnboardingStep(c *gin.Context, p *models.DeliveryPartner, step int, updates map[string]interface{}) {
    updates["onboarding_step"] = gorm.Expr("GREATEST(onboarding_step, ?)", step)
    if err := database.DB.Model(&models.DeliveryPartner{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
        return
    }
    database.DB.First(p, p.ID)
    c.JSON(http.StatusOK, onboardingView(*p))
}

func validURL(s string) bool {
    return len(s) <= 500 && (strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://"))
}

// GET /delivery/onboarding
func GetOnboarding(c *gin.Context) {
    p, ok := loadOnboardingPartner(c)
    if !ok {
        return
    }
    c.JSON(http.StatusOK, onboardingView(*p))
}

// GET /delivery/stores
func GetDeliveryStores(c *gin.Context) {
    var stores []models.Warehouse
    if err := database.DB.Select("id", "name", "city", "address", "lat", "lng").
        Where("is_active = ?", true).Order("city, name").Find(&stores).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load stores"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"stores": stores})
}

// PUT /delivery/onboarding/vehicle  (step 1)
func SaveOnboardingVehicle(c *gin.Context) {
    p, ok := loadOnboardingPartner(c)
    if !ok || !requireEditable(c, p, 0) {
        return
    }
    var req struct {
        VehicleType   string `json:"vehicle_type" binding:"required"`
        VehicleNumber string `json:"vehicle_number"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    vt := strings.ToLower(strings.TrimSpace(req.VehicleType))
    if !validVehicleTypes[vt] {
        c.JSON(http.StatusBadRequest, gin.H{"error": "vehicle_type must be motorcycle, bicycle or electric_scooter"})
        return
    }
    vn := strings.ToUpper(strings.TrimSpace(req.VehicleNumber))
    if len(vn) > 20 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "vehicle_number too long"})
        return
    }
    saveOnboardingStep(c, p, 1, map[string]interface{}{"vehicle_type": vt, "vehicle_number": vn})
}

// PUT /delivery/onboarding/store  (step 2)
func SaveOnboardingStore(c *gin.Context) {
    p, ok := loadOnboardingPartner(c)
    if !ok || !requireEditable(c, p, 1) {
        return
    }
    var req struct {
        WarehouseID uint `json:"warehouse_id" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    var wh models.Warehouse
    if err := database.DB.Where("id = ? AND is_active = ?", req.WarehouseID, true).First(&wh).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Store not found"})
        return
    }
    saveOnboardingStep(c, p, 2, map[string]interface{}{"warehouse_id": req.WarehouseID})
}

// PUT /delivery/onboarding/documents  (step 3)
// Any one of: aadhaar | driving_licence | voter_pan (voter ID + PAN).
func SaveOnboardingDocuments(c *gin.Context) {
    p, ok := loadOnboardingPartner(c)
    if !ok || !requireEditable(c, p, 2) {
        return
    }
    var req struct {
        IDDocType  string `json:"id_doc_type"`
        AadhaarURL string `json:"aadhaar_url"`
        LicenceURL string `json:"licence_url"`
        VoterURL   string `json:"voter_url"`
        PANURL     string `json:"pan_url"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    t := strings.ToLower(strings.TrimSpace(req.IDDocType))
    updates := map[string]interface{}{
        "id_doc_type": t, "aadhaar_url": "", "licence_url": "", "voter_url": "", "pan_url": "",
    }
    switch t {
    case "aadhaar":
        if !validURL(req.AadhaarURL) {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Aadhaar photo is required"})
            return
        }
        updates["aadhaar_url"] = req.AadhaarURL
    case "driving_licence":
        if !validURL(req.LicenceURL) {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Driving licence photo is required"})
            return
        }
        updates["licence_url"] = req.LicenceURL
    case "voter_pan":
        if !validURL(req.VoterURL) || !validURL(req.PANURL) {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Voter ID and PAN card photos are required"})
            return
        }
        updates["voter_url"] = req.VoterURL
        updates["pan_url"] = req.PANURL
    default:
        c.JSON(http.StatusBadRequest, gin.H{"error": "id_doc_type must be aadhaar, driving_licence or voter_pan"})
        return
    }
    saveOnboardingStep(c, p, 3, updates)
}
// PUT /delivery/onboarding/selfie  (step 4)
func SaveOnboardingSelfie(c *gin.Context) {
    p, ok := loadOnboardingPartner(c)
    if !ok || !requireEditable(c, p, 3) {
        return
    }
    var req struct {
        SelfieURL string `json:"selfie_url"`
    }
    if err := c.ShouldBindJSON(&req); err != nil || !validURL(req.SelfieURL) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Selfie is required"})
        return
    }
    saveOnboardingStep(c, p, 4, map[string]interface{}{"selfie_url": req.SelfieURL})
}

// PUT /delivery/onboarding/payout  (step 5, submits for admin review)
func SaveOnboardingPayout(c *gin.Context) {
    p, ok := loadOnboardingPartner(c)
    if !ok || !requireEditable(c, p, 4) {
        return
    }
    var req struct {
        UPIID         string `json:"upi_id"`
        AccountHolder string `json:"account_holder"`
        AccountNo     string `json:"account_no"`
        IFSC          string `json:"ifsc"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    upi := strings.ToLower(strings.TrimSpace(req.UPIID))
    holder := strings.TrimSpace(req.AccountHolder)
    acct := strings.TrimSpace(req.AccountNo)
    ifsc := strings.ToUpper(strings.TrimSpace(req.IFSC))
    if upi != "" {
        if !strings.Contains(upi, "@") || strings.Contains(upi, " ") || len(upi) > 80 {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UPI ID"})
            return
        }
        holder, acct, ifsc = "", "", ""
    } else {
        if holder == "" || len(holder) > 120 || !accountRe.MatchString(acct) || !ifscRe.MatchString(ifsc) {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Enter a UPI ID, or account holder, account number and IFSC"})
            return
        }
    }
    saveOnboardingStep(c, p, 5, map[string]interface{}{
        "upi_id": upi, "bank_account_holder": holder, "bank_account_no": acct, "bank_ifsc": ifsc,
        "approval_status": approvalPending,
    })
}