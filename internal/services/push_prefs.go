package services

import (
	"database/sql"
	"encoding/json"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
)

// PushAllowed reports whether the user wants pushes of the given category
// (orders, offers, wallet, recommendations, updates). Missing prefs or any
// error means "allowed", so a broken lookup never silences order updates.
func PushAllowed(userID uint, category string) bool {
	var raw sql.NullString
	if err := database.DB.Table("users").Select("notification_prefs").Where("id = ?", userID).Scan(&raw).Error; err != nil || !raw.Valid || raw.String == "" {
		return true
	}
	var prefs map[string]bool
	if json.Unmarshal([]byte(raw.String), &prefs) != nil {
		return true
	}
	if v, ok := prefs[category]; ok {
		return v
	}
	return true
}
