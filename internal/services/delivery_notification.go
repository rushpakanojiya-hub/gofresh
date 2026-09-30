package services

import (
"log"

"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// CreateDeliveryNotification writes an in-app notification for a delivery
// partner. Errors are logged, not returned - a failed notification write
// should never fail the operation that triggered it (e.g. an assignment
// should still succeed even if the notification insert fails).
func CreateDeliveryNotification(partnerID uint, title, message, notifType string, orderID *uint) {
notification := models.DeliveryNotification{
DeliveryPartnerID: partnerID,
Title:             title,
Message:           message,
Type:              notifType,
OrderID:           orderID,
}
if err := database.DB.Create(&notification).Error; err != nil {
log.Printf("failed to create delivery notification for partner %d: %v", partnerID, err)
}
}
