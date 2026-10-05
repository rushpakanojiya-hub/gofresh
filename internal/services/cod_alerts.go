package services

import (
    "fmt"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
)

// CODHardStopLimit: at or above this pending COD cash a partner gets no new orders at all (COD or prepaid).
const CODHardStopLimit = 2500.0

// CODWarnLimit: 80% of CODCashLimit, first alert to the rider.
const CODWarnLimit = CODCashLimit * 0.8

// NotifyCODThresholds alerts the rider when this delivery pushed their pending
// COD cash across 80% or 100% of the limit. Alerts only on crossing, so the
// rider is not notified again on every later delivery.
func NotifyCODThresholds(partnerID uint, orderTotal float64, orderID uint) {
    pending, err := PendingCODByPartner(database.DB)
    if err != nil {
        return
    }
    after := pending[partnerID]
    before := after - orderTotal
    if before < 0 {
        before = 0
    }
    oid := orderID
    switch {
    case before < CODCashLimit && after >= CODCashLimit:
        CreateDeliveryNotification(partnerID, "COD limit reached",
            fmt.Sprintf("You are holding Rs %.0f cash. Deposit it now to keep receiving COD orders.", after), "cod", &oid)
    case before < CODWarnLimit && after >= CODWarnLimit:
        CreateDeliveryNotification(partnerID, "COD cash almost at limit",
            fmt.Sprintf("You are holding Rs %.0f of the Rs %.0f limit. Please deposit soon.", after, CODCashLimit), "cod", &oid)
    }
}