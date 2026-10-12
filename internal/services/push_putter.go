package services

import (
	"context"
	"log"

	"firebase.google.com/go/v4/messaging"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	fb "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/firebase"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// SendPushToPutter sends a push to every device registered by a warehouse
// putter (device_tokens.staff_id). Customer/delivery push is untouched.
func SendPushToPutter(staffID uint, title string, body string, data map[string]string) {
	if fb.Client == nil {
		log.Println("Firebase not initialized, skipping putter push")
		return
	}
	var tokens []models.DeviceToken
	if err := database.DB.Where("staff_id = ?", staffID).Find(&tokens).Error; err != nil {
		log.Printf("Failed to load device tokens for putter %d: %v", staffID, err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	ctx := context.Background()
	for _, t := range tokens {
		msg := &messaging.Message{
			Notification: &messaging.Notification{Title: title, Body: body},
			Data:         data,
			Android:      &messaging.AndroidConfig{Priority: "high"},
			Token:        t.Token,
		}
		if _, err := fb.Client.Send(ctx, msg); err != nil {
			log.Printf("Putter push failed for token %s: %v", t.Token, err)
			if messaging.IsRegistrationTokenNotRegistered(err) || messaging.IsInvalidArgument(err) {
				database.DB.Delete(&t)
			}
			continue
		}
	}
	log.Printf("Putter push sent to staff %d (%d device(s)): %s", staffID, len(tokens), title)
}
