package handlers

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// BUG-12: DeletePayroll must not hard-delete paid payroll (or payroll that
// already has ledger entries), which would orphan its PAYROLL-<id> journal.

func TestDeletePayrollBlocksPaidAndLedgered(t *testing.T) {
    // Payroll has a foreign key to WarehouseStaff, which this test does not
    // need, so create a plain table (test database only) instead of AutoMigrate.
    if !database.DB.Migrator().HasTable("payrolls") {
        if err := database.DB.Exec(`CREATE TABLE payrolls (
            id bigserial PRIMARY KEY, staff_id bigint NOT NULL, amount double precision NOT NULL,
            month bigint NOT NULL, year bigint NOT NULL, status text NOT NULL DEFAULT 'pending',
            payment_method text, note text, paid_by_id bigint, paid_at timestamptz,
            created_at timestamptz, updated_at timestamptz)`).Error; err != nil {
            t.Fatalf("create payrolls table: %v", err)
        }
    }

    var opex models.Account
    if err := database.DB.Where("code = ?", "5003").First(&opex).Error; err != nil {
        opex = models.Account{Code: "5003", Name: "Operating Expenses", Type: "expense", IsActive: true}
        if err := database.DB.Create(&opex).Error; err != nil {
            t.Fatalf("seed account 5003: %v", err)
        }
    }
    var bank models.Account
    if err := database.DB.Where("code = ?", "1002").First(&bank).Error; err != nil {
        t.Fatalf("account 1002 missing: %v", err)
    }

    var ids []uint
    t.Cleanup(func() {
        if len(ids) > 0 {
            database.DB.Where("reference_type = ? AND reference_id IN ?", "payroll", ids).Delete(&models.LedgerEntry{})
            database.DB.Where("id IN ?", ids).Delete(&models.Payroll{})
        }
    })

    newPayroll := func(status string) models.Payroll {
        now := time.Now()
        p := models.Payroll{StaffID: 1, Amount: 500, Month: 10, Year: 2026, Status: status, PaymentMethod: "bank"}
        if status == "paid" {
            p.PaidAt = &now
        }
        if err := database.DB.Omit("Staff").Create(&p).Error; err != nil {
            t.Fatalf("seed payroll: %v", err)
        }
        ids = append(ids, p.ID)
        return p
    }
    exists := func(id uint) bool {
        var n int64
        database.DB.Model(&models.Payroll{}).Where("id = ?", id).Count(&n)
        return n == 1
    }
    ledgerRows := func(id uint) int64 {
        var n int64
        database.DB.Model(&models.LedgerEntry{}).Where("reference_type = ? AND reference_id = ?", "payroll", id).Count(&n)
        return n
    }

    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set("user_id", uint(1))
        c.Set("phone", "9000000001")
        c.Next()
    })
    r.DELETE("/payroll/:id", DeletePayroll)
    del := func(id uint) (int, string) {
        w := httptest.NewRecorder()
        r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/payroll/%d", id), nil))
        return w.Code, w.Body.String()
    }

    // 1) Paid payroll with its real ledger entries: blocked, nothing lost.
    paid := newPayroll("paid")
    if err := services.PostPayrollLedgerEntry(paid.ID); err != nil {
        t.Fatalf("post payroll ledger: %v", err)
    }
    if got := ledgerRows(paid.ID); got != 2 {
        t.Fatalf("setup: expected 2 ledger rows for paid payroll, got %d", got)
    }
    if code, body := del(paid.ID); code != http.StatusConflict {
        t.Errorf("delete paid payroll: got HTTP %d, want 409 (%s)", code, body)
    }
    if !exists(paid.ID) {
        t.Errorf("paid payroll row was deleted")
    }
    if got := ledgerRows(paid.ID); got != 2 {
        t.Errorf("paid payroll ledger rows = %d, want 2", got)
    }

    // 2) Pending payroll with no ledger entries: deletable.
    pending := newPayroll("pending")
    if code, body := del(pending.ID); code != http.StatusOK {
        t.Errorf("delete pending payroll: got HTTP %d, want 200 (%s)", code, body)
    }
    if exists(pending.ID) {
        t.Errorf("pending payroll row should have been deleted")
    }

    // 3) Safety net: status is not "paid" but ledger entries exist.
    stale := newPayroll("pending")
    sid := stale.ID
    if err := database.DB.Create(&models.LedgerEntry{
        TransactionRef: fmt.Sprintf("PAYROLL-%d", sid), AccountID: opex.ID, Type: "debit", Amount: 500,
        ReferenceType: "payroll", ReferenceID: &sid, EntryDate: time.Now(),
    }).Error; err != nil {
        t.Fatalf("seed stale ledger entry: %v", err)
    }
    if code, body := del(stale.ID); code != http.StatusConflict {
        t.Errorf("delete payroll with ledger entries: got HTTP %d, want 409 (%s)", code, body)
    }
    if !exists(stale.ID) {
        t.Errorf("payroll with ledger entries was deleted")
    }

    // 4) Unknown id.
    if code, body := del(999999999); code != http.StatusNotFound {
        t.Errorf("delete unknown payroll: got HTTP %d, want 404 (%s)", code, body)
    }
}