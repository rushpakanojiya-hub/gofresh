package handlers

import (
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

// Picker history: default is the last 7 days, ?days=N widens it (1..31).
func TestPickerHistoryDaysParam(t *testing.T) {
    database.EnsurePickerTables()
    seed := time.Now().UnixNano() % 1000000000
    user := models.User{Name: "hist-user", Phone: fmt.Sprintf("6%09d", seed), Role: "customer"}
    if err := database.DB.Create(&user).Error; err != nil {
        t.Fatalf("seed user: %v", err)
    }
    addr := models.Address{UserID: user.ID, FullName: "Hist", Phone: "9000000015", Line1: "1 St", City: "C", State: "S", Pincode: "400001"}
    if err := database.DB.Create(&addr).Error; err != nil {
        t.Fatalf("seed address: %v", err)
    }
    wh := models.Warehouse{Name: fmt.Sprintf("hist-wh-%d", seed), City: "TestCity", IsActive: true}
    if err := database.DB.Create(&wh).Error; err != nil {
        t.Fatalf("seed warehouse: %v", err)
    }
    pickerID := uint(900000000 + seed%1000)

    mkTask := func(daysAgo int) models.PickingTask {
        o := models.Order{UserID: user.ID, AddressID: addr.ID, ItemsAmount: 10, TotalAmount: 10, Status: models.OrderStatusDelivered}
        if err := database.DB.Create(&o).Error; err != nil {
            t.Fatalf("seed order: %v", err)
        }
        done := time.Now().AddDate(0, 0, -daysAgo)
        started := done.Add(-2 * time.Minute)
        task := models.PickingTask{OrderID: o.ID, WarehouseID: wh.ID, PickerID: &pickerID, Status: "completed", StartedAt: &started, CompletedAt: &done}
        if err := database.DB.Create(&task).Error; err != nil {
            t.Fatalf("seed task: %v", err)
        }
        return task
    }
    recent := mkTask(1)
    old := mkTask(10)
    t.Cleanup(func() {
        ids := []uint{recent.ID, old.ID}
        orderIDs := []uint{recent.OrderID, old.OrderID}
        database.DB.Where("id IN ?", ids).Delete(&models.PickingTask{})
        database.DB.Unscoped().Where("id IN ?", orderIDs).Delete(&models.Order{})
        database.DB.Unscoped().Delete(&models.Warehouse{}, wh.ID)
        database.DB.Unscoped().Delete(&models.Address{}, addr.ID)
        database.DB.Unscoped().Delete(&models.User{}, user.ID)
    })

    r := gin.New()
    r.Use(func(c *gin.Context) { c.Set("user_id", pickerID); c.Next() })
    r.GET("/history", GetPickerHistory)
    get := func(q string) (int, int64) {
        w := httptest.NewRecorder()
        r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history"+q, nil))
        if w.Code != http.StatusOK {
            t.Fatalf("history%s: HTTP %d: %s", q, w.Code, w.Body.String())
        }
        var resp struct {
            Orders      []map[string]interface{} `json:"orders"`
            TotalOrders int64                    `json:"total_orders"`
        }
        if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
            t.Fatalf("history json: %v", err)
        }
        return len(resp.Orders), resp.TotalOrders
    }

    if n, tot := get(""); n != 1 || tot != 1 {
        t.Errorf("default (7 days): orders=%d total=%d, want 1/1", n, tot)
    }
    if n, tot := get("?days=30"); n != 2 || tot != 2 {
        t.Errorf("days=30: orders=%d total=%d, want 2/2", n, tot)
    }
    if n, tot := get("?days=999"); n != 1 || tot != 1 {
        t.Errorf("days=999 (invalid): orders=%d total=%d, want 1/1 (fallback to 7 days)", n, tot)
    }
}