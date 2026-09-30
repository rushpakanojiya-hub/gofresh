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
)

func mustUnmarshal(t *testing.T, data []byte, v interface{}) {
t.Helper()
if err := json.Unmarshal(data, v); err != nil {
t.Fatalf("failed to unmarshal response: %v (body: %s)", err, string(data))
}
}

func fixedTestTime() time.Time {
return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func fmtPath(format string, args ...interface{}) string {
return fmt.Sprintf(format, args...)
}

// newCODNotificationTestRouter wires up the delivery partner routes added
// in Phase B - COD summary/settlements and notifications - plus the
// resolve-failed endpoint from Phase A, guarded the same way as the real
// app.
func newCODNotificationTestRouter() *gin.Engine {
r := gin.New()
delivery := r.Group("/api/v1/delivery")
delivery.Use(middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly())
delivery.GET("/cod-summary", GetMyCODSummary)
delivery.GET("/cod-settlements", GetMyCODSettlements)
delivery.GET("/notifications", GetMyDeliveryNotifications)
delivery.PUT("/notifications/:id/read", MarkDeliveryNotificationRead)
delivery.PUT("/notifications/read-all", MarkAllDeliveryNotificationsRead)
delivery.PUT("/orders/:id/resolve-failed", ResolveFailedDelivery)
return r
}

// ---------------------------------------------------------------------------
// COD summary
// ---------------------------------------------------------------------------

func TestGetMyCODSummary_NoDeliveriesReturnsZeroes(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

w := doRequest(r, http.MethodGet, "/api/v1/delivery/cod-summary", token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}
var resp CODSummary
mustUnmarshal(t, w.Body.Bytes(), &resp)
if resp.TotalCollected != 0 || resp.TotalDeposited != 0 || resp.PendingSettlement != 0 {
t.Errorf("expected all-zero summary for a partner with no deliveries, got %+v", resp)
}
}

func TestGetMyCODSummary_CountsOnlyDeliveredCODOrdersForThisPartner(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
otherPartner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

// Delivered COD order for this partner - should count.
order := seedAssignOrder(t, models.OrderStatusDelivered)
database.DB.Model(&order).Updates(map[string]interface{}{
"delivery_partner_id": partner.ID,
"payment_method":      models.PaymentMethodCOD,
"total_amount":        640.0,
})

// Delivered COD order for a DIFFERENT partner - must not count.
otherOrder := seedAssignOrder(t, models.OrderStatusDelivered)
database.DB.Model(&otherOrder).Updates(map[string]interface{}{
"delivery_partner_id": otherPartner.ID,
"payment_method":      models.PaymentMethodCOD,
"total_amount":        999.0,
})

// Delivered ONLINE order for this partner - must not count toward COD.
onlineOrder := seedAssignOrder(t, models.OrderStatusDelivered)
database.DB.Model(&onlineOrder).Updates(map[string]interface{}{
"delivery_partner_id": partner.ID,
"payment_method":      models.PaymentMethodOnline,
"total_amount":        300.0,
})

w := doRequest(r, http.MethodGet, "/api/v1/delivery/cod-summary", token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}
var resp CODSummary
mustUnmarshal(t, w.Body.Bytes(), &resp)
if resp.TotalCollected != 640.0 {
t.Errorf("expected total_collected=640 (only this partner's COD order), got %v", resp.TotalCollected)
}
if resp.PendingSettlement != 640.0 {
t.Errorf("expected pending_settlement=640 with no deposits yet, got %v", resp.PendingSettlement)
}
}

func TestGetMyCODSummary_VerifiedDepositReducesPendingSettlement(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

order := seedAssignOrder(t, models.OrderStatusDelivered)
database.DB.Model(&order).Updates(map[string]interface{}{
"delivery_partner_id": partner.ID,
"payment_method":      models.PaymentMethodCOD,
"total_amount":        1000.0,
})

verifiedDeposit := models.RiderCODDeposit{
DeliveryPartnerID: partner.ID,
Amount:            600.0,
DepositDate:       fixedTestTime(),
Status:            "verified",
CreatedByID:       partner.ID,
}
database.DB.Create(&verifiedDeposit)

// A pending (unverified) deposit must NOT reduce pending settlement.
pendingDeposit := models.RiderCODDeposit{
DeliveryPartnerID: partner.ID,
Amount:            200.0,
DepositDate:       fixedTestTime(),
Status:            "pending",
CreatedByID:       partner.ID,
}
database.DB.Create(&pendingDeposit)

w := doRequest(r, http.MethodGet, "/api/v1/delivery/cod-summary", token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}
var resp CODSummary
mustUnmarshal(t, w.Body.Bytes(), &resp)
if resp.TotalDeposited != 600.0 {
t.Errorf("expected total_deposited=600 (verified only), got %v", resp.TotalDeposited)
}
if resp.PendingSettlement != 400.0 {
t.Errorf("expected pending_settlement=1000-600=400, got %v", resp.PendingSettlement)
}
}

// ---------------------------------------------------------------------------
// COD settlements (own history)
// ---------------------------------------------------------------------------

func TestGetMyCODSettlements_OnlyReturnsOwnDeposits(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
otherPartner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

database.DB.Create(&models.RiderCODDeposit{
DeliveryPartnerID: partner.ID,
Amount:            500.0,
DepositDate:       fixedTestTime(),
Status:            "verified",
CreatedByID:       partner.ID,
})
database.DB.Create(&models.RiderCODDeposit{
DeliveryPartnerID: otherPartner.ID,
Amount:            999.0,
DepositDate:       fixedTestTime(),
Status:            "verified",
CreatedByID:       otherPartner.ID,
})

w := doRequest(r, http.MethodGet, "/api/v1/delivery/cod-settlements", token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}
var resp struct {
Settlements []models.RiderCODDeposit `json:"settlements"`
}
mustUnmarshal(t, w.Body.Bytes(), &resp)
if len(resp.Settlements) != 1 {
t.Fatalf("expected exactly 1 settlement (own only), got %d", len(resp.Settlements))
}
if resp.Settlements[0].Amount != 500.0 {
t.Errorf("expected the partner's own 500.0 deposit, got %v", resp.Settlements[0].Amount)
}
}

// ---------------------------------------------------------------------------
// Notifications
// ---------------------------------------------------------------------------

func TestGetMyDeliveryNotifications_OnlyReturnsOwnNotifications(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
otherPartner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

database.DB.Create(&models.DeliveryNotification{DeliveryPartnerID: partner.ID, Title: "Mine", Message: "m1", Type: "new_assignment"})
database.DB.Create(&models.DeliveryNotification{DeliveryPartnerID: otherPartner.ID, Title: "Not mine", Message: "m2", Type: "new_assignment"})

w := doRequest(r, http.MethodGet, "/api/v1/delivery/notifications", token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}
var resp struct {
Notifications []models.DeliveryNotification `json:"notifications"`
UnreadCount   int64                          `json:"unread_count"`
}
mustUnmarshal(t, w.Body.Bytes(), &resp)
if len(resp.Notifications) != 1 {
t.Fatalf("expected exactly 1 notification (own only), got %d", len(resp.Notifications))
}
if resp.Notifications[0].Title != "Mine" {
t.Errorf("expected the partner's own notification, got %q", resp.Notifications[0].Title)
}
if resp.UnreadCount != 1 {
t.Errorf("expected unread_count=1, got %d", resp.UnreadCount)
}
}

func TestGetMyDeliveryNotifications_UnreadOnlyFilter(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

database.DB.Create(&models.DeliveryNotification{DeliveryPartnerID: partner.ID, Title: "Unread", Message: "m1", Type: "new_assignment", IsRead: false})
database.DB.Create(&models.DeliveryNotification{DeliveryPartnerID: partner.ID, Title: "Read", Message: "m2", Type: "new_assignment", IsRead: true})

w := doRequest(r, http.MethodGet, "/api/v1/delivery/notifications?unread_only=true", token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}
var resp struct {
Notifications []models.DeliveryNotification `json:"notifications"`
}
mustUnmarshal(t, w.Body.Bytes(), &resp)
if len(resp.Notifications) != 1 || resp.Notifications[0].Title != "Unread" {
t.Fatalf("expected only the unread notification, got %+v", resp.Notifications)
}
}

func TestMarkDeliveryNotificationRead_OnlyOwnerCanMark(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
otherPartner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, otherPartner)

notif := models.DeliveryNotification{DeliveryPartnerID: partner.ID, Title: "X", Message: "m", Type: "new_assignment"}
database.DB.Create(&notif)

// otherPartner tries to mark partner's notification as read - must fail.
w := doRequest(r, http.MethodPut, fmtPath("/api/v1/delivery/notifications/%d/read", notif.ID), token, nil)
if w.Code != http.StatusNotFound {
t.Fatalf("expected 404 when marking another partner's notification, got %d: %s", w.Code, w.Body.String())
}

var fresh models.DeliveryNotification
database.DB.First(&fresh, notif.ID)
if fresh.IsRead {
t.Errorf("notification must remain unread after a non-owner's attempt")
}
}

func TestMarkDeliveryNotificationRead_Success(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

notif := models.DeliveryNotification{DeliveryPartnerID: partner.ID, Title: "X", Message: "m", Type: "new_assignment"}
database.DB.Create(&notif)

w := doRequest(r, http.MethodPut, fmtPath("/api/v1/delivery/notifications/%d/read", notif.ID), token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}

var fresh models.DeliveryNotification
database.DB.First(&fresh, notif.ID)
if !fresh.IsRead {
t.Errorf("expected notification to be marked read")
}
if fresh.ReadAt == nil {
t.Errorf("expected read_at to be set")
}
}

func TestMarkAllDeliveryNotificationsRead_OnlyAffectsOwnUnread(t *testing.T) {
r := newCODNotificationTestRouter()
partner := seedAssignPartner(t, true)
otherPartner := seedAssignPartner(t, true)
token := deliveryPartnerToken(t, partner)

n1 := models.DeliveryNotification{DeliveryPartnerID: partner.ID, Title: "A", Message: "m", Type: "new_assignment"}
n2 := models.DeliveryNotification{DeliveryPartnerID: partner.ID, Title: "B", Message: "m", Type: "new_assignment"}
nOther := models.DeliveryNotification{DeliveryPartnerID: otherPartner.ID, Title: "C", Message: "m", Type: "new_assignment"}
database.DB.Create(&n1)
database.DB.Create(&n2)
database.DB.Create(&nOther)

w := doRequest(r, http.MethodPut, "/api/v1/delivery/notifications/read-all", token, nil)
if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
}

var fresh1, fresh2, freshOther models.DeliveryNotification
database.DB.First(&fresh1, n1.ID)
database.DB.First(&fresh2, n2.ID)
database.DB.First(&freshOther, nOther.ID)
if !fresh1.IsRead || !fresh2.IsRead {
t.Errorf("expected both of this partner's notifications to be read")
}
if freshOther.IsRead {
t.Errorf("must not mark another partner's notification as read")
}
}
