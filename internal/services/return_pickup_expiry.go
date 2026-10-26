package services

import (
    "log"
    "time"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// ExpireStaleReturnPickups (BUG-24) clears return pickups that were offered to a
// rider but not accepted within the window, then offers them again. Without this
// an expired pickup stays "assigned" forever and can neither be accepted nor reassigned.
func ExpireStaleReturnPickups() {
    var ids []uint
    if err := database.DB.Model(&models.ReturnRequest{}).
        Where("status = ? AND pickup_status = ? AND pickup_assignment_expires_at IS NOT NULL AND pickup_assignment_expires_at < ?",
            models.ReturnStatusApproved, models.PickupStatusAssigned, time.Now()).
        Pluck("id", &ids).Error; err != nil {
        log.Printf("[return-expiry] lookup failed: %v", err)
        return
    }
    for _, id := range ids {
        res := database.DB.Model(&models.ReturnRequest{}).
            Where("id = ? AND pickup_status = ? AND pickup_assignment_expires_at < ?", id, models.PickupStatusAssigned, time.Now()).
            Updates(map[string]interface{}{"delivery_partner_id": nil, "pickup_status": nil, "pickup_assignment_expires_at": nil})
        if res.Error != nil {
            log.Printf("[return-expiry] clear failed for return %d: %v", id, res.Error)
            continue
        }
        if res.RowsAffected == 1 {
            log.Printf("[return-expiry] return %d pickup expired, reassigning", id)
            AutoAssignReturnPickup(id)
        }
    }
}