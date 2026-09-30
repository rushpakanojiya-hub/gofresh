package handlers

import (
"fmt"
"net/http"
"strconv"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
"gorm.io/gorm"
"gorm.io/gorm/clause"
)

// ---------------------------------------------------------------------------
// Wallet - customer side
// ---------------------------------------------------------------------------

// GetWallet godoc
// GET /api/v1/wallet (protected)
// Returns the user's current balance and recent transaction history.
func GetWallet(c *gin.Context) {
userID := c.MustGet("user_id").(uint)

var wallet models.Wallet
err := database.DB.Where("user_id = ?", userID).First(&wallet).Error
if err != nil {
// No wallet yet — user has never had a credit/debit. Report a zero
// balance instead of a 404, since "no wallet" == "empty wallet" from
// the client's point of view.
c.JSON(http.StatusOK, models.WalletResponse{
Balance:      0,
Transactions: []models.WalletTransaction{},
})
return
}

var transactions []models.WalletTransaction
database.DB.Where("wallet_id = ?", wallet.ID).
Order("created_at DESC").
Limit(50).
Find(&transactions)

c.JSON(http.StatusOK, models.WalletResponse{
Balance:      wallet.Balance,
Transactions: transactions,
})
}

// CreateWalletTopupOrder godoc
// POST /api/v1/wallet/topup (protected)
// Creates a Razorpay order for a wallet top-up. The wallet is NOT credited
// here - only after VerifyWalletTopup confirms the payment signature. This
// replaces the old AddMoneyToWallet, which trusted the client-supplied
// amount directly with no gateway involved at all.
func CreateWalletTopupOrder(c *gin.Context) {
    userID := c.MustGet("user_id").(uint)

    var req models.CreateWalletTopupRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    rzpOrderID, err := utils.CreateRazorpayOrder(req.Amount, fmt.Sprintf("wallet_topup_user_%d", userID))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    topup := models.WalletTopup{
        UserID:          userID,
        RazorpayOrderID: rzpOrderID,
        Amount:          req.Amount,
        Currency:        "INR",
        Status:          models.WalletTopupStatusCreated,
    }
    if err := database.DB.Create(&topup).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record wallet top-up order"})
        return
    }

    c.JSON(http.StatusOK, models.CreateWalletTopupResponse{
        RazorpayOrderID: rzpOrderID,
        Amount:          int64(req.Amount*100 + 0.5),
        Currency:        "INR",
        KeyID:           config.AppConfig.RazorpayKeyID,
    })
}

// VerifyWalletTopup godoc
// POST /api/v1/wallet/topup/verify (protected)
// Verifies the Razorpay signature for a wallet top-up and, only on success,
// credits the wallet with the amount recorded when the order was created -
// never a client-supplied amount.
func VerifyWalletTopup(c *gin.Context) {
    userID := c.MustGet("user_id").(uint)

    var req models.VerifyWalletTopupRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var topup models.WalletTopup
    if err := database.DB.Where("razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, userID).First(&topup).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No wallet top-up order found for this user"})
        return
    }
    if topup.Status == models.WalletTopupStatusPaid {
        c.JSON(http.StatusBadRequest, gin.H{"error": "This top-up has already been credited"})
        return
    }

    if !utils.VerifyRazorpaySignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
        topup.Status = models.WalletTopupStatusFailed
        database.DB.Save(&topup)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Payment signature verification failed"})
        return
    }

    var wallet models.Wallet
    txErr := database.DB.Transaction(func(tx *gorm.DB) error {
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&topup, topup.ID).Error; err != nil {
            return err
        }
        if topup.Status == models.WalletTopupStatusPaid {
            return nil
        }
        topup.RazorpayPaymentID = req.RazorpayPaymentID
        topup.RazorpaySignature = req.RazorpaySignature
        topup.Status = models.WalletTopupStatusPaid
        if err := tx.Save(&topup).Error; err != nil {
            return err
        }
        return utils.CreditWallet(tx, userID, topup.Amount, models.WalletReasonAddMoney, "wallet_topup", &topup.ID, "Added via app")
    })
    if txErr != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Payment was verified but failed to credit wallet - contact support"})
        return
    }

    database.DB.Where("user_id = ?", userID).First(&wallet)
    c.JSON(http.StatusOK, gin.H{"wallet": wallet})
}


// ---------------------------------------------------------------------------
// Wallet - admin side
// ---------------------------------------------------------------------------

// AdminCreditWallet godoc
// POST /api/v1/admin/wallet/credit/:user_id (admin only)
// Manually credits a user's wallet — used for promotional cashback,
// goodwill credits, or correcting a support issue.
func AdminCreditWallet(c *gin.Context) {
adminID := c.MustGet("user_id").(uint)
userIDParam := c.Param("user_id")
userID64, err := strconv.ParseUint(userIDParam, 10, 64)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
return
}
userID := uint(userID64)

var user models.User
if err := database.DB.First(&user, userID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
return
}

var req models.AdminWalletCreditRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var wallet models.Wallet
txErr := database.DB.Transaction(func(tx *gorm.DB) error {
return utils.CreditWallet(tx, userID, req.Amount, models.WalletReasonAdminCredit, "admin", &adminID, req.Note)
})
if txErr != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to credit wallet"})
return
}

database.DB.Where("user_id = ?", userID).First(&wallet)
c.JSON(http.StatusOK, gin.H{"wallet": wallet})
}
