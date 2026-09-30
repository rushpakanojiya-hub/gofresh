package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/middleware"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ---------------------------------------------------------------------------
// Phase 3: Delivery Order Assignment - test router + seed helpers
// ---------------------------------------------------------------------------

// newAssignmentTestRouter wires up exactly the Phase 3 routes, guarded the
// same way as the real app: admin routes behind AuthMiddleware+AdminOnly,
// delivery-partner routes behind AuthMiddleware+DeliveryPartnerOnly.
func newAssignmentTestRouter() *gin.Engine {
	r := gin.New()

	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.AuthMiddleware(), middleware.AdminOnly())
	admin.PUT("/orders/:id/assign-delivery", AssignDeliveryPartner)
	admin.DELETE("/delivery-partners/:id", DeleteDeliveryPartner)

	delivery := r.Group("/api/v1/delivery")
	delivery.GET("/orders", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), GetMyDeliveries)
	delivery.PUT("/orders/:id/accept", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), AcceptAssignment)
	delivery.PUT("/orders/:id/reject", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), RejectAssignment)

	return r
}

var assignmentSeedSeq int

// uniquePhone returns a fresh 10-digit phone number for each call, so
// parallel/sequential tests never collide on the DeliveryPartner/User
// uniqueIndex on phone.
func uniquePhone(prefix string) string {
	assignmentSeedSeq++
	n := (time.Now().UnixNano()/1000 + int64(assignmentSeedSeq)) % 1000000000
	return fmt.Sprintf("9%09d", n)
}

func seedAssignPartner(t *testing.T, online bool) models.DeliveryPartner {
	t.Helper()
	partner := models.DeliveryPartner{
		Name:     "Assignment Test Partner",
		Phone:    uniquePhone("92220"),
		IsActive: true,
		IsOnline: online,
	}
	if err := database.DB.Create(&partner).Error; err != nil {
		t.Fatalf("failed to seed delivery partner: %v", err)
	}
	return partner
}

func seedAssignUser(t *testing.T) models.User {
	t.Helper()
	user := models.User{
		Name:  "Assignment Test Customer",
		Phone: uniquePhone("93330"),
		Role:  "customer",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func seedAssignAddress(t *testing.T, userID uint) models.Address {
	t.Helper()
	lat, lng := 23.0225, 72.5714
	addr := models.Address{
		UserID:   userID,
		FullName: "Test Customer",
		Phone:    "9876500000",
		Line1:    "123 Test Street",
		City:     "Ahmedabad",
		State:    "Gujarat",
		Pincode:  "380001",
		Lat:      &lat,
		Lng:      &lng,
	}
	if err := database.DB.Create(&addr).Error; err != nil {
		t.Fatalf("failed to seed address: %v", err)
	}
	return addr
}

// seedAssignOrder creates an order in the given status with no delivery
// partner assigned yet, ready to be handed to AssignDeliveryPartner.
func seedAssignOrder(t *testing.T, status string) models.Order {
	t.Helper()
	user := seedAssignUser(t)
	addr := seedAssignAddress(t, user.ID)
	order := models.Order{
		UserID:        user.ID,
		AddressID:     addr.ID,
		ItemsAmount:   500,
		TotalAmount:   550,
		Status:        status,
		PaymentMethod: models.PaymentMethodCOD,
		PaymentStatus: models.OrderPaymentStatusPending,
	}
	if err := database.DB.Create(&order).Error; err != nil {
		t.Fatalf("failed to seed order: %v", err)
	}
	database.DB.Preload("Address").First(&order, order.ID)
	return order
}

func adminAssignToken(t *testing.T) string {
	t.Helper()
	token, err := utils.GenerateJWT(1, "9000000001", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}
	return token
}

// assignOrder is a small helper that calls the admin assign-delivery
// endpoint and returns the response recorder.
func assignOrder(r *gin.Engine, orderID uint, partnerID uint) *jsonResponse {
	w := doRequest(r, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/orders/%d/assign-delivery", orderID),
		adminAssignTokenCache,
		gin.H{"delivery_partner_id": partnerID},
	)
	return &jsonResponse{w.Code, w.Body.Bytes()}
}

type jsonResponse struct {
	Code int
	Body []byte
}

func (j *jsonResponse) asMap() map[string]interface{} {
	var m map[string]interface{}
	json.Unmarshal(j.Body, &m)
	return m
}

// adminAssignTokenCache is set per-test via setAdminToken to keep the
// assignOrder helper signature simple; see TestMain-adjacent tests below
// for direct doRequest usage where more control is needed.
var adminAssignTokenCache string

// ---------------------------------------------------------------------------
// 1. Assign order to delivery boy
// ---------------------------------------------------------------------------

func TestAssignDeliveryPartner_Success(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true) // online
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	resp := assignOrder(r, order.ID, partner.ID)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body)
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryPartnerID == nil || *fresh.DeliveryPartnerID != partner.ID {
		t.Errorf("expected order to be assigned to partner %d, got %+v", partner.ID, fresh.DeliveryPartnerID)
	}
	if fresh.DeliveryAssignmentStatus == nil || *fresh.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusAssigned {
		t.Errorf("expected assignment status 'assigned', got %v", fresh.DeliveryAssignmentStatus)
	}
}

// ---------------------------------------------------------------------------
// 2. Assign order to offline delivery boy must fail
// ---------------------------------------------------------------------------

func TestAssignDeliveryPartner_OfflinePartnerRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, false) // offline
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	resp := assignOrder(r, order.ID, partner.ID)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for offline partner, got %d: %s", resp.Code, resp.Body)
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryPartnerID != nil {
		t.Errorf("order should remain unassigned when partner is offline, got %+v", fresh.DeliveryPartnerID)
	}
}

// ---------------------------------------------------------------------------
// 3 & 4. Get assigned orders / delivery boy sees only own assignments
// ---------------------------------------------------------------------------

func TestGetMyDeliveries_OnlyReturnsOwnAssignments(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)

	partnerA := seedAssignPartner(t, true)
	partnerB := seedAssignPartner(t, true)
	orderA := seedAssignOrder(t, models.OrderStatusConfirmed)
	orderB := seedAssignOrder(t, models.OrderStatusConfirmed)

	if resp := assignOrder(r, orderA.ID, partnerA.ID); resp.Code != http.StatusOK {
		t.Fatalf("setup: failed to assign orderA: %d %s", resp.Code, resp.Body)
	}
	if resp := assignOrder(r, orderB.ID, partnerB.ID); resp.Code != http.StatusOK {
		t.Fatalf("setup: failed to assign orderB: %d %s", resp.Code, resp.Body)
	}

	tokenA := deliveryPartnerToken(t, partnerA)
	w := doRequest(r, http.MethodGet, "/api/v1/delivery/orders", tokenA, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Orders []map[string]interface{} `json:"orders"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Orders) != 1 {
		t.Fatalf("expected exactly 1 order for partnerA, got %d", len(resp.Orders))
	}
	gotID := uint(resp.Orders[0]["order_id"].(float64))
	if gotID != orderA.ID {
		t.Errorf("expected order %d, got %d", orderA.ID, gotID)
	}
	for _, o := range resp.Orders {
		if uint(o["order_id"].(float64)) == orderB.ID {
			t.Errorf("partnerA was able to see partnerB's order (IDOR)")
		}
	}
}

// ---------------------------------------------------------------------------
// 5. Accept own assignment
// ---------------------------------------------------------------------------

func TestAcceptAssignment_OwnAssignmentSucceeds(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)
	assignOrder(r, order.ID, partner.ID)

	token := deliveryPartnerToken(t, partner)
	w := doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/accept", order.ID), token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryAssignmentStatus == nil || *fresh.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusAccepted {
		t.Errorf("expected assignment status 'accepted', got %v", fresh.DeliveryAssignmentStatus)
	}
}

// ---------------------------------------------------------------------------
// 6. Accept another delivery boy's assignment must fail
// ---------------------------------------------------------------------------

func TestAcceptAssignment_AnotherPartnersOrderRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	owner := seedAssignPartner(t, true)
	intruder := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)
	assignOrder(r, order.ID, owner.ID)

	intruderToken := deliveryPartnerToken(t, intruder)
	w := doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/accept", order.ID), intruderToken, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no IDOR leak) for another partner's order, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryAssignmentStatus == nil || *fresh.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusAssigned {
		t.Errorf("assignment status must remain 'assigned' after a rejected intrusion attempt, got %v", fresh.DeliveryAssignmentStatus)
	}
}

// ---------------------------------------------------------------------------
// 7. Reject own assignment
// ---------------------------------------------------------------------------

func TestRejectAssignment_OwnAssignmentSucceeds(t *testing.T) {
	services.AutoReassignOnReject = false
	defer func() { services.AutoReassignOnReject = true }()
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)
	assignOrder(r, order.ID, partner.ID)

	token := deliveryPartnerToken(t, partner)
	w := doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/reject", order.ID), token, gin.H{"reason": "Vehicle breakdown"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryAssignmentStatus == nil || *fresh.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusRejected {
		t.Errorf("expected assignment status 'rejected', got %v", fresh.DeliveryAssignmentStatus)
	}
	if fresh.DeliveryRejectionReason == nil || *fresh.DeliveryRejectionReason != "Vehicle breakdown" {
		t.Errorf("expected rejection reason to be stored, got %v", fresh.DeliveryRejectionReason)
	}
	// The order itself must not be deleted.
	var count int64
	database.DB.Model(&models.Order{}).Where("id = ?", order.ID).Count(&count)
	if count != 1 {
		t.Errorf("order row must not be deleted on rejection")
	}
}

// ---------------------------------------------------------------------------
// 8. Reject another delivery boy's assignment must fail
// ---------------------------------------------------------------------------

func TestRejectAssignment_AnotherPartnersOrderRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	owner := seedAssignPartner(t, true)
	intruder := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)
	assignOrder(r, order.ID, owner.ID)

	intruderToken := deliveryPartnerToken(t, intruder)
	w := doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/reject", order.ID), intruderToken, gin.H{"reason": "not mine"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for another partner's order, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 9. Accept already accepted order must fail
// ---------------------------------------------------------------------------

func TestAcceptAssignment_AlreadyAcceptedRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)
	assignOrder(r, order.ID, partner.ID)

	token := deliveryPartnerToken(t, partner)
	w1 := doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/accept", order.ID), token, nil)
	if w1.Code != http.StatusOK {
		t.Fatalf("setup: first accept should succeed, got %d: %s", w1.Code, w1.Body.String())
	}

	w2 := doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/accept", order.ID), token, nil)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when accepting an already-accepted order, got %d: %s", w2.Code, w2.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 10. Assign same order twice must fail safely (no double active assignment)
// ---------------------------------------------------------------------------

func TestAssignDeliveryPartner_DoubleAssignmentRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partnerA := seedAssignPartner(t, true)
	partnerB := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	resp1 := assignOrder(r, order.ID, partnerA.ID)
	if resp1.Code != http.StatusOK {
		t.Fatalf("setup: first assignment should succeed, got %d: %s", resp1.Code, resp1.Body)
	}

	// Second assignment attempt (different partner) while the first is
	// still ASSIGNED/pending must be rejected - an order can never end up
	// with two active delivery partners.
	resp2 := assignOrder(r, order.ID, partnerB.ID)
	if resp2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a second assignment attempt, got %d: %s", resp2.Code, resp2.Body)
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryPartnerID == nil || *fresh.DeliveryPartnerID != partnerA.ID {
		t.Errorf("order must remain assigned to the original partner (A), got %+v", fresh.DeliveryPartnerID)
	}
}

// ---------------------------------------------------------------------------
// 11. Unauthorized assignment access
// ---------------------------------------------------------------------------

func TestGetMyDeliveries_UnauthorizedWithoutToken(t *testing.T) {
	r := newAssignmentTestRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/delivery/orders", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAcceptAssignment_UnauthorizedWithoutToken(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)
	assignOrder(r, order.ID, partner.ID)

	w := doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/accept", order.ID), "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 12. Role authorization
// ---------------------------------------------------------------------------

func TestAssignDeliveryPartner_NonAdminRoleRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	partner := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	// A valid delivery-partner token (not admin) must not be able to
	// reach the admin-only assignment endpoint.
	token := deliveryPartnerToken(t, partner)
	w := doRequest(r, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/orders/%d/assign-delivery", order.ID),
		token, gin.H{"delivery_partner_id": partner.ID})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin role, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetMyDeliveries_NonDeliveryRoleRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	token, err := utils.GenerateJWT(1, "9000000002", "admin")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	w := doRequest(r, http.MethodGet, "/api/v1/delivery/orders", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-delivery-partner role, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Extra: invalid order status transitions
// ---------------------------------------------------------------------------

func TestAssignDeliveryPartner_RejectsInvalidOrderStatus(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusPending) // not confirmed/shipped

	resp := assignOrder(r, order.ID, partner.ID)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a pending order, got %d: %s", resp.Code, resp.Body)
	}
}

func TestAssignDeliveryPartner_UnknownOrderReturns404(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)

	resp := assignOrder(r, 9_999_999, partner.ID)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown order, got %d: %s", resp.Code, resp.Body)
	}
}

func TestAssignDeliveryPartner_UnknownPartnerReturns404(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	resp := assignOrder(r, order.ID, 9_999_999)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown partner, got %d: %s", resp.Code, resp.Body)
	}
}

// TestAssignDeliveryPartner_ReassignAfterRejectionAllowed verifies that,
// unlike the "already active" double-assignment case above, an order whose
// partner rejected it CAN be re-assigned to a new partner - this is the one
// legitimate re-assignment path this phase allows (no auto-reassignment,
// but an admin/dispatcher can do it manually).
func TestAssignDeliveryPartner_ReassignAfterRejectionAllowed(t *testing.T) {
	services.AutoReassignOnReject = false
	defer func() { services.AutoReassignOnReject = true }()
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partnerA := seedAssignPartner(t, true)
	partnerB := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	assignOrder(r, order.ID, partnerA.ID)
	tokenA := deliveryPartnerToken(t, partnerA)
	doRequest(r, http.MethodPut, fmt.Sprintf("/api/v1/delivery/orders/%d/reject", order.ID), tokenA, gin.H{"reason": "too far"})

	resp := assignOrder(r, order.ID, partnerB.ID)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected re-assignment after rejection to succeed, got %d: %s", resp.Code, resp.Body)
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryPartnerID == nil || *fresh.DeliveryPartnerID != partnerB.ID {
		t.Errorf("expected order reassigned to partnerB, got %+v", fresh.DeliveryPartnerID)
	}
	if fresh.DeliveryAssignmentStatus == nil || *fresh.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusAssigned {
		t.Errorf("expected fresh 'assigned' status after reassignment, got %v", fresh.DeliveryAssignmentStatus)
	}
}

func TestAssignDeliveryPartner_ReassignAfterExpiryAllowed(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partnerA := seedAssignPartner(t, true)
	partnerB := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	assignOrder(r, order.ID, partnerA.ID)

	// Simulate the periodic expiry sweep (services.ExpireStaleAssignments)
	// marking this assignment as expired, rather than the partner
	// explicitly rejecting it.
	database.DB.Model(&models.Order{}).Where("id = ?", order.ID).
		Update("delivery_assignment_status", models.DeliveryAssignmentStatusExpired)

	resp := assignOrder(r, order.ID, partnerB.ID)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected re-assignment after expiry to succeed, got %d: %s", resp.Code, resp.Body)
	}

	var fresh models.Order
	database.DB.First(&fresh, order.ID)
	if fresh.DeliveryPartnerID == nil || *fresh.DeliveryPartnerID != partnerB.ID {
		t.Errorf("expected order reassigned to partnerB, got %+v", fresh.DeliveryPartnerID)
	}
	if fresh.DeliveryAssignmentStatus == nil || *fresh.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusAssigned {
		t.Errorf("expected fresh 'assigned' status after reassignment, got %v", fresh.DeliveryAssignmentStatus)
	}
}

func TestDeleteDeliveryPartner_WithActiveOrderRejected(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)
	order := seedAssignOrder(t, models.OrderStatusConfirmed)

	assignOrder(r, order.ID, partner.ID)

	w := doRequest(r, http.MethodDelete, fmt.Sprintf("/api/v1/admin/delivery-partners/%d", partner.ID), adminAssignTokenCache, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when deleting a partner with an active order, got %d: %s", w.Code, w.Body.String())
	}

	var stillExists models.DeliveryPartner
	if err := database.DB.First(&stillExists, partner.ID).Error; err != nil {
		t.Errorf("expected partner to still exist after rejected deletion, got error: %v", err)
	}
}

func TestDeleteDeliveryPartner_NoActiveOrdersSucceeds(t *testing.T) {
	r := newAssignmentTestRouter()
	adminAssignTokenCache = adminAssignToken(t)
	partner := seedAssignPartner(t, true)

	w := doRequest(r, http.MethodDelete, fmt.Sprintf("/api/v1/admin/delivery-partners/%d", partner.ID), adminAssignTokenCache, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when deleting a partner with no active orders, got %d: %s", w.Code, w.Body.String())
	}
}
