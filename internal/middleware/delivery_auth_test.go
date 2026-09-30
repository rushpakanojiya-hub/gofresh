package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestMain wires database.DB to a real Postgres instance and a JWT secret,
// mirroring internal/services/delivery_assignment_test.go. If no database is
// reachable, every test in this package is skipped (not failed) so
// `go test ./...` still passes in environments without Postgres available.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	config.AppConfig = &config.Config{
		JWTSecret:      envOr("JWT_SECRET", "test_secret_do_not_use_in_prod"),
		JWTExpiryHours: "72",
	}

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
		fmt.Printf("[delivery_auth_test] skipping package: could not connect to test database: %v\n", err)
		os.Exit(0)
	}

	if err := db.AutoMigrate(&models.DeliveryPartner{}); err != nil {
		fmt.Printf("[delivery_auth_test] skipping package: migration failed: %v\n", err)
		os.Exit(0)
	}

	database.DB = db
	os.Exit(m.Run())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func resetDeliveryPartnersTable(t *testing.T) {
	t.Helper()
	if err := database.DB.Exec("TRUNCATE TABLE delivery_partners RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to reset delivery_partners table: %v", err)
	}
}

func seedPartner(t *testing.T, phone string, active bool) models.DeliveryPartner {
	t.Helper()
	partner := models.DeliveryPartner{Name: "Test Partner", Phone: phone, IsActive: active}
	if err := database.DB.Select("*").Create(&partner).Error; err != nil {
		t.Fatalf("failed to seed delivery partner: %v", err)
	}
	return partner
}

// newAuthedRequest builds a request carrying the given bearer token and runs
// it through the given middleware chain, returning the response.
func runChain(token string, handlers ...gin.HandlerFunc) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	r.GET("/protected", append(handlers, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})...)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c.Request = req
	r.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// AuthMiddleware: token presence / validity (shared entry point for delivery)
// ---------------------------------------------------------------------------

func TestAuthMiddleware_MissingToken_Returns401(t *testing.T) {
	w := runChain("", AuthMiddleware())
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	w := runChain("not-a-real-jwt", AuthMiddleware())
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_ExpiredToken_Returns401(t *testing.T) {
	// Build a token that expired one hour ago.
	expired, err := utils.GenerateJWTWithExpiry(1, "9999999999", "delivery_partner", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("failed to build expired token: %v", err)
	}
	w := runChain(expired, AuthMiddleware())
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken_SetsRoleAndProceeds(t *testing.T) {
	resetDeliveryPartnersTable(t)
	partner := seedPartner(t, "9000011111", true)
	token, err := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	w := runChain(token, AuthMiddleware())
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token, got %d, body=%s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DeliveryPartnerOnly: role separation + live active-status check
// ---------------------------------------------------------------------------

func TestDeliveryPartnerOnly_ValidActivePartner_Allowed(t *testing.T) {
	resetDeliveryPartnersTable(t)
	partner := seedPartner(t, "9000022222", true)
	token, _ := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")

	w := runChain(token, AuthMiddleware(), DeliveryPartnerOnly())
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for active delivery partner, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestDeliveryPartnerOnly_DeactivatedPartner_Forbidden(t *testing.T) {
	resetDeliveryPartnersTable(t)
	partner := seedPartner(t, "9000033333", false) // inactive
	token, _ := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")

	w := runChain(token, AuthMiddleware(), DeliveryPartnerOnly())
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for deactivated delivery partner, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestDeliveryPartnerOnly_DeletedPartner_Rejected(t *testing.T) {
	resetDeliveryPartnersTable(t)
	partner := seedPartner(t, "9000044444", true)
	token, _ := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")

	if err := database.DB.Delete(&partner).Error; err != nil {
		t.Fatalf("failed to soft-delete partner: %v", err)
	}

	w := runChain(token, AuthMiddleware(), DeliveryPartnerOnly())
	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 for a token belonging to a deleted partner, got %d", w.Code)
	}
}

func TestDeliveryPartnerOnly_AdminRole_Forbidden(t *testing.T) {
	// An admin token must never satisfy the delivery-partner check (BOLA/role
	// confusion guard): admin APIs and delivery APIs use disjoint roles.
	token, _ := utils.GenerateJWT(1, "9000000001", "admin")

	w := runChain(token, AuthMiddleware(), DeliveryPartnerOnly())
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for admin role on delivery-only route, got %d", w.Code)
	}
}

func TestDeliveryPartnerOnly_WarehouseStaffRole_Forbidden(t *testing.T) {
	token, _ := utils.GenerateJWT(1, "9000000002", "warehouse_staff")

	w := runChain(token, AuthMiddleware(), DeliveryPartnerOnly())
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for warehouse_staff role on delivery-only route, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Admin/Warehouse-only middleware must reject delivery_partner tokens
// (verifies delivery boys cannot reach admin or warehouse endpoints).
// ---------------------------------------------------------------------------

func TestAdminOnly_DeliveryPartnerRole_Forbidden(t *testing.T) {
	resetDeliveryPartnersTable(t)
	partner := seedPartner(t, "9000055555", true)
	token, _ := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")

	w := runChain(token, AuthMiddleware(), AdminOnly())
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for delivery_partner on admin-only route, got %d", w.Code)
	}
}

func TestWarehouseStaffOnly_DeliveryPartnerRole_Forbidden(t *testing.T) {
	resetDeliveryPartnersTable(t)
	partner := seedPartner(t, "9000066666", true)
	token, _ := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")

	w := runChain(token, AuthMiddleware(), WarehouseStaffOnly())
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for delivery_partner on warehouse-only route, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// CustomerOnly middleware must reject non-customer tokens - this is the
// direct regression guard for the BOLA/role-confusion vulnerability where
// DeliveryPartner and User rows share overlapping auto-increment IDs
// (e.g. delivery partner ID 1 and customer ID 1 both exist), so any
// customer-only route MUST verify role == "customer" and not just trust
// the numeric user_id claim.
// ---------------------------------------------------------------------------

func TestCustomerOnly_DeliveryPartnerRole_Forbidden(t *testing.T) {
	resetDeliveryPartnersTable(t)
	partner := seedPartner(t, "9000077777", true)
	token, _ := utils.GenerateJWT(partner.ID, partner.Phone, "delivery_partner")

	w := runChain(token, AuthMiddleware(), CustomerOnly())
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for delivery_partner role on customer-only route, got %d", w.Code)
	}
}

func TestCustomerOnly_AdminRole_Forbidden(t *testing.T) {
	token, _ := utils.GenerateJWT(1, "9000000003", "admin")

	w := runChain(token, AuthMiddleware(), CustomerOnly())
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for admin role on customer-only route, got %d", w.Code)
	}
}

func TestCustomerOnly_ValidCustomerRole_Allowed(t *testing.T) {
	token, _ := utils.GenerateJWT(1, "9000088888", "customer")

	w := runChain(token, AuthMiddleware(), CustomerOnly())
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid customer role, got %d", w.Code)
	}
}
