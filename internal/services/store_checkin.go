package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

const (
	// StoreQRTTL is how long one generated store QR stays scannable.
	StoreQRTTL = 60 * time.Second
	// StoreCheckinMaxAge is how long a check-in stays valid at most.
	StoreCheckinMaxAge = 12 * time.Hour
	// StoreCheckinRadiusM is the max distance (metres) from the store to check in.
	StoreCheckinRadiusM = 300.0
)

var (
	ErrCheckinInvalidToken = errors.New("QR invalid ya expire ho gaya hai, store screen par naya QR scan karo")
	ErrCheckinWrongStore   = errors.New("ye QR doosre store ka hai")
	ErrCheckinNoLocation   = errors.New("aapki location nahi mili, location on karke dobara try karo")
	ErrCheckinTooFar       = errors.New("aap store se bahut door ho")
	ErrCheckinNotFound     = errors.New("partner ya store nahi mila")
)

func storeCheckinKey() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "default_secret_change_me"
	}
	m := hmac.New(sha256.New, []byte(s))
	m.Write([]byte("store-checkin-v1"))
	return m.Sum(nil)
}

func signStoreCheckin(payload string) string {
	m := hmac.New(sha256.New, storeCheckinKey())
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// GenerateStoreCheckinToken returns a short-lived signed token for the store QR.
func GenerateStoreCheckinToken(warehouseID uint) (string, time.Time) {
	exp := time.Now().Add(StoreQRTTL)
	payload := fmt.Sprintf("%d.%d", warehouseID, exp.Unix())
	return "SC1." + payload + "." + signStoreCheckin(payload), exp
}

func parseStoreCheckinToken(token string) (uint, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 4 || parts[0] != "SC1" {
		return 0, ErrCheckinInvalidToken
	}
	payload := parts[1] + "." + parts[2]
	if !hmac.Equal([]byte(signStoreCheckin(payload)), []byte(parts[3])) {
		return 0, ErrCheckinInvalidToken
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return 0, ErrCheckinInvalidToken
	}
	wid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, ErrCheckinInvalidToken
	}
	return uint(wid), nil
}

// CheckInPartner verifies the scanned token and records the partner's check-in.
func CheckInPartner(partnerID uint, token string, lat, lng *float64) (*models.Warehouse, error) {
	wid, err := parseStoreCheckinToken(token)
	if err != nil {
		return nil, err
	}
	var p models.DeliveryPartner
	if err := database.DB.First(&p, partnerID).Error; err != nil {
		return nil, ErrCheckinNotFound
	}
	if p.WarehouseID != nil && *p.WarehouseID != wid {
		return nil, ErrCheckinWrongStore
	}
	var wh models.Warehouse
	if err := database.DB.Where("id = ? AND is_active = ?", wid, true).First(&wh).Error; err != nil {
		return nil, ErrCheckinNotFound
	}
	if wh.Lat != 0 || wh.Lng != 0 {
		pl, pg := lat, lng
		if pl == nil || pg == nil {
			pl, pg = p.CurrentLat, p.CurrentLng
		}
		if pl == nil || pg == nil {
			return nil, ErrCheckinNoLocation
		}
		if haversineKm(*pl, *pg, wh.Lat, wh.Lng)*1000 > StoreCheckinRadiusM {
			return nil, ErrCheckinTooFar
		}
	}
	err = database.DB.Model(&models.DeliveryPartner{}).
		Where("id = ?", partnerID).
		Updates(map[string]interface{}{
			"checked_in_warehouse_id": wid,
			"checked_in_at":           time.Now(),
		}).Error
	if err != nil {
		return nil, err
	}
	return &wh, nil
}

// ClearPartnerCheckin removes the partner's check-in (used when they go offline).
func ClearPartnerCheckin(partnerID uint) error {
	return database.DB.Model(&models.DeliveryPartner{}).
		Where("id = ?", partnerID).
		Updates(map[string]interface{}{
			"checked_in_warehouse_id": nil,
			"checked_in_at":           nil,
		}).Error
}
