package services

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrAssignmentOrderNotOwned / ErrAssignmentNotPending are the sentinel
// errors RespondToAssignment returns for the two ways an accept/reject
// request can be legitimately refused.
var (
	ErrAssignmentOrderNotOwned = errors.New("order not found or not assigned to you")
	ErrAssignmentNotPending    = errors.New("assignment is not pending")
	AutoReassignOnReject       = true
)

// AssignmentTimeout returns the configured acceptance window (how long a
// partner has to accept/reject before the offer expires), defaulting to 5
// minutes if not configured.
func AssignmentTimeout() time.Duration {
	minutes := 5
	if config.AppConfig != nil && config.AppConfig.DeliveryAssignmentTimeoutMinutes > 0 {
		minutes = config.AppConfig.DeliveryAssignmentTimeoutMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func parseAttemptedIDs(csv string) map[uint]bool {
	ids := make(map[uint]bool)
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.ParseUint(part, 10, 64); err == nil {
			ids[uint(n)] = true
		}
	}
	return ids
}

func appendAttemptedID(csv string, id uint) string {
	ids := parseAttemptedIDs(csv)
	ids[id] = true
	parts := make([]string, 0, len(ids))
	for pid := range ids {
		parts = append(parts, strconv.FormatUint(uint64(pid), 10))
	}
	return strings.Join(parts, ",")
}

// RespondToAssignment lets the assigned delivery partner accept or reject
// their pending assignment. It's the single source of truth for that state
// transition - shared by the PUT /delivery/orders/:id/accept and
// PUT /delivery/orders/:id/reject handlers, and by tests.
//
// Ownership is enforced by scoping the lookup to
// "id = ? AND delivery_partner_id = ?" using the *caller-supplied*
// partnerID (handlers set this from the verified JWT, never from a
// client-supplied field), so one partner can never accept or discover
// another partner's order (IDOR/BOLA protection).
//
// The transition only succeeds from the ASSIGNED state, enforced both by
// an explicit check and by a conditional UPDATE ... WHERE
// delivery_assignment_status = 'assigned', so two concurrent responses to
// the same assignment (e.g. a double-tap, or a response racing an
// in-flight timeout expiry) can only ever have one winner - the order is
// never assigned/responded to twice.
//
// On rejection, the next eligible partner (excluding everyone already
// offered this order) is automatically tried in the background - see
// TryAssignNextPartner. The existing push-notification system
// (SendPushToPartner) is reused for that, exactly as auto-assign and
// manual admin assignment already do.
func RespondToAssignment(orderID, partnerID uint, newStatus string, reason string) (*models.Order, error) {
	var order models.Order
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND delivery_partner_id = ?", orderID, partnerID).
			First(&order).Error; err != nil {
			return ErrAssignmentOrderNotOwned
		}

		if order.DeliveryAssignmentStatus == nil || *order.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusAssigned {
			return ErrAssignmentNotPending
		}

		updates := map[string]interface{}{"delivery_assignment_status": newStatus}
		if newStatus == models.DeliveryAssignmentStatusRejected && reason != "" {
			updates["delivery_rejection_reason"] = reason
		}
		if newStatus == models.DeliveryAssignmentStatusAccepted {
			updates["delivery_assignment_expires_at"] = nil
			updates["delivery_status"] = models.DeliveryStatusAccepted
		}

		result := tx.Model(&models.Order{}).
			Where("id = ? AND delivery_partner_id = ? AND delivery_assignment_status = ?",
				order.ID, partnerID, models.DeliveryAssignmentStatusAssigned).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("failed to update assignment: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrAssignmentNotPending
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Re-fetch into a fresh struct rather than reusing the existing `order`
	// variable - GORM's scanner does not overwrite an already non-nil pointer
	// field (like DeliveryAssignmentExpiresAt) with nil when scanning a SQL
	// NULL back into a reused struct, so a stale expiry would otherwise
	// survive in the in-memory value even though the DB column was cleared.
	var freshOrder models.Order
	database.DB.Preload("Address").Preload("Items").First(&freshOrder, order.ID)
	order = freshOrder

	if newStatus == models.DeliveryAssignmentStatusRejected && AutoReassignOnReject {
		go TryAssignNextPartner(order.ID)
	}

	return &order, nil
}

// pickEligiblePartnerExcluding selects the best eligible delivery partner
// for order's address, using the same eligibility/scoring rules as
// AutoAssignDeliveryPartner (active, online, located, spare capacity,
// nearest-first with a load penalty), but skips any partner ID in
// exclude. Kept separate from AutoAssignDeliveryPartner intentionally, so
// that function's existing, already-tested selection logic is never
// touched by this feature.
func pickEligiblePartnerExcluding(tx *gorm.DB, order *models.Order, exclude map[uint]bool) (*models.DeliveryPartner, error) {
	if order.Address.Lat == nil || order.Address.Lng == nil {
		return nil, nil
	}

	var partners []models.DeliveryPartner
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("is_active = ? AND is_online = ? AND current_lat IS NOT NULL AND current_lng IS NOT NULL AND checked_in_warehouse_id IS NOT NULL AND checked_in_at > ?", true, true, time.Now().Add(-StoreCheckinMaxAge)).
		Order("id").
		Find(&partners).Error; err != nil {
		return nil, fmt.Errorf("failed to load delivery partners: %w", err)
	}
	if len(partners) == 0 {
		return nil, nil
	}

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
		return nil, fmt.Errorf("failed to load partner workloads: %w", err)
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
                        return nil, pErr
                }
        }

        custLat, custLng := *order.Address.Lat, *order.Address.Lng
	staleCutoff := time.Now().Add(-staleLocationWindow)

	var best *models.DeliveryPartner
	var bestScore float64
	var bestFresh bool

	for i := range partners {
		p := &partners[i]
		if exclude[p.ID] {
			continue
		}
		maxActive := p.MaxActiveOrders
		if maxActive <= 0 {
			maxActive = 5
		}
		if loadByPartner[p.ID] >= int64(maxActive) {
			continue
		}
		if pendingCOD[p.ID] >= CODHardStopLimit {
                continue
            }
            if isCOD && pendingCOD[p.ID] > 0 && pendingCOD[p.ID]+order.TotalAmount > CODCashLimit {
                        continue
                }
                fresh := p.LastLocationUpdate != nil && p.LastLocationUpdate.After(staleCutoff)
		distanceKm := haversineKm(custLat, custLng, *p.CurrentLat, *p.CurrentLng)
		const loadPenaltyKm = 3.0
		score := distanceKm + float64(loadByPartner[p.ID])*loadPenaltyKm

		if best == nil ||
            p.CheckedInAt.Before(*best.CheckedInAt) ||
            (p.CheckedInAt.Equal(*best.CheckedInAt) &&
                ((fresh && !bestFresh) || (fresh == bestFresh && score < bestScore))) {
			best = p
			bestScore = score
			bestFresh = fresh
		}
	}
	return best, nil
}

// TryAssignNextPartner is called after an assignment is rejected or
// expires. It looks for the next eligible partner who hasn't already been
// offered this order and gives them a fresh acceptance window. If nobody
// is left, the order is left unassigned (delivery_partner_id = NULL) with
// its terminal assignment status (rejected/expired) intact, for an admin
// to handle manually - the order is never silently lost.
//
// Guarded by locking the order row for the transaction's duration and by a
// conditional UPDATE ... WHERE delivery_assignment_status = <the status
// just read>, so a concurrent call (e.g. a duplicate timeout tick, or a
// reject racing an expiry sweep) can never assign the same order twice.
func TryAssignNextPartner(orderID uint) {
	var newPartnerID uint
	var newPartnerName string
	var exhausted bool

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Address").First(&order, orderID).Error; err != nil {
			return fmt.Errorf("order %d not found: %w", orderID, err)
		}

		// Only act on an order that is actually in a terminal
		// rejected/expired state - guards against acting twice on the
		// same event and against racing a fresh manual/auto assignment
		// that landed in between.
		if order.DeliveryAssignmentStatus == nil ||
			(*order.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusRejected &&
				*order.DeliveryAssignmentStatus != models.DeliveryAssignmentStatusExpired) {
			return errAutoAssignSkipped
		}

		serviceable, err := isPincodeServiceable(tx, order.Address.Pincode)
		if err != nil {
			return err
		}
		if !serviceable {
			// Persist the clear (not a rollback-triggering skip) - returning
			// errAutoAssignSkipped here used to roll back this very update,
			// since gorm.DB.Transaction rolls back on ANY non-nil closure
			// return, sentinel or not.
			result := tx.Model(&models.Order{}).
				Where("id = ? AND delivery_assignment_status = ?", order.ID, *order.DeliveryAssignmentStatus).
				Update("delivery_partner_id", nil)
			if result.Error != nil {
				return fmt.Errorf("failed to clear unserviceable order %d: %w", order.ID, result.Error)
			}
			exhausted = true
			return nil
		}

		exclude := parseAttemptedIDs(order.DeliveryAttemptedPartnerIDs)
		if order.DeliveryPartnerID != nil {
			exclude[*order.DeliveryPartnerID] = true
		}

		best, err := pickEligiblePartnerExcluding(tx, &order, exclude)
		if err != nil {
			return err
		}

		statusToMatch := *order.DeliveryAssignmentStatus

		if best == nil {
			// Nobody left to try - clear the partner so the order shows as
			// unassigned/needs-attention, but keep the terminal status as a
			// record of what happened.
			result := tx.Model(&models.Order{}).
				Where("id = ? AND delivery_assignment_status = ?", order.ID, statusToMatch).
				Update("delivery_partner_id", nil)
			if result.Error != nil {
				return fmt.Errorf("failed to clear exhausted order %d: %w", order.ID, result.Error)
			}
			exhausted = true
			return nil
		}

		newExpiry := time.Now().Add(AssignmentTimeout())
		newAttempted := appendAttemptedID(order.DeliveryAttemptedPartnerIDs, best.ID)
		assignedStatus := models.DeliveryAssignmentStatusAssigned

		result := tx.Model(&models.Order{}).
			Where("id = ? AND delivery_assignment_status = ?", order.ID, statusToMatch).
			Updates(map[string]interface{}{
				"delivery_partner_id":            best.ID,
				"delivery_assignment_status":     assignedStatus,
				"delivery_rejection_reason":      nil,
				"delivery_assignment_expires_at": newExpiry,
				"delivery_attempted_partner_ids": newAttempted,
				"delivery_status":                models.DeliveryStatusAssigned,
			})
		if result.Error != nil {
			return fmt.Errorf("failed to reassign order %d: %w", order.ID, result.Error)
		}
		if result.RowsAffected == 0 {
			return errAutoAssignSkipped
		}

		newPartnerID = best.ID
		newPartnerName = best.Name
		return nil
	})

	if err != nil {
		if !errors.Is(err, errAutoAssignSkipped) {
			log.Printf("[reassign] failed for order %d: %v", orderID, err)
		}
		return
	}

	if exhausted {
		log.Printf("[reassign] order %d has no remaining eligible delivery partners; left unassigned for manual handling", orderID)
		return
	}

	log.Printf("[reassign] order %d reassigned to delivery partner %d (%s)", orderID, newPartnerID, newPartnerName)

	go SendPushToPartnerWithData(newPartnerID, "New Order Assigned", "You have a new delivery order. Tap to view and accept it.", map[string]string{"type": "new_assignment", "order_id": fmt.Sprint(orderID)})
}

// ExpireStaleAssignments finds every order whose current acceptance window
// has passed without a response, moves it to EXPIRED, and tries the next
// eligible partner for each. Intended to run on a short interval (e.g.
// every minute) from a cron job - see cmd/api/main.go.
func ExpireStaleAssignments() {
	var orderIDs []uint
	err := database.DB.Model(&models.Order{}).
		Where("delivery_assignment_status = ? AND delivery_assignment_expires_at IS NOT NULL AND delivery_assignment_expires_at < ?",
			models.DeliveryAssignmentStatusAssigned, time.Now()).
		Pluck("id", &orderIDs).Error
	if err != nil {
		log.Printf("[reassign] failed to scan for expired assignments: %v", err)
		return
	}

	for _, id := range orderIDs {
		result := database.DB.Model(&models.Order{}).
			Where("id = ? AND delivery_assignment_status = ?", id, models.DeliveryAssignmentStatusAssigned).
			Update("delivery_assignment_status", models.DeliveryAssignmentStatusExpired)
		if result.Error != nil {
			log.Printf("[reassign] failed to mark order %d expired: %v", id, result.Error)
			continue
		}
		if result.RowsAffected == 0 {
			// Someone else (accept/reject landing at the same instant) beat
			// the scanner to it.
			continue
		}
		log.Printf("[reassign] order %d's assignment expired, trying next partner", id)
		TryAssignNextPartner(id)
	}
}
