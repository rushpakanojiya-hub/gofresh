package handlers

import (
"crypto/rand"
"fmt"
"log"
"math/big"
"net/http"
"os"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
    "gorm.io/gorm"
"gorm.io/gorm/clause"
)

const otpValidityMinutes = 5

// generateOTP returns a random 6-digit numeric code, e.g. "042917".
func generateOTP() (string, error) {
n, err := rand.Int(rand.Reader, big.NewInt(1000000))
if err != nil {
return "", err
}
return fmt.Sprintf("%06d", n.Int64()), nil
}

// otpDebugResponse builds the OTP-send response. In any non-release
// environment it echoes the code back so local/staging testing works
// without a real SMS gateway wired up. In production (GIN_MODE=release)
// the code is NEVER included in the response - only logged server-side -
// since returning it in the API response would let anyone log in as any
// phone number without ever receiving an SMS.
func otpDebugResponse(code string, phone string) gin.H {
    log.Printf("[OTP] %s -> %s", phone, code)
    resp := gin.H{
        "message":            "OTP sent successfully",
        "expires_in_minutes": otpValidityMinutes,
    }
    // Only echo the OTP in the response outside production, so local/staging
    // testing works without a real SMS gateway wired up. In production
    // (GIN_MODE=release) the code is NEVER included in the response - only
    // logged server-side - since returning it in the API response would let
    // anyone log in as any phone number without ever receiving an SMS.
    if os.Getenv("OTP_DEBUG_ECHO") == "true" {
        resp["otp"] = code
    }
    return resp
}

// SendOTP godoc
// POST /api/v1/auth/send-otp
// No real SMS gateway is wired up yet, so the OTP is logged server-side and
// only echoed in the response outside production - see otpDebugResponse.
func SendOTP(c *gin.Context) {
var req models.SendOTPRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

code, err := generateOTP()
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
return
}

// Clear any older codes for this phone, then store the fresh one.
database.DB.Where("phone = ?", req.Phone).Delete(&models.OTP{})
otp := models.OTP{
Phone:     req.Phone,
Code:      code,
ExpiresAt: time.Now().Add(otpValidityMinutes * time.Minute),
}
if err := database.DB.Create(&otp).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save OTP"})
return
}

c.JSON(http.StatusOK, otpDebugResponse(code, req.Phone))
}

// VerifyOTP godoc
// POST /api/v1/auth/verify-otp
// TEST MODE: checks the code against the local OTP table instead of Twilio.
// Creates the user on first login, logs them in on repeat visits.
func VerifyOTP(c *gin.Context) {
var req models.VerifyOTPRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var otp models.OTP
err := database.DB.Where("phone = ?", req.Phone).
Order("id desc").First(&otp).Error
if err != nil {
c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired OTP"})
return
}
if otp.Attempts >= config.AppConfig.CustomerOTPMaxAttempts {
c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many incorrect attempts. Please request a new OTP."})
return
}
if time.Now().After(otp.ExpiresAt) {
c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired OTP"})
return
}
if otp.Code != req.OTP {
database.DB.Model(&models.OTP{}).
Where("id = ?", otp.ID).
Update("attempts", gorm.Expr("attempts + 1"))
c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired OTP"})
return
}
database.DB.Delete(&otp)

// Find or create the user -- first successful OTP verification = signup.
var user models.User
err = database.DB.Where("phone = ?", req.Phone).First(&user).Error
if err != nil {
    user = models.User{
        Phone: req.Phone,
        Role:  "customer",
    }
    // Use ON CONFLICT DO NOTHING (keyed on the unique phone index) instead
    // of a plain Create, so that if a concurrent request for the same new
    // phone number wins the race and creates the user first, this insert
    // becomes a silent no-op (user.ID stays 0) instead of erroring out with
    // a unique-constraint violation and failing an otherwise-valid login.
    if err := database.DB.Clauses(clause.OnConflict{
        Columns:   []clause.Column{{Name: "phone"}},
        DoNothing: true,
    }).Create(&user).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }
    if user.ID == 0 {
        // Lost the race - another concurrent request already created this
        // user. Re-fetch the winning row instead of proceeding with a
        // zero-value user.
        if err := database.DB.Where("phone = ?", req.Phone).First(&user).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user"})
            return
        }
    } else {
        // Won the race - we're the ones who actually inserted the new user,
        // so we're responsible for creating their cart.
        cart := models.Cart{UserID: user.ID}
        database.DB.Create(&cart)
    }
}

if user.IsBlocked {
        c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been blocked. Please contact support."})
return
}

token, err := utils.GenerateJWT(user.ID, user.Phone, user.Role)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
return
}

c.JSON(http.StatusOK, models.AuthResponse{
Token: token,
User:  user,
})
}

// Me godoc - returns the logged-in user's profile
// GET /api/v1/auth/me (protected)
func Me(c *gin.Context) {
userID, _ := c.Get("user_id")

var user models.User
if err := database.DB.First(&user, userID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
return
}

c.JSON(http.StatusOK, user)
}

// UpdateProfile godoc - updates the logged-in user's editable profile fields
// PUT /api/v1/auth/me (protected)
func UpdateProfile(c *gin.Context) {
userID, _ := c.Get("user_id")

var req models.UpdateProfileRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var user models.User
if err := database.DB.First(&user, userID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
return
}

user.Name = req.Name
if err := database.DB.Save(&user).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
return
}

c.JSON(http.StatusOK, user)
}

// DeleteAccount godoc
// DELETE /api/v1/auth/me (protected)
// Soft-deletes the account by reusing the existing IsBlocked flag, which
// VerifyOTP already checks and refuses login for. No new column needed.
func DeleteAccount(c *gin.Context) {
userID, _ := c.Get("user_id")

var user models.User
if err := database.DB.First(&user, userID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
return
}

if err := database.DB.Model(&user).Update("is_blocked", true).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
return
}

c.JSON(http.StatusOK, gin.H{"message": "Account deleted successfully"})
}
