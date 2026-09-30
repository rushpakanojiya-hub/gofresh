package services

import (
    "fmt"
    "os"
    "sync"
    "testing"
    "time"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

// TestMain wires database.DB to a real Postgres instance for this package's
// tests, since the concurrency guarantees we're testing here (row-level
// locking, transactional isolation) only hold against a real database, not
// a mock. Connection details come from the same DB_* env vars the app
// itself uses (see internal/config), falling back to the docker-compose
// defaults in this repo's .env.example. If no database is reachable, every
// test in this package is skipped rather than failed, so `go test ./...`
// still passes in environments without Postgres available.
func TestMain(m *testing.M) {
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
        fmt.Printf("[delivery_assignment_test] skipping package: could not connect to test database: %v\n", err)
        os.Exit(0)
    }

    if err := db.AutoMigrate(&models.User{}, &models.Address{}, &models.DeliveryPartner{}, &models.Order{}, &models.DeliveryZone{}, &models.DeliveryNotification{}); err != nil {
        fmt.Printf("[delivery_assignment_test] skipping package: migration failed: %v\n", err)
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

// resetDeliveryAssignmentTables truncates every table this test package
// touches (and restarts identity counters) so each test starts from a
// clean, predictable slate regardless of what earlier tests inserted.
func resetDeliveryAssignmentTables(t *testing.T) {
    t.Helper()
    if err := database.DB.Exec("TRUNCATE TABLE orders, delivery_partners, addresses, users, delivery_zones, delivery_notifications, rider_cod_deposits, rider_payouts RESTART IDENTITY CASCADE").Error; err != nil {
        t.Fatalf("failed to reset tables: %v", err)
    }
}

// seedUserAndAddress creates a user and a delivery address at the given
// coordinates (default test pincode "123456"), returning the address ID
// for use on a test order.
func seedUserAndAddress(t *testing.T, phone string, lat, lng float64) uint {
    t.Helper()
    return seedUserAndAddressWithPincode(t, phone, lat, lng, "123456")
}

// seedUserAndAddressWithPincode is like seedUserAndAddress but lets the
// caller control the pincode, for delivery-zone serviceability tests.
func seedUserAndAddressWithPincode(t *testing.T, phone string, lat, lng float64, pincode string) uint {
    t.Helper()
    user := models.User{Name: "Test User", Phone: phone}
    if err := database.DB.Create(&user).Error; err != nil {
        t.Fatalf("failed to seed user: %v", err)
    }
    addr := models.Address{
        UserID:   user.ID,
        FullName: "Test User",
        Phone:    phone,
        Line1:    "1 Test Street",
        City:     "Testville",
        State:    "TS",
        Pincode:  pincode,
        Lat:      &lat,
        Lng:      &lng,
    }
    if err := database.DB.Create(&addr).Error; err != nil {
        t.Fatalf("failed to seed address: %v", err)
    }
    return addr.ID
}

// seedDeliveryZone creates an active delivery zone covering the given
// comma-separated pincodes.
func seedDeliveryZone(t *testing.T, pincodes string) models.DeliveryZone {
    t.Helper()
    zone := models.DeliveryZone{
        Name:     "Test Zone",
        Pincodes: pincodes,
        IsActive: true,
    }
    if err := database.DB.Create(&zone).Error; err != nil {
        t.Fatalf("failed to seed delivery zone: %v", err)
    }
    return zone
}

// seedReadyOrder creates an order at the given address, in the state
// AutoAssignDeliveryPartner expects to act on: no delivery partner yet.
func seedReadyOrder(t *testing.T, addressID uint) models.Order {
    t.Helper()
    order := models.Order{
        UserID:      1,
        AddressID:   addressID,
        ItemsAmount: 100,
        TotalAmount: 100,
        Status:      models.OrderStatusReadyForDispatch,
    }
    if err := database.DB.Create(&order).Error; err != nil {
        t.Fatalf("failed to seed order: %v", err)
    }
    return order
}

// seedActivePartner creates an active, ONLINE delivery partner with a
// fresh location fix at the given coordinates. IsOnline must be true here
// (Phase 3): auto-assign only ever picks from online partners, same as
// the admin assign-delivery endpoint.
func seedActivePartner(t *testing.T, phone string, lat, lng float64) models.DeliveryPartner {
    t.Helper()
    now := time.Now()
    partner := models.DeliveryPartner{
        Name:               "Partner " + phone,
        Phone:              phone,
        IsActive:           true,
        IsOnline:           true,
        CurrentLat:         &lat,
        CurrentLng:         &lng,
        LastLocationUpdate: &now,
    }
    if err := database.DB.Create(&partner).Error; err != nil {
        t.Fatalf("failed to seed delivery partner: %v", err)
    }
    return partner
}

// TestAutoAssign_ConcurrentOrders_NeverDoubleBookSamePartner is the core
// regression test for the race this change fixes: N orders becoming
// READY_FOR_DISPATCH at the same instant, with exactly N active partners
// available. Sequential (correctly-locked) execution must spread the load
// evenly - one order per partner - because each assignment's load count
// should account for every assignment before it. Before the fix, all N
// goroutines could read the same "everyone is free" snapshot concurrently
// and pile onto the single closest partner instead.
func TestAutoAssign_ConcurrentOrders_NeverDoubleBookSamePartner(t *testing.T) {
    resetDeliveryAssignmentTables(t)

    const n = 8

    // All orders sit at (near) the same location, so distance alone
    // doesn't naturally spread them out - only the load-penalty term
    // (which depends on seeing every concurrent assignment) does.
    var orders []models.Order
    for i := 0; i < n; i++ {
        addrID := seedUserAndAddress(t, fmt.Sprintf("90000000%02d", i), 19.0760, 72.8777)
        orders = append(orders, seedReadyOrder(t, addrID))
    }

    var partners []models.DeliveryPartner
    for i := 0; i < n; i++ {
        partners = append(partners, seedActivePartner(t, fmt.Sprintf("80000000%02d", i), 19.0760+float64(i)*0.001, 72.8777))
    }

    var wg sync.WaitGroup
    for _, o := range orders {
        wg.Add(1)
        go func(orderID uint) {
            defer wg.Done()
            AutoAssignDeliveryPartner(orderID)
        }(o.ID)
    }
    wg.Wait()

    // Every order must end up assigned, and no two orders may share a
    // partner - if two goroutines had raced on a stale load snapshot,
    // two orders would land on the same (nearest) partner instead.
    seenPartners := make(map[uint]uint) // partnerID -> orderID that claimed it
    for _, o := range orders {
        var fresh models.Order
        if err := database.DB.First(&fresh, o.ID).Error; err != nil {
            t.Fatalf("failed to reload order %d: %v", o.ID, err)
        }
        if fresh.DeliveryPartnerID == nil {
            t.Errorf("order %d was never assigned a delivery partner", o.ID)
            continue
        }
        pid := *fresh.DeliveryPartnerID
        if otherOrder, taken := seenPartners[pid]; taken {
            t.Errorf("delivery partner %d was assigned to both order %d and order %d", pid, otherOrder, o.ID)
        }
        seenPartners[pid] = o.ID
    }
    if len(seenPartners) != n {
        t.Errorf("expected %d distinct partners used, got %d", n, len(seenPartners))
    }
    _ = partners
}

// TestAutoAssign_ConcurrentCallsSameOrder_AssignsExactlyOnce fires the same
// order's assignment concurrently (e.g. a duplicate webhook/retry, or a
// call racing an admin's manual assignment) and checks it never ends up
// double-processed or left in an inconsistent state: exactly one
// assignment wins, cleanly, with the others rolling back as no-ops.
func TestAutoAssign_ConcurrentCallsSameOrder_AssignsExactlyOnce(t *testing.T) {
    resetDeliveryAssignmentTables(t)

    addrID := seedUserAndAddress(t, "9000000099", 19.0760, 72.8777)
    order := seedReadyOrder(t, addrID)
    seedActivePartner(t, "8000000099", 19.0761, 72.8778)
    seedActivePartner(t, "8000000098", 19.0762, 72.8779)

    const attempts = 10
    var wg sync.WaitGroup
    for i := 0; i < attempts; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            AutoAssignDeliveryPartner(order.ID)
        }()
    }
    wg.Wait()

    var fresh models.Order
    if err := database.DB.First(&fresh, order.ID).Error; err != nil {
        t.Fatalf("failed to reload order: %v", err)
    }
    if fresh.DeliveryPartnerID == nil {
        t.Fatalf("order was never assigned a delivery partner")
    }

    // Re-run once more now that it's assigned: must remain a no-op and
    // must not change the already-assigned partner.
    originalPartner := *fresh.DeliveryPartnerID
    AutoAssignDeliveryPartner(order.ID)
    if err := database.DB.First(&fresh, order.ID).Error; err != nil {
        t.Fatalf("failed to reload order: %v", err)
    }
    if *fresh.DeliveryPartnerID != originalPartner {
        t.Errorf("re-running auto-assign on an already-assigned order changed the partner: %d -> %d", originalPartner, *fresh.DeliveryPartnerID)
    }
}

// TestAutoAssign_SkipsAlreadyAssignedOrder is a basic sanity/regression
// check that the existing no-op behavior (order already has a partner) is
// preserved by the transactional rewrite.
func TestAutoAssign_SkipsAlreadyAssignedOrder(t *testing.T) {
    resetDeliveryAssignmentTables(t)

    addrID := seedUserAndAddress(t, "9000000077", 19.0760, 72.8777)
    order := seedReadyOrder(t, addrID)
    partner := seedActivePartner(t, "8000000077", 19.0761, 72.8778)

    if err := database.DB.Model(&models.Order{}).Where("id = ?", order.ID).
        Update("delivery_partner_id", partner.ID).Error; err != nil {
        t.Fatalf("failed to pre-assign order: %v", err)
    }

    otherPartner := seedActivePartner(t, "8000000076", 19.0700, 72.8700)

    AutoAssignDeliveryPartner(order.ID)

    var fresh models.Order
    if err := database.DB.First(&fresh, order.ID).Error; err != nil {
        t.Fatalf("failed to reload order: %v", err)
    }
    if fresh.DeliveryPartnerID == nil || *fresh.DeliveryPartnerID != partner.ID {
        t.Errorf("expected order to keep its pre-existing partner %d, got %v", partner.ID, fresh.DeliveryPartnerID)
    }
    _ = otherPartner
}

// TestAutoAssign_SkipsPartnerAtCapacity checks that a partner already
// holding MaxActiveOrders confirmed/shipped deliveries is never picked for
// a new one, even when they'd otherwise win on distance, and that the
// order instead goes to a farther partner with spare capacity.
func TestAutoAssign_SkipsPartnerAtCapacity(t *testing.T) {
    resetDeliveryAssignmentTables(t)

    addrID := seedUserAndAddress(t, "9000000060", 19.0760, 72.8777)
    order := seedReadyOrder(t, addrID)

    // Nearest partner, but at capacity (MaxActiveOrders = 1, already
    // carrying 1 confirmed order).
    fullPartner := seedActivePartner(t, "8000000060", 19.0761, 72.8778)
    if err := database.DB.Model(&fullPartner).Update("max_active_orders", 1).Error; err != nil {
        t.Fatalf("failed to set capacity: %v", err)
    }
    busyAddrID := seedUserAndAddress(t, "9000000061", 19.0761, 72.8778)
    busyOrder := seedReadyOrder(t, busyAddrID)
    if err := database.DB.Model(&busyOrder).Updates(map[string]interface{}{
        "status":              models.OrderStatusConfirmed,
        "delivery_partner_id": fullPartner.ID,
    }).Error; err != nil {
        t.Fatalf("failed to seed existing load: %v", err)
    }

    // Farther partner with spare capacity - should win instead.
    farPartner := seedActivePartner(t, "8000000059", 19.2000, 72.9000)

    AutoAssignDeliveryPartner(order.ID)

    var fresh models.Order
    if err := database.DB.First(&fresh, order.ID).Error; err != nil {
        t.Fatalf("failed to reload order: %v", err)
    }
    if fresh.DeliveryPartnerID == nil {
        t.Fatalf("order was never assigned a delivery partner")
    }
    if *fresh.DeliveryPartnerID != farPartner.ID {
        t.Errorf("expected order to skip the at-capacity partner %d and go to %d, got %d", fullPartner.ID, farPartner.ID, *fresh.DeliveryPartnerID)
    }
}

// TestAutoAssign_SkipsPartnerWithoutValidLocation checks that a partner
// with no GPS fix at all is never selected, even as a last resort.
func TestAutoAssign_SkipsPartnerWithoutValidLocation(t *testing.T) {
    resetDeliveryAssignmentTables(t)

    addrID := seedUserAndAddress(t, "9000000050", 19.0760, 72.8777)
    order := seedReadyOrder(t, addrID)

    // Active, online, but never sent a location update.
    locationless := models.DeliveryPartner{
        Name:     "No GPS Partner",
        Phone:    "8000000050",
        IsActive: true,
        IsOnline: true,
    }
    if err := database.DB.Create(&locationless).Error; err != nil {
        t.Fatalf("failed to seed locationless partner: %v", err)
    }

    AutoAssignDeliveryPartner(order.ID)

    var fresh models.Order
    if err := database.DB.First(&fresh, order.ID).Error; err != nil {
        t.Fatalf("failed to reload order: %v", err)
    }
    if fresh.DeliveryPartnerID != nil {
        t.Errorf("expected order to remain unassigned with no located partners available, got partner %v", *fresh.DeliveryPartnerID)
    }
}

// TestAutoAssign_SkipsOrderOutsideServiceableZone checks that when
// delivery zones are configured, an order whose address pincode falls
// outside every active zone is left unassigned.
func TestAutoAssign_SkipsOrderOutsideServiceableZone(t *testing.T) {
    resetDeliveryAssignmentTables(t)
    seedDeliveryZone(t, "380001,380002")

    addrID := seedUserAndAddressWithPincode(t, "9000000040", 19.0760, 72.8777, "999999")
    order := seedReadyOrder(t, addrID)
    seedActivePartner(t, "8000000040", 19.0761, 72.8778)

    AutoAssignDeliveryPartner(order.ID)

    var fresh models.Order
    if err := database.DB.First(&fresh, order.ID).Error; err != nil {
        t.Fatalf("failed to reload order: %v", err)
    }
    if fresh.DeliveryPartnerID != nil {
        t.Errorf("expected order outside every delivery zone to remain unassigned, got partner %v", *fresh.DeliveryPartnerID)
    }
}

// TestAutoAssign_AssignsWithinServiceableZone is the positive counterpart
// of the above: a matching active zone must not block assignment.
func TestAutoAssign_AssignsWithinServiceableZone(t *testing.T) {
    resetDeliveryAssignmentTables(t)
    seedDeliveryZone(t, "380001,380002")

    addrID := seedUserAndAddressWithPincode(t, "9000000041", 19.0760, 72.8777, "380002")
    order := seedReadyOrder(t, addrID)
    partner := seedActivePartner(t, "8000000041", 19.0761, 72.8778)

    AutoAssignDeliveryPartner(order.ID)

    var fresh models.Order
    if err := database.DB.First(&fresh, order.ID).Error; err != nil {
        t.Fatalf("failed to reload order: %v", err)
    }
    if fresh.DeliveryPartnerID == nil || *fresh.DeliveryPartnerID != partner.ID {
        t.Errorf("expected order inside a serviceable zone to be assigned to %d, got %v", partner.ID, fresh.DeliveryPartnerID)
    }
}
