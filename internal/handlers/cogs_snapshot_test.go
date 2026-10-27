package handlers

import (
    "encoding/json"
    "math"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// BUG-09: P&L and the finance dashboard must compute COGS from the
// order_items.cost_price snapshot, so later product cost edits do not rewrite
// history. Rows without a snapshot (cost_price = 0) fall back to the live
// product cost.

func cogsFromEndpoint(t *testing.T, handler gin.HandlerFunc, dashboard bool) float64 {
    t.Helper()
    r := gin.New()
    r.GET("/x", handler)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
    if w.Code != http.StatusOK {
        t.Fatalf("endpoint returned %d: %s", w.Code, w.Body.String())
    }
    var body map[string]interface{}
    if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
        t.Fatalf("bad json: %v", err)
    }
    src := body
    if dashboard {
        rev, ok := body["revenue"].(map[string]interface{})
        if !ok {
            t.Fatalf("dashboard response has no revenue object: %s", w.Body.String())
        }
        src = rev
    }
    v, ok := src["cogs"].(float64)
    if !ok {
        t.Fatalf("response has no numeric cogs: %s", w.Body.String())
    }
    return v
}

func assertCOGSDelta(t *testing.T, label string, got, base, want float64) {
    t.Helper()
    if math.Abs((got-base)-want) > 0.001 {
        t.Errorf("%s: COGS delta = %.2f, want %.2f (base %.2f, got %.2f)", label, got-base, want, base, got)
    }
}

func TestCOGSUsesOrderItemSnapshotNotLiveProductCost(t *testing.T) {
    cat := models.Category{Name: "cogs-test-category"}
    if err := database.DB.Create(&cat).Error; err != nil {
        t.Fatalf("seed category: %v", err)
    }
    pA := models.Product{Name: "cogs-test-A", Price: 100, CostPrice: 80, CategoryID: cat.ID}
    pB := models.Product{Name: "cogs-test-B", Price: 60, CostPrice: 30, CategoryID: cat.ID}
    if err := database.DB.Create(&pA).Error; err != nil {
        t.Fatalf("seed product A: %v", err)
    }
    if err := database.DB.Create(&pB).Error; err != nil {
        t.Fatalf("seed product B: %v", err)
    }
    order := seedAssignOrder(t, models.OrderStatusDelivered)

    t.Cleanup(func() {
        database.DB.Unscoped().Where("order_id = ?", order.ID).Delete(&models.OrderItem{})
        database.DB.Unscoped().Delete(&models.Order{}, order.ID)
        database.DB.Unscoped().Delete(&models.Product{}, []uint{pA.ID, pB.ID})
        database.DB.Unscoped().Delete(&models.Category{}, cat.ID)
    })

    // Baseline after the order exists but before it has any items.
    basePL := cogsFromEndpoint(t, GetProfitLoss, false)
    baseDash := cogsFromEndpoint(t, FinanceDashboard, true)

    items := []models.OrderItem{
        {OrderID: order.ID, ProductID: pA.ID, Quantity: 2, Price: 100, CostPrice: 80}, // snapshot taken at checkout
        {OrderID: order.ID, ProductID: pB.ID, Quantity: 1, Price: 60, CostPrice: 0},   // pre-snapshot row
    }
    if err := database.DB.Create(&items).Error; err != nil {
        t.Fatalf("seed order items: %v", err)
    }

    // Before any cost edit: 2*80 (snapshot) + 1*30 (fallback) = 190.
    assertCOGSDelta(t, "P&L before cost edit", cogsFromEndpoint(t, GetProfitLoss, false), basePL, 190)
    assertCOGSDelta(t, "dashboard before cost edit", cogsFromEndpoint(t, FinanceDashboard, true), baseDash, 190)

    // Admin edits product costs afterwards.
    if err := database.DB.Model(&models.Product{}).Where("id = ?", pA.ID).Update("cost_price", 180).Error; err != nil {
        t.Fatalf("edit cost A: %v", err)
    }
    if err := database.DB.Model(&models.Product{}).Where("id = ?", pB.ID).Update("cost_price", 50).Error; err != nil {
        t.Fatalf("edit cost B: %v", err)
    }

    // A is frozen by its snapshot: 2*80 = 160. B has no snapshot so it
    // follows the live cost: 1*50. Total 210. The old (buggy) code gives 410.
    assertCOGSDelta(t, "P&L after cost edit", cogsFromEndpoint(t, GetProfitLoss, false), basePL, 210)
    assertCOGSDelta(t, "dashboard after cost edit", cogsFromEndpoint(t, FinanceDashboard, true), baseDash, 210)
}