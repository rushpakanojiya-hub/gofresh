package services

import (
    "fmt"
    "strings"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

// clearAttemptedPartner removes one partner from an unassigned order's
// attempted list, so a fresh check-in scan makes them eligible again.
// BUG-53: the read-modify-write now runs in a transaction under a row lock.
func clearAttemptedPartner(orderID, partnerID uint) {
    _ = database.DB.Transaction(func(tx *gorm.DB) error {
        var o models.Order
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
            Select("id", "delivery_attempted_partner_ids", "delivery_partner_id").
            First(&o, orderID).Error; err != nil {
            return err
        }
        if o.DeliveryPartnerID != nil {
            return nil
        }
        m := parseAttemptedIDs(o.DeliveryAttemptedPartnerIDs)
        if !m[partnerID] {
            return nil
        }
        delete(m, partnerID)
        parts := make([]string, 0, len(m))
        for id := range m {
            parts = append(parts, fmt.Sprint(id))
        }
        return tx.Model(&models.Order{}).
            Where("id = ? AND delivery_partner_id IS NULL", orderID).
            Update("delivery_attempted_partner_ids", strings.Join(parts, ",")).Error
    })
}