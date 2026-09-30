package handlers

import (
"net/http"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
"gorm.io/gorm"
)

// SendWarehouseStaffOTP godoc
// POST /api/v1/warehouse/send-otp
// Sends an OTP to a phone number that belongs to an existing, active
// warehouse staff member (created by the admin). Does NOT create a new
// staff member — the admin must add them first.
func SendWarehouseStaffOTP(c *gin.Context) {
var req models.SendOTPRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var staff models.WarehouseStaff
if err := database.DB.Where("phone = ?", req.Phone).First(&staff).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "No warehouse staff found with this phone number"})
return
}
if !staff.IsActive {
c.JSON(http.StatusForbidden, gin.H{"error": "This warehouse staff account is inactive"})
return
}

code, err := generateOTP()
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
return
}

database.DB.Where("phone = ? AND verified = ?", req.Phone, false).Delete(&models.OTP{})
otp := models.OTP{
Phone:     req.Phone,
Code:      code,
ExpiresAt: time.Now().Add(otpValidityMinutes * time.Minute),
Verified:  false,
}
if err := database.DB.Create(&otp).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create OTP"})
return
}

c.JSON(http.StatusOK, otpDebugResponse(code, req.Phone))
}

// VerifyWarehouseStaffOTP godoc
// POST /api/v1/warehouse/verify-otp
// Verifies the OTP and returns a JWT with role "warehouse_staff".
func VerifyWarehouseStaffOTP(c *gin.Context) {
var req models.VerifyOTPRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var otp models.OTP
err := database.DB.
Where("phone = ? AND verified = ?", req.Phone, false).
Order("created_at DESC").
First(&otp).Error
if err != nil {
c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OTP"})
return
}
if otp.Attempts >= 5 {
c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many incorrect attempts. Please request a new OTP."})
return
}
if time.Now().After(otp.ExpiresAt) {
c.JSON(http.StatusUnauthorized, gin.H{"error": "OTP has expired, please request a new one"})
return
}
if otp.Code != req.OTP {
database.DB.Model(&models.OTP{}).Where("id = ?", otp.ID).Update("attempts", gorm.Expr("attempts + 1"))
c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OTP"})
return
}
database.DB.Model(&otp).Update("verified", true)

var staff models.WarehouseStaff
if err := database.DB.Preload("Warehouse").Where("phone = ?", req.Phone).First(&staff).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse staff not found"})
return
}
if !staff.IsActive {
c.JSON(http.StatusForbidden, gin.H{"error": "This warehouse staff account is inactive"})
return
}

token, err := utils.GenerateJWT(staff.ID, staff.Phone, "warehouse_staff")
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
return
}

c.JSON(http.StatusOK, gin.H{"token": token, "warehouse_staff": staff})
}
