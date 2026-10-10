package services

import (
    "time"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// Every 30s, retry auto-assign for ready_for_dispatch orders that still
// have no delivery partner (last 12h only).
func init() {
    go dispatchRetryLoop()
}

func dispatchRetryLoop() {
    t := time.NewTicker(30 * time.Second)
    for range t.C {
        if database.DB == nil {
            continue
        }
        var ids []uint
        if err := database.DB.Model(&models.Order{}).
            Where("delivery_partner_id IS NULL AND status = ? AND created_at > ?", models.OrderStatusReadyForDispatch, time.Now().Add(-12*time.Hour)).
            Order("created_at ASC").
            Pluck("id", &ids).Error; err != nil {
            continue
        }
        for _, id := range ids {
            AutoAssignDeliveryPartner(id)
        }
    }
}