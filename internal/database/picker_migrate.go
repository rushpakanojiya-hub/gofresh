package database

import (
	"log"
	"sync"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var pickerOnce sync.Once

// EnsurePickerTables creates the picker tables once per process and seeds
// default slots (09:00-13:00 and 17:00-21:00) for active warehouses if no slots exist yet.
func EnsurePickerTables() {
	pickerOnce.Do(func() {
		if err := DB.AutoMigrate(&models.PickerSlot{}, &models.PickerSlotBooking{}, &models.PickerPresence{}); err != nil {
			log.Printf("picker: automigrate failed: %v", err)
			return
		}
		var n int64
		DB.Model(&models.PickerSlot{}).Count(&n)
		if n > 0 {
			return
		}
		var whs []models.Warehouse
		DB.Where("is_active = ?", true).Find(&whs)
		for _, w := range whs {
			for _, t := range [][2]string{{"09:00", "13:00"}, {"17:00", "21:00"}} {
				DB.Create(&models.PickerSlot{WarehouseID: w.ID, StartTime: t[0], EndTime: t[1], Capacity: 10, IsActive: true})
			}
		}
	})
}
