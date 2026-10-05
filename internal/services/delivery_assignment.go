package services

import (
    "errors"
    "fmt"
    "log"
    "math"
    "strings"
    "time"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

// staleLocationWindow is how recent a partner's last location update must
// be for them to be considered "trackable" for nearest-match purposes.
// Partners with an older (or missing) location are still eligible, but are
// ranked after trackable ones, since we can't judge their real distance.
const staleLocationWindow = 3 * time.Hour

// haversineKm returns the great-circle distance in kilometers between two
// lat/lng points. This is the standard formula used for short-to-medium
// range distance estimates where earth curvature matters slightly.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
    const earthRadiusKm = 6371.0
    dLat := (lat2 - lat1) * math.Pi / 180.0
    dLng := (lng2 - lng1) * math.Pi / 180.0
    rLat1 := lat1 * math.Pi / 180.0
    rLat2 := lat2 * math.Pi / 180.0

    a := math.Sin(dLat/2)*math.Sin(dLat/2) +
        math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
    c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
    return earthRadiusKm * c
}

// errAutoAssignSkipped is an internal sentinel used to unwind the
// transaction (with a clean rollback - there's nothing to undo since no
// write happened yet) when there's simply nothing to do: already assigned,
// no address coordinates, or no partner available. It is never logged as a
// failure.
var errAutoAssignSkipped = errors.New("auto-assign: nothing to do")

// isPincodeServiceable reports whether pincode falls inside any active
// DeliveryZone. If no active zones are configured at all, the zone system
// is treated as not-yet-set-up and every pincode is considered
// serviceable, so this never blocks auto-assign in deployments that
// haven't configured zones yet.
func isPincodeServiceable(tx *gorm.DB, pincode string) (bool, error) {
    pincode = strings.TrimSpace(pincode)
    var zones []models.DeliveryZone
    if err := tx.Where("is_active = ?", true).Find(&zones).Error; err != nil {
        return false, fmt.Errorf("failed to load delivery zones: %w", err)
    }
    if len(zones) == 0 {
        return true, nil
    }
    if pincode == "" {
        return false, nil
    }
    for _, zone := range zones {
        for _, code := range strings.Split(zone.Pincodes, ",") {
            if strings.TrimSpace(code) == pincode {
                return true, nil
            }
        }
    }
    return false, nil
}

// AutoAssignDeliveryPartner picks the nearest eligible, currently-unburdened
// delivery partner and assigns them to the given order. It's a no-op (logs
// and returns) if the order already has a partner, if the order's address
// has no coordinates, if the order's pincode falls outside every configured
// delivery zone, or if no eligible partner is available.
//
// Eligibility: a partner must be active, online, have a valid (non-null)
// current GPS location, and have spare capacity (fewer active
// confirmed/shipped orders than their configured MaxActiveOrders). Partners
// failing any of these are not candidates at all, not merely deprioritized.
//
// Among eligible partners, "nearest" is computed with the existing
// Haversine distance against the partner's last known live location.
// Partners are also deprioritized if they already have an active
// (confirmed/shipped) delivery in progress (workload balancing), and a
// partner with a fresh (recent) GPS fix is always preferred over one with a
// stale fix, so load is spread out and distance is trusted only when it's
// actually current.
//
// Concurrency: the read (order + partner list + current load) and the
// write (assigning the picked partner) all happen inside a single DB
// transaction, and the candidate partner rows are locked with SELECT ...
// FOR UPDATE for the duration of that transaction. If two orders become
// ready for dispatch at the same moment, the second call blocks on that
// lock until the first transaction commits, so it always recomputes load
// against up-to-date data and can never pick a partner based on a stale
// snapshot the first call has already acted on. The final write is also a
// conditional UPDATE ... WHERE delivery_partner_id IS NULL, so a
// concurrent manual assignment (admin endpoint) landing in between can
// never be silently overwritten. Partner rows are locked in a fixed (id
// ascending) order so two concurrent assignment transactions can only
// queue behind each other, never deadlock.
func AutoAssignDeliveryPartner(orderID uint) {
    var assignedPartnerID uint
    var assignedPartnerName string

    err := database.DB.Transaction(func(tx *gorm.DB) error {
        var order models.Order
        if err := tx.Preload("Address").First(&order, orderID).Error; err != nil {
            return fmt.Errorf("order %d not found: %w", orderID, err)
        }

        if order.DeliveryPartnerID != nil {
            return errAutoAssignSkipped // already assigned, nothing to do
        }
        if order.Address.Lat == nil || order.Address.Lng == nil {
            log.Printf("[auto-assign] order %d's address has no coordinates, skipping auto-assign", orderID)
            return errAutoAssignSkipped
        }

        serviceable, err := isPincodeServiceable(tx, order.Address.Pincode)
        if err != nil {
            return err
        }
        if !serviceable {
            log.Printf("[auto-assign] order %d's address pincode %q is outside every configured delivery zone, skipping auto-assign", orderID, order.Address.Pincode)
            return errAutoAssignSkipped
        }

        // Lock every active delivery partner row (in a deterministic id
        // order) for the rest of this transaction. This is the crux of the
        // concurrency fix: a second, concurrent AutoAssignDeliveryPartner
        // transaction trying to lock the same rows blocks right here until
        // this transaction commits or rolls back, so partner selection and
        // load counting can never race against another in-flight assignment.
        // Phase 3: only ONLINE (is_online) partners are eligible for NEW
        // assignments, on top of the existing IsActive (admin-enabled) check.
        // A partner going offline never touches orders already assigned to
        // them - this filter only affects who gets picked for a *new* one.
        // A partner also needs a valid (non-null) current GPS fix to be
        // eligible at all - "valid location" is a hard eligibility gate here,
        // distinct from the freshness *preference* applied further down.
        var partners []models.DeliveryPartner
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
            Where("is_active = ? AND is_online = ? AND current_lat IS NOT NULL AND current_lng IS NOT NULL AND checked_in_warehouse_id IS NOT NULL AND checked_in_at > ?", true, true, time.Now().Add(-StoreCheckinMaxAge)).
            Order("id").
            Find(&partners).Error; err != nil {
            return fmt.Errorf("failed to load delivery partners: %w", err)
        }
        if len(partners) == 0 {
            log.Printf("[auto-assign] no eligible (active, online, located) delivery partners available for order %d", orderID)
            return errAutoAssignSkipped
        }

        // Load count of currently active (confirmed/shipped) deliveries per
        // partner, so we can prefer less-loaded partners. Safe to read
        // without an extra lock: the partners we might assign to are
        // already locked above, and this count is only ever bumped by
        // another assignment landing on one of those same locked rows.
        type loadRow struct {
            DeliveryPartnerID uint
            Cnt               int64
        }
        var loads []loadRow
        if err := tx.Model(&models.Order{}).
            Select("delivery_partner_id, count(*) as cnt").
            Where("delivery_partner_id IS NOT NULL AND status IN ? AND updated_at > ?", []string{models.OrderStatusConfirmed, models.OrderStatusPicking, models.OrderStatusPicked, models.OrderStatusPacking, models.OrderStatusPacked, models.OrderStatusReadyForDispatch, models.OrderStatusHandedOver, models.OrderStatusShipped}, time.Now().Add(-12*time.Hour)).
            Group("delivery_partner_id").
            Scan(&loads).Error; err != nil {
            return fmt.Errorf("failed to load partner workloads: %w", err)
        }
        loadByPartner := make(map[uint]int64, len(loads))
        for _, l := range loads {
            loadByPartner[l.DeliveryPartnerID] = l.Cnt
        }

        isCOD := order.PaymentMethod == models.PaymentMethodCOD
        var pendingCOD map[uint]float64
        {
            var pErr error
            pendingCOD, pErr = PendingCODByPartner(tx)
            if pErr != nil {
                return pErr
            }
        }

        custLat, custLng := *order.Address.Lat, *order.Address.Lng
        staleCutoff := time.Now().Add(-staleLocationWindow)

        var bestPartner *models.DeliveryPartner
        var bestScore float64
        var bestHasFreshLocation bool

        for i := range partners {
            p := &partners[i]

            // Capacity: a partner already at (or somehow over) their configured
            // MaxActiveOrders is not a candidate for this assignment at all.
            maxActive := p.MaxActiveOrders
            if maxActive <= 0 {
                maxActive = 5 // safety net for any pre-migration row with a zero value
            }
            if loadByPartner[p.ID] >= int64(maxActive) {
                continue
            }

            if pendingCOD[p.ID] >= CODHardStopLimit {
                continue
            }
            if isCOD && pendingCOD[p.ID] > 0 && pendingCOD[p.ID]+order.TotalAmount > CODCashLimit {
                continue // partner already holds the cash limit
            }

            hasFreshLocation := p.LastLocationUpdate != nil && p.LastLocationUpdate.After(staleCutoff)

            // Location is already guaranteed non-null by the eligibility query
            // above (current_lat/current_lng IS NOT NULL), so distance is always
            // computable here; only its recency affects preference below.
            distanceKm := haversineKm(custLat, custLng, *p.CurrentLat, *p.CurrentLng)

            // Each active delivery adds a fixed "penalty" in km-equivalent
            // terms so a partner already juggling orders isn't picked over a
            // slightly-farther free partner.
            const loadPenaltyKm = 3.0
            score := distanceKm + float64(loadByPartner[p.ID])*loadPenaltyKm

            if bestPartner == nil ||
                (hasFreshLocation && !bestHasFreshLocation) ||
                (hasFreshLocation == bestHasFreshLocation && score < bestScore) {
                bestPartner = p
                bestScore = score
                bestHasFreshLocation = hasFreshLocation
            }
        }

        if bestPartner == nil {
            log.Printf("[auto-assign] could not select a partner for order %d", orderID)
            return errAutoAssignSkipped
        }

        // Conditional, atomic write: only assign if the order is still
        // unassigned. Guards against a concurrent manual assignment (admin
        // endpoint) landing between our read of `order` above and this
        // write - if that happened, RowsAffected is 0 and we treat it as a
        // no-op rather than clobbering the manual assignment.
        //
        // delivery_assignment_expires_at / delivery_attempted_partner_ids
        // start the acceptance-timeout clock and the "already offered to"
        // history for this fresh assignment (see delivery_acceptance.go).
        assignedStatus := models.DeliveryAssignmentStatusAssigned
        result := tx.Model(&models.Order{}).
            Where("id = ? AND delivery_partner_id IS NULL", order.ID).
            Updates(map[string]interface{}{
                "delivery_partner_id":            bestPartner.ID,
                "delivery_assignment_status":     assignedStatus,
                "delivery_status":                models.DeliveryStatusAssigned,
                "delivery_rejection_reason":      nil,
                "delivery_assignment_expires_at": time.Now().Add(AssignmentTimeout()),
                "delivery_attempted_partner_ids": fmt.Sprint(bestPartner.ID),
                "assigned_at":                    time.Now(),
            })
        if result.Error != nil {
            return fmt.Errorf("failed to assign partner %d to order %d: %w", bestPartner.ID, order.ID, result.Error)
        }
        if result.RowsAffected == 0 {
            return errAutoAssignSkipped
        }

        assignedPartnerID = bestPartner.ID
        assignedPartnerName = bestPartner.Name
        return nil
    })

    if err != nil {
        if !errors.Is(err, errAutoAssignSkipped) {
            log.Printf("[auto-assign] failed to assign order %d: %v", orderID, err)
        }
        return
    }

    log.Printf("[auto-assign] order %d assigned to delivery partner %d (%s)", orderID, assignedPartnerID, assignedPartnerName)

    go SendPushToPartnerWithData(assignedPartnerID, "New Order Assigned", "You have a new delivery order. Tap to view and accept it.", map[string]string{"type": "new_assignment", "order_id": fmt.Sprint(orderID)})
CreateDeliveryNotification(
assignedPartnerID,
"New delivery assigned",
fmt.Sprintf("Order #%d has been assigned to you", orderID),
"new_assignment",
&orderID,
)
}


// TryAssignPendingOrdersToPartner looks for orders that are CONFIRMED but
// have never been assigned a delivery partner (delivery_partner_id IS NULL)
// and retries AutoAssignDeliveryPartner for each one. This exists because
// UpdateDeliveryAvailability only flips is_online and previously left
// already-placed orders stuck unassigned until an admin manually assigned
// them or a brand-new order came in after this partner went online.
func TryAssignPendingOrdersToPartner(partnerID uint) {
    var orderIDs []uint
    err := database.DB.Model(&models.Order{}).
        Where("delivery_partner_id IS NULL AND status = ?", models.OrderStatusConfirmed).
        Pluck("id", &orderIDs).Error
    if err != nil {
        log.Printf("[auto-assign] failed to load pending unassigned orders for partner %d: %v", partnerID, err)
        return
    }
    for _, id := range orderIDs {
        AutoAssignDeliveryPartner(id)
    }
}
