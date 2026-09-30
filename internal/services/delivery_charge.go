package services

import (
"math"
"strings"

"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// Delivery charge pricing: a base fee plus a per-km charge from the
// nearest active warehouse, capped at maxDeliveryFee so a far-out address
// doesn't produce an absurd charge. If the address has no coordinates yet
// (older addresses saved before lat/lng was collected) or there's no
// active warehouse to measure from, we fall back to the flat charge that
// was used before distance-based pricing existed.
const (
baseDeliveryFee    = 20.0
perKmDeliveryFee   = 8.0
maxDeliveryFee     = 150.0
fallbackFlatCharge = 50.0
fallbackEstimatedDays = 3
)

// ServiceabilityResult is what checkout needs to know about how (and
// whether) an address can be served: the delivery charge, whether COD is
// allowed there, the estimated delivery time, and which DeliveryZone (if
// any) produced these numbers. ZoneID is nil when no active zone matched
// this pincode and the haversine-distance fallback was used instead.
type ServiceabilityResult struct {
DeliveryCharge float64
CODAvailable   bool
EstimatedDays  int
Serviceable    bool
ZoneID         *uint
}

// findMatchingZone returns the first active DeliveryZone whose comma-separated
// Pincodes list contains the given pincode, or nil if none match. Matching is
// exact-pincode, not prefix/range - a zone must explicitly list a pincode to
// serve it.
func findMatchingZone(pincode string) *models.DeliveryZone {
if pincode == "" {
return nil
}
var zones []models.DeliveryZone
if err := database.DB.Where("is_active = ?", true).Find(&zones).Error; err != nil {
return nil
}
for i := range zones {
for _, p := range strings.Split(zones[i].Pincodes, ",") {
if strings.TrimSpace(p) == pincode {
return &zones[i]
}
}
}
return nil
}

// CalculateServiceability is the single source of truth for "can we deliver
// here, what does it cost, is COD allowed, and how long will it take" -
// used by both Checkout and GetCheckoutEstimate so the two can never
// disagree with each other or with what admin configured in DeliveryZones.
//
// Priority order (see SRS discussion on serviceability):
//  1. An active DeliveryZone whose Pincodes list contains this address's
//     pincode wins outright - its DeliveryCharge, IsCODAvailable and
//     EstimatedDays are authoritative.
//  2. If no zone matches, fall back to the pre-existing haversine-distance
//     pricing from the nearest active warehouse, with COD allowed and the
//     flat fallbackEstimatedDays - this preserves existing behavior for
//     areas admin hasn't explicitly zoned yet, rather than blocking
//     checkout entirely for pincodes nobody has configured.
func CalculateServiceability(pincode string, addrLat, addrLng *float64) ServiceabilityResult {
if zone := findMatchingZone(pincode); zone != nil {
zoneID := zone.ID
return ServiceabilityResult{
DeliveryCharge: zone.DeliveryCharge,
CODAvailable:   zone.IsCODAvailable,
EstimatedDays:  zone.EstimatedDays,
Serviceable:    true,
ZoneID:         &zoneID,
}
}

// No configured zone for this pincode - fall back to distance-based
// pricing exactly as before, with COD allowed by default.
	var activeZoneCount int64
	database.DB.Model(&models.DeliveryZone{}).Where("is_active = ?", true).Count(&activeZoneCount)
	if activeZoneCount > 0 {
		return ServiceabilityResult{
			Serviceable: false,
		}
	}
	return ServiceabilityResult{
DeliveryCharge: calculateHaversineDeliveryCharge(addrLat, addrLng),
CODAvailable:   true,
EstimatedDays:  fallbackEstimatedDays,
Serviceable:    true,
ZoneID:         nil,
}
}

// CalculateDeliveryCharge is kept for any caller that only needs the charge
// number and doesn't care about zone/COD/ETA - it now goes through the same
// zone-aware logic as CalculateServiceability instead of always using the
// haversine fallback, so a caller can't accidentally bypass zone pricing.
func CalculateDeliveryCharge(pincode string, addrLat, addrLng *float64) float64 {
return CalculateServiceability(pincode, addrLat, addrLng).DeliveryCharge
}

// calculateHaversineDeliveryCharge is the original distance-based pricing,
// used only when no DeliveryZone matches the address's pincode.
func calculateHaversineDeliveryCharge(addrLat, addrLng *float64) float64 {
if addrLat == nil || addrLng == nil {
return fallbackFlatCharge
}

var warehouses []models.Warehouse
if err := database.DB.Where("is_active = ?", true).Find(&warehouses).Error; err != nil || len(warehouses) == 0 {
return fallbackFlatCharge
}

nearestKm := math.MaxFloat64
for _, w := range warehouses {
d := haversineKm(*addrLat, *addrLng, w.Lat, w.Lng)
if d < nearestKm {
nearestKm = d
}
}

charge := baseDeliveryFee + perKmDeliveryFee*nearestKm
if charge > maxDeliveryFee {
charge = maxDeliveryFee
}
// Round to 2 decimal places.
return math.Round(charge*100) / 100
}
