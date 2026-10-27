package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// BUG-13: admin marking an online order delivered must also close the courier
// delivery_status, and live rider GPS must not be shared after delivery.
func TestAdminDeliveredClosesDeliveryStatusAndTracking(t *testing.T) {
    seed := time.Now().UnixNano() % 1000000000
    user := models.User{Name: "bug13-user", Phone: fmt.Sprintf("9%09d", seed), Role: "customer"}
    if err := database.DB.Create(&user).Error; err != nil {
        t.Fatalf("seed user: %v", err)
    }
    addr := models.Address{UserID: user.ID, FullName: "Bug13", Phone: "9000000013", Line1: "1 Test St", City: "TestCity", State: "TS", Pincode: "400001"}
    if err := database.DB.Create(&addr).Error; err != nil {
        t.Fatalf("seed address: %v", err)
    }
    lat, lng := 19.07, 72.87
    partner := models.DeliveryPartner{Name: "bug13-rider", Phone: fmt.Sprintf("8%09d", seed), IsActive: true, IsOnline: true, CurrentLat: &lat, CurrentLng: &lng, MaxActiveOrders: 1, ApprovalStatus: "approved", OnboardingStep: 5}
    if err := database.DB.Create(&partner).Error; err != nil {
        t.Fatalf("seed partner: %v", err)
    }
    ds := models.DeliveryStatusOutForDelivery
    order := models.Order{
        UserID: user.ID, AddressID: addr.ID, ItemsAmount: 100, TotalAmount: 100,
        Status: models.OrderStatusShipped, PaymentMethod: models.PaymentMethodOnline,
        PaymentStatus: models.OrderPaymentStatusPaid, DeliveryPartnerID: &partner.ID, DeliveryStatus: &ds,
    }
    if err := database.DB.Create(&order).Error; err != nil {
        t.Fatalf("seed order: %v", err)
    }
    t.Cleanup(func() {
        database.DB.Unscoped().Delete(&models.Order{}, order.ID)
        database.DB.Unscoped().Delete(&models.DeliveryPartner{}, partner.ID)
        database.DB.Unscoped().Delete(&models.Address{}, addr.ID)
        database.DB.Unscoped().Delete(&models.User{}, user.ID)
    })

    admin := gin.New()
    admin.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Set("phone", "9000000001"); c.Next() })
    admin.PUT("/orders/:id/status", UpdateOrderStatus)
    cust := gin.New()
    cust.Use(func(c *gin.Context) { c.Set("user_id", user.ID); c.Next() })
    cust.GET("/orders/:id/tracking", GetOrderTracking)

    leaks := func() bool {
        w := httptest.NewRecorder()
        cust.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/orders/%d/tracking", order.ID), nil))
        var m map[string]interface{}
        if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
            t.Fatalf("tracking json: %v (%s)", err, w.Body.String())
        }
        tr, _ := m["tracking"].(map[string]interface{})
        v, ok := tr["current_lat"]
        return ok && v != nil
    }

    if !leaks() {
        t.Fatalf("setup: expected live GPS to be visible while out_for_delivery")
    }

    body, _ := json.Marshal(gin.H{"status": "delivered"})
    req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/orders/%d/status", order.ID), bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    admin.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("admin delivered: got HTTP %d, want 200 (%s)", w.Code, w.Body.String())
    }

    var fresh models.Order
    database.DB.First(&fresh, order.ID)
    if fresh.Status != models.OrderStatusDelivered {
        t.Errorf("status = %s, want delivered", fresh.Status)
    }
    if fresh.DeliveryStatus == nil || *fresh.DeliveryStatus != models.DeliveryStatusDelivered {
        t.Errorf("delivery_status not set to delivered after admin delivered")
    }
    if leaks() {
        t.Errorf("live GPS still visible after admin marked order delivered")
    }

    // Defence in depth: even if some path leaves delivery_status stale, a
    // delivered order must never expose live GPS.
    stale := models.DeliveryStatusOutForDelivery
    database.DB.Model(&models.Order{}).Where("id = ?", order.ID).Update("delivery_status", stale)
    if leaks() {
        t.Errorf("live GPS visible for delivered order with stale delivery_status")
    }
}