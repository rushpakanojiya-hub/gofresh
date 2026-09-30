package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
)

const razorpayOrdersURL = "https://api.razorpay.com/v1/orders"

type razorpayOrderResponse struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

// CreateRazorpayOrder calls Razorpay's Orders API (https://razorpay.com/docs/api/orders/)
// to create an order for the given amount in rupees, and returns the
// Razorpay order ID to hand to the frontend Checkout widget.
// Uses net/http + Basic Auth directly — no razorpay-go SDK dependency needed.
func CreateRazorpayOrder(amountRupees float64, receipt string) (string, error) {
	cfg := config.AppConfig
	if cfg.RazorpayKeyID == "" || cfg.RazorpayKeySecret == "" {
		return "", errors.New("Razorpay is not configured on the server (missing RAZORPAY_KEY_ID/RAZORPAY_KEY_SECRET)")
	}

	amountPaise := int64(amountRupees*100 + 0.5) // round to nearest paisa

	payload, err := json.Marshal(map[string]interface{}{
		"amount":   amountPaise,
		"currency": "INR",
		"receipt":  receipt,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, razorpayOrdersURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(cfg.RazorpayKeyID, cfg.RazorpayKeySecret)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach Razorpay: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("razorpay order creation failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var rzpOrder razorpayOrderResponse
	if err := json.Unmarshal(respBody, &rzpOrder); err != nil {
		return "", fmt.Errorf("failed to parse Razorpay response: %w", err)
	}

	return rzpOrder.ID, nil
}

// VerifyRazorpaySignature re-derives the HMAC-SHA256 signature Razorpay
// documents for Checkout verification — hmac_sha256(order_id + "|" + payment_id,
// key_secret) — and compares it against what the frontend sent back.
// See: https://razorpay.com/docs/payments/server-integration/php/payment-gateway/build-integration/#step-5-verify-payment-signature
func VerifyRazorpaySignature(razorpayOrderID, razorpayPaymentID, signature string) bool {
	cfg := config.AppConfig
	payload := razorpayOrderID + "|" + razorpayPaymentID

	mac := hmac.New(sha256.New, []byte(cfg.RazorpayKeySecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// VerifyRazorpayWebhookSignature verifies the X-Razorpay-Signature header against the raw body.
func VerifyRazorpayWebhookSignature(body []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
