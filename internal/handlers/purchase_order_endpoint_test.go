package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// BUG-11: endpoint-level checks that purchase order status changes cannot
// undo or fake received stock.

func poTestRouter() *gin.Engine {
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set("user_id", uint(1))
        c.Set("phone", "9000000001")
        c.Next()
    })
    r.PUT("/po/:id/status", UpdatePurchaseOrderStatus)
    r.PUT("/po/:id/receive", ReceivePurchaseOrderItems)
    return r
}

func poCall(t *testing.T, r *gin.Engine, path string, body gin.H) (int, string) {
    t.Helper()
    b, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    return w.Code, w.Body.String()
}

func poStatusOf(t *testing.T, id uint) string {
    t.Helper()
    var po models.PurchaseOrder
    if err := database.DB.First(&po, id).Error; err != nil {
        t.Fatalf("load PO %d: %v", id, err)
    }
    return po.Status
}

func poStockOf(productID, warehouseID uint) int {
    var inv models.Inventory
    if err := database.DB.Where("product_id = ? AND warehouse_id = ?", productID, warehouseID).First(&inv).Error; err != nil {
        return 0
    }
    return inv.Stock
}

func TestPurchaseOrderStatusCannotUndoOrFakeReceivedStock(t *testing.T) {
    if err := database.DB.AutoMigrate(
        &models.Vendor{}, &models.Warehouse{}, &models.PurchaseOrder{},
        &models.PurchaseOrderItem{}, &models.Inventory{}, &models.StockMovement{},
    ); err != nil {
        t.Fatalf("migrate: %v", err)
    }

    cat := models.Category{Name: "po-test-category"}
    if err := database.DB.Create(&cat).Error; err != nil {
        t.Fatalf("seed category: %v", err)
    }
    prod := models.Product{Name: "po-test-product", Price: 50, CostPrice: 20, CategoryID: cat.ID}
    if err := database.DB.Create(&prod).Error; err != nil {
        t.Fatalf("seed product: %v", err)
    }
    vendor := models.Vendor{Name: "po-test-vendor", IsActive: true}
    if err := database.DB.Create(&vendor).Error; err != nil {
        t.Fatalf("seed vendor: %v", err)
    }
    wh := models.Warehouse{Name: "po-test-warehouse", City: "TestCity", IsActive: true}
    if err := database.DB.Create(&wh).Error; err != nil {
        t.Fatalf("seed warehouse: %v", err)
    }

    newPO := func(num string) models.PurchaseOrder {
        po := models.PurchaseOrder{
            PONumber: num, VendorID: vendor.ID, WarehouseID: wh.ID,
            Status: "draft", TotalAmount: 200, CreatedByID: 1,
            Items: []models.PurchaseOrderItem{{ProductID: prod.ID, QuantityOrdered: 10, UnitPrice: 20}},
        }
        if err := database.DB.Create(&po).Error; err != nil {
            t.Fatalf("seed PO %s: %v", num, err)
        }
        return po
    }
    po1 := newPO("PO-TEST-BUG11-A")
    po2 := newPO("PO-TEST-BUG11-B")

    t.Cleanup(func() {
        ids := []uint{po1.ID, po2.ID}
        database.DB.Where("reference_id IN ?", ids).Delete(&models.StockMovement{})
        database.DB.Where("purchase_order_id IN ?", ids).Delete(&models.PurchaseOrderItem{})
        database.DB.Where("id IN ?", ids).Delete(&models.PurchaseOrder{})
        database.DB.Where("product_id = ?", prod.ID).Delete(&models.Inventory{})
        database.DB.Unscoped().Delete(&models.Warehouse{}, wh.ID)
        database.DB.Delete(&models.Vendor{}, vendor.ID)
        database.DB.Unscoped().Delete(&models.Product{}, prod.ID)
        database.DB.Unscoped().Delete(&models.Category{}, cat.ID)
    })

    r := poTestRouter()
    statusURL := func(id uint) string { return fmt.Sprintf("/po/%d/status", id) }
    receiveURL := func(id uint) string { return fmt.Sprintf("/po/%d/receive", id) }
    receive := func(id uint, itemID uint, qty int) int {
        code, body := poCall(t, r, receiveURL(id), gin.H{"items": []gin.H{{"item_id": itemID, "quantity_received": qty}}})
        if code != http.StatusOK {
            t.Fatalf("receive %d on PO %d: got %d: %s", qty, id, code, body)
        }
        return code
    }
    expect := func(label string, gotCode, wantCode int, body string) {
        t.Helper()
        if gotCode != wantCode {
            t.Errorf("%s: got HTTP %d, want %d (%s)", label, gotCode, wantCode, body)
        }
    }
    expectState := func(label string, id uint, wantStatus string, wantStock int) {
        t.Helper()
        if s := poStatusOf(t, id); s != wantStatus {
            t.Errorf("%s: PO status = %s, want %s", label, s, wantStatus)
        }
        if st := poStockOf(prod.ID, wh.ID); st != wantStock {
            t.Errorf("%s: stock = %d, want %d", label, st, wantStock)
        }
    }

    // draft -> sent is a valid manual transition.
    code, body := poCall(t, r, statusURL(po1.ID), gin.H{"status": "sent"})
    expect("draft -> sent", code, http.StatusOK, body)

    // Receive 4 of 10: partially_received, stock 4.
    receive(po1.ID, po1.Items[0].ID, 4)
    expectState("after partial receipt", po1.ID, "partially_received", 4)

    // Cancelling (or reverting) a partially received PO must be refused and
    // must leave status and stock untouched.
    code, body = poCall(t, r, statusURL(po1.ID), gin.H{"status": "cancelled", "reason": "test"})
    expect("partially_received -> cancelled", code, http.StatusConflict, body)
    expectState("after blocked cancel (partial)", po1.ID, "partially_received", 4)

    code, body = poCall(t, r, statusURL(po1.ID), gin.H{"status": "draft"})
    expect("partially_received -> draft", code, http.StatusConflict, body)
    expectState("after blocked revert to draft", po1.ID, "partially_received", 4)

    // Receive the rest: received, stock 10.
    receive(po1.ID, po1.Items[0].ID, 6)
    expectState("after full receipt", po1.ID, "received", 10)

    code, body = poCall(t, r, statusURL(po1.ID), gin.H{"status": "cancelled", "reason": "test"})
    expect("received -> cancelled", code, http.StatusConflict, body)
    expectState("after blocked cancel (received)", po1.ID, "received", 10)

    // A draft PO cannot be marked received without receiving items, so no
    // stock appears from nowhere.
    code, body = poCall(t, r, statusURL(po2.ID), gin.H{"status": "received"})
    expect("draft -> received via status endpoint", code, http.StatusBadRequest, body)
    expectState("after blocked fake receive", po2.ID, "draft", 10)
}