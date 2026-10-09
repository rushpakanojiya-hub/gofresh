package handlers

import (
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

type opsOrderOut struct {
	models.Order
	PickerID   *uint  `json:"picker_id"`
	PickerName string `json:"picker_name"`
	TaskStatus string `json:"task_status"`
	Delayed    bool   `json:"delayed"`
}

// opsEnrichOrders adds picker and delay info to a page of warehouse orders.
func opsEnrichOrders(wid uint, orders []models.Order) []opsOrderOut {
	out := make([]opsOrderOut, 0, len(orders))
	if len(orders) == 0 {
		return out
	}
	ids := make([]uint, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}

	var tasks []models.PickingTask
	database.DB.Where("warehouse_id = ? AND order_id IN ?", wid, ids).Find(&tasks)
	taskByOrder := make(map[uint]models.PickingTask, len(tasks))
	staffIDs := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		taskByOrder[t.OrderID] = t
		if t.PickerID != nil {
			staffIDs = append(staffIDs, *t.PickerID)
		}
	}
	names := map[uint]string{}
	if len(staffIDs) > 0 {
		var staff []models.WarehouseStaff
		database.DB.Where("id IN ?", staffIDs).Find(&staff)
		for _, s := range staff {
			names[s.ID] = s.Name
		}
	}

	newCutoff := opsNewCutoff()
	progCutoff := opsProgressCutoff()
	inProgress := map[string]bool{
		models.OrderStatusPicking: true, models.OrderStatusPicked: true,
		models.OrderStatusPacking: true, models.OrderStatusPacked: true,
		models.OrderStatusReadyForDispatch: true,
	}
	for _, o := range orders {
		r := opsOrderOut{Order: o}
		if t, ok := taskByOrder[o.ID]; ok {
			r.PickerID = t.PickerID
			r.TaskStatus = t.Status
			if t.PickerID != nil {
				r.PickerName = names[*t.PickerID]
			}
		}
		r.Delayed = (o.Status == models.OrderStatusConfirmed && o.CreatedAt.Before(newCutoff)) ||
			(inProgress[o.Status] && o.UpdatedAt.Before(progCutoff))
		out = append(out, r)
	}
	_ = time.Now
	return out
}
