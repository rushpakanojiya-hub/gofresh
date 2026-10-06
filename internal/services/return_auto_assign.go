package services

import (
	"fmt"
	"log"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AutoAssignReturnPickup assigns an approved, unassigned return pickup to the
// rider who checked in earliest at the order's store (FIFO, same idea as
// forward-delivery auto-assign). Riders who already hold an active return
// pickup (>= MaxActiveOrders) are skipped. No-op if nobody is eligible; the
// return stays pending until TryAssignPendingReturnsToPartner picks it up
// after the next store check-in.
func AutoAssignReturnPickup(returnReqID uint) {
	var assignedID uint
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var rr models.ReturnRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rr, returnReqID).Error; err != nil {
			return err
		}
		if rr.Status != models.ReturnStatusApproved {
			return errAutoAssignSkipped
		}
		if rr.PickupStatus != nil && *rr.PickupStatus != "" {
			return errAutoAssignSkipped
		}

		var order models.Order
		if err := tx.Select("id", "warehouse_id").First(&order, rr.OrderID).Error; err != nil {
			return err
		}
		if order.WarehouseID == nil {
			return errAutoAssignSkipped
		}

		var partners []models.DeliveryPartner
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("is_active = ? AND is_online = ? AND checked_in_warehouse_id = ? AND checked_in_at > ?",
				true, true, *order.WarehouseID, time.Now().Add(-StoreCheckinMaxAge)).
			Order("checked_in_at ASC, id ASC").
			Find(&partners).Error; err != nil {
			return err
		}
		if len(partners) == 0 {
			log.Printf("[return-assign] no checked-in rider for return %d", returnReqID)
			return errAutoAssignSkipped
		}

		type loadRow struct {
			DeliveryPartnerID uint
			Cnt               int64
		}
		var rows []loadRow
		if err := tx.Model(&models.ReturnRequest{}).
			Select("delivery_partner_id, count(*) as cnt").
			Where("delivery_partner_id IS NOT NULL AND pickup_status IN ?", []string{
				models.PickupStatusAssigned, models.PickupStatusAccepted, models.PickupStatusEnRoute,
				models.PickupStatusArrived, models.PickupStatusPickedUp,
			}).
			Group("delivery_partner_id").
			Scan(&rows).Error; err != nil {
			return err
		}
		load := make(map[uint]int64, len(rows))
		for _, r := range rows {
			load[r.DeliveryPartnerID] = r.Cnt
		}

		var chosen *models.DeliveryPartner
		for i := range partners {
			p := &partners[i]
			maxActive := p.MaxActiveOrders
			if maxActive <= 0 {
				maxActive = 5
			}
			if load[p.ID] >= int64(maxActive) {
				continue
			}
			chosen = p
			break
		}
		if chosen == nil {
			return errAutoAssignSkipped
		}

		res := tx.Model(&models.ReturnRequest{}).
			Where("id = ? AND (pickup_status IS NULL OR pickup_status = '')", rr.ID).
			Updates(map[string]interface{}{
				"delivery_partner_id":          chosen.ID,
				"pickup_status":                models.PickupStatusAssigned,
				"pickup_assignment_expires_at": time.Now().Add(returnPickupAcceptWindow),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errAutoAssignSkipped
		}
		assignedID = chosen.ID
		return nil
	})
	if err != nil {
		if err != errAutoAssignSkipped {
			log.Printf("[return-assign] failed for return %d: %v", returnReqID, err)
		}
		return
	}
	log.Printf("[return-assign] return %d assigned to partner %d", returnReqID, assignedID)
	go SendPushToPartnerWithData(assignedID, "New Return Pickup", "You have a new return pickup. Tap to view and accept it.",
		map[string]string{"type": "new_return_pickup", "return_request_id": fmt.Sprint(returnReqID)})
}

// TryAssignPendingReturnsToPartner retries every approved-but-unassigned
// return. Call it after a rider checks in or finishes a task.
func TryAssignPendingReturnsToPartner(partnerID uint) {
	var ids []uint
	if err := database.DB.Model(&models.ReturnRequest{}).
		Where("status = ? AND delivery_partner_id IS NULL AND (pickup_status IS NULL OR pickup_status = '')", models.ReturnStatusApproved).
		Order("created_at ASC").
		Pluck("id", &ids).Error; err != nil {
		log.Printf("[return-assign] pending lookup failed for partner %d: %v", partnerID, err)
		return
	}
	for _, id := range ids {
		AutoAssignReturnPickup(id)
	}
}
