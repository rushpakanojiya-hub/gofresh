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

// BUG-15: the damaged/expired summary cards must respect warehouse_id and category_id.
func TestInventoryOverviewDamagedExpiredFollowFilters(t *testing.T) {
    sfx := time.Now().UnixNano() % 1000000000
    var pids, whids, cids []uint
    t.Cleanup(func() {
        database.DB.Where("product_id IN ?", pids).Delete(&models.StockMovement{})
        database.DB.Where("product_id IN ?", pids).Delete(&models.Batch{})
        database.DB.Unscoped().Where("id IN ?", pids).Delete(&models.Product{})
        database.DB.Unscoped().Where("id IN ?", whids).Delete(&models.Warehouse{})
        database.DB.Unscoped().Where("id IN ?", cids).Delete(&models.Category{})
    })

    var cats [2]models.Category
    var whs [2]models.Warehouse
    var prods [2]models.Product
    for i := 0; i < 2; i++ {
        cats[i] = models.Category{Name: fmt.Sprintf("bug15-cat-%d-%d", i, sfx)}
        if err := database.DB.Create(&cats[i]).Error; err != nil {
            t.Fatalf("seed category: %v", err)
        }
        cids = append(cids, cats[i].ID)
        whs[i] = models.Warehouse{Name: fmt.Sprintf("bug15-wh-%d-%d", i, sfx), City: "TestCity", IsActive: true}
        if err := database.DB.Create(&whs[i]).Error; err != nil {
            t.Fatalf("seed warehouse: %v", err)
        }
        whids = append(whids, whs[i].ID)
        prods[i] = models.Product{Name: fmt.Sprintf("bug15-prod-%d-%d", i, sfx), Price: 50, CostPrice: 20, CategoryID: cats[i].ID}
        if err := database.DB.Create(&prods[i]).Error; err != nil {
            t.Fatalf("seed product: %v", err)
        }
        pids = append(pids, prods[i].ID)
        mv := models.StockMovement{ProductID: prods[i].ID, WarehouseID: whs[i].ID, PreviousQty: 5, Change: -5, NewQty: 0, MovementType: models.MovementDamaged, Reason: models.AdjustReasonDamaged}
        if err := database.DB.Create(&mv).Error; err != nil {
            t.Fatalf("seed stock movement: %v", err)
        }
        b := models.Batch{ProductID: prods[i].ID, WarehouseID: whs[i].ID, BatchNumber: fmt.Sprintf("B15-%d-%d", i, sfx), ExpiryDate: time.Now().AddDate(0, 0, -1), Quantity: 5, CreatedByStaffID: 1}
        if err := database.DB.Create(&b).Error; err != nil {
            t.Fatalf("seed batch: %v", err)
        }
    }

    r := gin.New()
    r.GET("/inventory", GetInventoryOverview)
    overview := func(q string) models.InventoryOverviewResponse {
        w := httptest.NewRecorder()
        r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/inventory?page=1&limit=10&"+q, nil))
        if w.Code != http.StatusOK {
            t.Fatalf("overview %s: HTTP %d: %s", q, w.Code, w.Body.String())
        }
        var resp models.InventoryOverviewResponse
        if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
            t.Fatalf("overview json: %v", err)
        }
        return resp
    }
    check := func(label, q string, wantDamaged, wantExpired int64) {
        t.Helper()
        got := overview(q)
        if got.DamagedStock != wantDamaged {
            t.Errorf("%s: damaged_stock = %d, want %d", label, got.DamagedStock, wantDamaged)
        }
        if got.ExpiredStock != wantExpired {
            t.Errorf("%s: expired_stock = %d, want %d", label, got.ExpiredStock, wantExpired)
        }
    }

    check("warehouse 1", fmt.Sprintf("warehouse_id=%d", whs[0].ID), 1, 1)
    check("category 2", fmt.Sprintf("category_id=%d", cats[1].ID), 1, 1)
    check("warehouse 1 + category 2", fmt.Sprintf("warehouse_id=%d&category_id=%d", whs[0].ID, cats[1].ID), 0, 0)
}