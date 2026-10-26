package handlers

import (
    "testing"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// seedTestAdmin creates a real, unblocked admin row, because AdminOnly
// re-checks the users table (BUG-77/78).
func seedTestAdmin(t *testing.T) models.User {
    t.Helper()
    u := models.User{Phone: uniquePhone("8"), Role: "admin"}
    if err := database.DB.Create(&u).Error; err != nil {
        t.Fatalf("failed to seed admin user: %v", err)
    }
    return u
}