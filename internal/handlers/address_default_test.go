package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "sync"
    "testing"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// BUG-14: a user must always end up with exactly one default address.

type addrEnv struct {
    r      *gin.Engine
    userID uint
}

func newAddrEnv(t *testing.T) *addrEnv {
    t.Helper()
    seed := time.Now().UnixNano() % 1000000000
    user := models.User{Name: "bug14-user", Phone: fmt.Sprintf("7%09d", seed), Role: "customer"}
    if err := database.DB.Create(&user).Error; err != nil {
        t.Fatalf("seed user: %v", err)
    }
    t.Cleanup(func() {
        database.DB.Where("user_id = ?", user.ID).Delete(&models.Address{})
        database.DB.Delete(&models.User{}, user.ID)
    })
    r := gin.New()
    r.Use(func(c *gin.Context) { c.Set("user_id", user.ID); c.Next() })
    r.POST("/addresses", CreateAddress)
    r.PUT("/addresses/:id", UpdateAddress)
    r.PUT("/addresses/:id/default", SetDefaultAddress)
    return &addrEnv{r: r, userID: user.ID}
}

func (e *addrEnv) call(method, path string, body gin.H) (int, []byte) {
    var rd *bytes.Reader
    if body == nil {
        rd = bytes.NewReader(nil)
    } else {
        b, _ := json.Marshal(body)
        rd = bytes.NewReader(b)
    }
    req := httptest.NewRequest(method, path, rd)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    e.r.ServeHTTP(w, req)
    return w.Code, w.Body.Bytes()
}

func addrBody(isDefault bool) gin.H {
    return gin.H{"full_name": "Bug14", "phone": "9000000014", "line1": "1 Test St", "city": "TestCity", "state": "TS", "pincode": "400001", "is_default": isDefault}
}

func (e *addrEnv) create(t *testing.T, isDefault bool) uint {
    t.Helper()
    code, body := e.call(http.MethodPost, "/addresses", addrBody(isDefault))
    if code != http.StatusCreated {
        t.Fatalf("create address: HTTP %d (%s)", code, body)
    }
    var a models.Address
    if err := json.Unmarshal(body, &a); err != nil {
        t.Fatalf("create address json: %v", err)
    }
    return a.ID
}

func (e *addrEnv) defaults() []uint {
    var ids []uint
    database.DB.Model(&models.Address{}).
        Where("user_id = ? AND is_default = ? AND is_deleted = false", e.userID, true).
        Order("id").Pluck("id", &ids)
    return ids
}

func (e *addrEnv) wantOnlyDefault(t *testing.T, label string, id uint) {
    t.Helper()
    got := e.defaults()
    if len(got) != 1 || got[0] != id {
        t.Errorf("%s: default addresses = %v, want exactly [%d]", label, got, id)
    }
}

func TestAddressDefaultFlagSequential(t *testing.T) {
    e := newAddrEnv(t)
    a1 := e.create(t, false) // first address becomes default automatically
    e.wantOnlyDefault(t, "first create", a1)
    a2 := e.create(t, true)
    e.wantOnlyDefault(t, "create with is_default", a2)
    a3 := e.create(t, false)
    e.wantOnlyDefault(t, "create without is_default", a2)

    if code, body := e.call(http.MethodPut, fmt.Sprintf("/addresses/%d/default", a1), nil); code != http.StatusOK {
        t.Fatalf("set default: HTTP %d (%s)", code, body)
    }
    e.wantOnlyDefault(t, "set default", a1)

    if code, body := e.call(http.MethodPut, fmt.Sprintf("/addresses/%d", a3), addrBody(true)); code != http.StatusOK {
        t.Fatalf("update address: HTTP %d (%s)", code, body)
    }
    e.wantOnlyDefault(t, "update with is_default", a3)
}

func TestAddressDefaultFlagConcurrentSetDefault(t *testing.T) {
    e := newAddrEnv(t)
    var ids []uint
    for i := 0; i < 8; i++ {
        ids = append(ids, e.create(t, false))
    }
    for round := 0; round < 15; round++ {
        var wg sync.WaitGroup
        for _, id := range ids {
            wg.Add(1)
            go func(id uint) {
                defer wg.Done()
                if code, body := e.call(http.MethodPut, fmt.Sprintf("/addresses/%d/default", id), nil); code != http.StatusOK {
                    t.Errorf("set default %d: HTTP %d (%s)", id, code, body)
                }
            }(id)
        }
        wg.Wait()
        if n := len(e.defaults()); n != 1 {
            t.Fatalf("round %d: %d default addresses after concurrent set-default, want 1", round, n)
        }
    }
}

func TestAddressDefaultFlagConcurrentCreate(t *testing.T) {
    for _, isDefault := range []bool{false, true} {
        e := newAddrEnv(t)
        var wg sync.WaitGroup
        for i := 0; i < 8; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                if code, body := e.call(http.MethodPost, "/addresses", addrBody(isDefault)); code != http.StatusCreated {
                    t.Errorf("concurrent create: HTTP %d (%s)", code, body)
                }
            }()
        }
        wg.Wait()
        if n := len(e.defaults()); n != 1 {
            t.Errorf("concurrent create (is_default=%v): %d default addresses, want 1", isDefault, n)
        }
    }
}