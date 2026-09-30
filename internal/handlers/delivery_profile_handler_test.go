package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/middleware"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestMain wires database.DB (and JWT config) to a real Postgres instance
// for this package's delivery-profile tests. Connection details come from
// the same DB_* env vars the app itself uses, falling back to the
// docker-compose defaults in this repo's .env.example. If no database is
// reachable, every test in this file is skipped rather than failed, so
// `go test ./...` still passes in environments without Postgres available -
// same approach as internal/services/delivery_assignment_test.go.
func TestMain(m *testing.M) {
	config.AppConfig = config.LoadConfig()

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"),
		envOr("DB_PASSWORD", "postgres"),
		envOr("TEST_DB_NAME", "ecommerce_test"),
		envOr("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Printf("[delivery_profile_handler_test] skipping package: could not connect to test database: %v\n", err)
		os.Exit(0)
	}

	// Also migrates the models needed by delivery_assignment_handler_test.go
	// (Phase 3, same package - only one TestMain is allowed per package),
	// plus Invoice/InvoiceItem/Account/LedgerEntry for invoicing/ledger
	// tests (e.g. BE-14 ConfirmDelivery tests). Deliberately NOT calling
	// database.AutoMigrate() here - that also runs production-only raw
	// SQL patches (e.g. a PostGIS "geometry" column) that fatal out on a
	// plain test DB without the PostGIS extension.
	if err := db.AutoMigrate(
		&models.DeliveryPartner{},
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.Address{},
		&models.Order{},
		&models.OrderItem{},
		&models.DeliveryNotification{},
		&models.Invoice{},
		&models.InvoiceItem{},
		&models.Account{},
		&models.LedgerEntry{},
		&models.RiderCODDeposit{},
		&models.RiderPayout{},
	); err != nil {
		fmt.Printf("[delivery_profile_handler_test] skipping package: migration failed: %v\n", err)
		os.Exit(0)
	}

	_ = db.Exec("TRUNCATE TABLE orders, order_items, delivery_partners, addresses, users, delivery_zones, delivery_notifications, rider_cod_deposits, rider_payouts RESTART IDENTITY CASCADE").Error

	database.DB = db

	// Minimal chart of accounts needed by ledger_posting.go for the COD
	// sale + refund + wallet-refund + discount paths exercised in tests.
	for _, acc := range []models.Account{
		{Code: "1001", Name: "Cash", Type: "asset", IsActive: true},
		{Code: "1002", Name: "Bank", Type: "asset", IsActive: true},
		{Code: "2002", Name: "GST Payable", Type: "liability", IsActive: true},
		{Code: "2004", Name: "Customer Refund Payable", Type: "liability", IsActive: true},
		{Code: "2005", Name: "Customer Wallet Liability", Type: "liability", IsActive: true},
		{Code: "4001", Name: "Product Sales", Type: "revenue", IsActive: true},
		{Code: "5002", Name: "Discount Given", Type: "expense", IsActive: true},
	} {
		var existing models.Account
		if err := db.Where("code = ?", acc.Code).First(&existing).Error; err != nil {
			db.Create(&acc)
		}
	}
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newDeliveryProfileTestRouter builds a router with just the Phase 2
// delivery profile/availability routes, guarded the same way as the real
// app (AuthMiddleware + DeliveryPartnerOnly).
func newDeliveryProfileTestRouter() *gin.Engine {
	r := gin.New()
	delivery := r.Group("/api/v1/delivery")
	delivery.GET("/profile", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), GetDeliveryProfile)
	delivery.PUT("/profile", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), UpdateDeliveryProfile)
	delivery.GET("/availability", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), GetDeliveryAvailability)
	delivery.PUT("/availability", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), UpdateDeliveryAvailability)
	return r
}

// seedDeliveryPartner creates a fresh delivery partner row for a test.
func seedDeliveryPartner(t *testing.T, phone string) models.DeliveryPartner {
	t.Helper()
	partner := models.DeliveryPartner{
		Name:          "Test Partner",
		Phone:         uniquePhone("91111"),
		VehicleNumber: "GJ01AB1234",
		IsActive:      true,
	}
	if err := database.DB.Create(&partner).Error; err != nil {
		t.Fatalf("failed to seed delivery partner: %v", err)
	}
	return partner
}

func deliveryPartnerToken(t *testing.T, partner models.DeliveryPartner) string {
	t.Helper()
	token, err := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func doRequest(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetDeliveryProfile_ReturnsOwnProfile(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100001")
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodGet, "/api/v1/delivery/profile", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["phone"] != partner.Phone {
		t.Errorf("expected phone %s, got %v", partner.Phone, resp["phone"])
	}
	if resp["account_status"] != "active" {
		t.Errorf("expected account_status 'active', got %v", resp["account_status"])
	}
	for _, sensitive := range []string{"password", "token", "otp"} {
		if _, ok := resp[sensitive]; ok {
			t.Errorf("response leaked sensitive field %q", sensitive)
		}
	}
}

func TestGetDeliveryProfile_CannotAccessAnotherPartnersProfile(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	self := seedDeliveryPartner(t, "9111100002")
	other := seedDeliveryPartner(t, "9111100003")
	token := deliveryPartnerToken(t, self)

	w := doRequest(r, http.MethodGet, "/api/v1/delivery/profile", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["phone"] == other.Phone {
		t.Errorf("partner was able to view another partner's profile (IDOR)")
	}
	if resp["phone"] != self.Phone {
		t.Errorf("expected own phone %s, got %v", self.Phone, resp["phone"])
	}
}

func TestUpdateDeliveryProfile_UpdatesOwnAllowedFields(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100004")
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodPut, "/api/v1/delivery/profile", token, gin.H{
		"name":           "Updated Name",
		"vehicle_number": "GJ05ZZ9999",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.DeliveryPartner
	if err := database.DB.First(&fresh, partner.ID).Error; err != nil {
		t.Fatalf("failed to reload partner: %v", err)
	}
	if fresh.Name != "Updated Name" || fresh.VehicleNumber != "GJ05ZZ9999" {
		t.Errorf("profile fields were not updated: %+v", fresh)
	}
}

// TestUpdateDeliveryProfile_IgnoresProtectedFields sends role/status/id/
// warehouse fields in the body (as an attacker might) and verifies none of
// them influence the stored record - only name/vehicle_number are ever
// bound by UpdateDeliveryProfileRequest.
func TestUpdateDeliveryProfile_IgnoresProtectedFields(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100005")
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodPut, "/api/v1/delivery/profile", token, gin.H{
		"name":      "Still Allowed",
		"is_active": false,
		"role":      "admin",
		"id":        9999,
		"warehouse": "Warehouse X",
		"is_online": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.DeliveryPartner
	if err := database.DB.First(&fresh, partner.ID).Error; err != nil {
		t.Fatalf("failed to reload partner: %v", err)
	}
	if !fresh.IsActive {
		t.Errorf("is_active was modified via profile update - protected field leaked through")
	}
	if fresh.IsOnline {
		t.Errorf("is_online was modified via profile update - protected field leaked through")
	}
	if fresh.ID != partner.ID {
		t.Errorf("id was modified via profile update")
	}
}

func TestUpdateDeliveryProfile_RejectsEmptyName(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100006")
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodPut, "/api/v1/delivery/profile", token, gin.H{"name": ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDeliveryAvailability_DefaultsOffline(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100007")
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodGet, "/api/v1/delivery/availability", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "offline" {
		t.Errorf("expected default status 'offline', got %v", resp["status"])
	}
}

func TestUpdateDeliveryAvailability_SetOnline(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100008")
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodPut, "/api/v1/delivery/availability", token, gin.H{"status": "online"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "online" || resp["is_online"] != true {
		t.Errorf("expected online status in response, got %v", resp)
	}

	var fresh models.DeliveryPartner
	database.DB.First(&fresh, partner.ID)
	if !fresh.IsOnline {
		t.Errorf("expected is_online=true to be persisted")
	}
}

func TestUpdateDeliveryAvailability_SetOffline(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100009")
	database.DB.Model(&partner).Update("is_online", true)
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodPut, "/api/v1/delivery/availability", token, gin.H{"status": "offline"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.DeliveryPartner
	database.DB.First(&fresh, partner.ID)
	if fresh.IsOnline {
		t.Errorf("expected is_online=false to be persisted")
	}
}

func TestUpdateDeliveryAvailability_InvalidValueRejected(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100010")
	token := deliveryPartnerToken(t, partner)

	w := doRequest(r, http.MethodPut, "/api/v1/delivery/availability", token, gin.H{"status": "busy"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.DeliveryPartner
	database.DB.First(&fresh, partner.ID)
	if fresh.IsOnline {
		t.Errorf("invalid status should not have changed is_online")
	}
}

func TestDeliveryProfile_UnauthorizedWithoutToken(t *testing.T) {
	r := newDeliveryProfileTestRouter()

	w := doRequest(r, http.MethodGet, "/api/v1/delivery/profile", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeliveryProfile_InvalidTokenRejected(t *testing.T) {
	r := newDeliveryProfileTestRouter()

	w := doRequest(r, http.MethodGet, "/api/v1/delivery/profile", "not-a-real-token", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDeliveryProfile_NonDeliveryRoleRejected verifies a valid token for a
// different role (e.g. "customer") is blocked by DeliveryPartnerOnly, even
// though AuthMiddleware alone would accept it.
func TestDeliveryProfile_NonDeliveryRoleRejected(t *testing.T) {
	r := newDeliveryProfileTestRouter()

	token, err := utils.GenerateJWT(1, "9000000000", "customer")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	w := doRequest(r, http.MethodGet, "/api/v1/delivery/profile", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-delivery role, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeliveryProfile_NotFoundForDeletedPartner(t *testing.T) {
	r := newDeliveryProfileTestRouter()
	partner := seedDeliveryPartner(t, "9111100011")
	token := deliveryPartnerToken(t, partner)

	database.DB.Delete(&partner)

	w := doRequest(r, http.MethodGet, "/api/v1/delivery/profile", token, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a deleted partner, got %d: %s", w.Code, w.Body.String())
	}
}
