package handlers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const walletReasonGiftCard = "gift_card"

func giftCardUserID(c *gin.Context) (uint, bool) {
	v, _ := c.Get("user_id")
	switch id := v.(type) {
	case uint:
		return id, true
	case int:
		return uint(id), true
	case int64:
		return uint(id), true
	case float64:
		return uint(id), true
	}
	return 0, false
}

func newGiftCardCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 12)
	// BUG-42: never fall back to a predictable code if the RNG fails.
	if _, err := rand.Read(b); err != nil {
		panic("gift card code generation failed: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return "GC" + string(b)
}

// RedeemGiftCard POST /api/v1/gift-cards/redeem
func RedeemGiftCard(c *gin.Context) {
	uid, ok := giftCardUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gift card code is required"})
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	status, msg := http.StatusOK, ""
	var card models.GiftCard
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&card).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status, msg = http.StatusNotFound, "Invalid gift card code"
			}
			return err
		}
		if card.RedeemedByUserID != nil {
			status, msg = http.StatusConflict, "This gift card has already been redeemed"
			return errors.New("already redeemed")
		}
		now := time.Now()
		if now.After(card.ExpiresAt) {
			status, msg = http.StatusGone, "This gift card has expired"
			return errors.New("expired")
		}
		card.RedeemedByUserID = &uid
		card.RedeemedAt = &now
		if err := tx.Save(&card).Error; err != nil {
			return err
		}
		refID := card.ID
		if err := utils.CreditWallet(tx, uid, card.Amount, walletReasonGiftCard, "gift_card", &refID, "Gift card "+card.Code); err != nil {
			return err
		}
		return services.PostWalletCreditLedgerTx(tx, "5006", "gift_card", &refID, fmt.Sprintf("WALLET-GIFTCARD-%d", card.ID), card.Amount, "Gift card redeemed: "+card.Code)
	})
	if err != nil {
		if msg == "" {
			status, msg = http.StatusInternalServerError, "Failed to redeem gift card"
		}
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          fmt.Sprint(card.ID),
		"card_number": card.Code,
		"balance":     0, // BUG-42: redeemed card has no balance left,
		"expiry_date": card.ExpiresAt.Format(time.RFC3339),
		"status":      "redeemed",
	})
}

// AdminCreateGiftCard POST /api/v1/admin/gift-cards
func AdminCreateGiftCard(c *gin.Context) {
	var req struct {
		Amount    float64 `json:"amount" binding:"required,gt=0"`
		Code      string  `json:"code"`
		ValidDays int     `json:"valid_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ValidDays <= 0 {
		req.ValidDays = 365
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		code = newGiftCardCode()
	}
	card := models.GiftCard{Code: code, Amount: req.Amount, ExpiresAt: time.Now().AddDate(0, 0, req.ValidDays)}
	if err := database.DB.Create(&card).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Could not create gift card (code may already exist)"})
		return
	}
	c.JSON(http.StatusCreated, card)
}
