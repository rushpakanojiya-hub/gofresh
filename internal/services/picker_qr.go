package services

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PickerQRTTL is how long a picker handover QR stays scannable.
const PickerQRTTL = 2 * time.Hour

var ErrPickerQRInvalid = errors.New("This QR is invalid or expired")

// GeneratePickerQRToken returns a signed token the picker app shows as a QR.
func GeneratePickerQRToken(orderID uint) string {
	exp := time.Now().Add(PickerQRTTL).Unix()
	payload := fmt.Sprintf("pk1:%d:%d", orderID, exp)
	return fmt.Sprintf("PK1.%d.%d.%s", orderID, exp, signStoreCheckin(payload))
}

// ParsePickerQRToken verifies signature and expiry, and returns the order id.
func ParsePickerQRToken(token string) (uint, error) {
	parts := strings.SplitN(strings.TrimSpace(token), ".", 4)
	if len(parts) != 4 || parts[0] != "PK1" {
		return 0, ErrPickerQRInvalid
	}
	oid, e1 := strconv.ParseUint(parts[1], 10, 64)
	exp, e2 := strconv.ParseInt(parts[2], 10, 64)
	if e1 != nil || e2 != nil {
		return 0, ErrPickerQRInvalid
	}
	want := signStoreCheckin(fmt.Sprintf("pk1:%d:%d", oid, exp))
	if !hmac.Equal([]byte(want), []byte(parts[3])) || time.Now().Unix() > exp {
		return 0, ErrPickerQRInvalid
	}
	return uint(oid), nil
}
