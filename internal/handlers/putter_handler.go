package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var putterTablesOnce sync.Once

func ensurePutterTables() {
	putterTablesOnce.Do(func() {
		_ = database.DB.AutoMigrate(&models.PutterShift{})
	})
}

// PutterRoleOnly lets only active warehouse staff with role "putter" use the
// putter app routes. Managers/pickers get 403.
func PutterRoleOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID, ok := pickerStaffID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		var st models.WarehouseStaff
		if err := database.DB.Select("id", "role", "is_active").First(&st, staffID).Error; err != nil || st.Role != "putter" || !st.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Only putter accounts can use the putter app"})
			return
		}
		c.Next()
	}
}

func putterActiveShift(staffID uint) (models.PutterShift, bool) {
	var s models.PutterShift
	res := database.DB.Where("staff_id = ? AND ended_at IS NULL", staffID).Order("id DESC").Limit(1).Find(&s)
	return s, res.Error == nil && res.RowsAffected > 0
}

// GetPutterDashboard GET /warehouse/putter/dashboard
func GetPutterDashboard(c *gin.Context) {
	ensurePutterTables()
	EnsurePutawayTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var staff models.WarehouseStaff
	if err := database.DB.Preload("Warehouse").First(&staff, staffID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Staff not found"})
		return
	}
	wid := staff.WarehouseID
	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Now().In(loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	mine := func() *gorm.DB {
		return database.DB.Where("warehouse_id = ? AND putter_id = ?", wid, staffID)
	}
	var pending, inProgress, done []models.PutawayTask
	opsPutawayPreload(mine().Where("status = ?", models.PutawayPending)).Order("created_at ASC").Limit(100).Find(&pending)
	opsPutawayPreload(mine().Where("status = ?", models.PutawayInProgress)).Order("started_at ASC").Limit(100).Find(&inProgress)
	opsPutawayPreload(mine().Where("status = ? AND completed_at >= ?", models.PutawayCompleted, dayStart)).Order("completed_at DESC").Limit(100).Find(&done)

	var shifts []models.PutterShift
	database.DB.Where("staff_id = ? AND started_at >= ?", staffID, dayStart).Find(&shifts)
	total := 0
	for _, s := range shifts {
		end := time.Now()
		if s.EndedAt != nil {
			end = *s.EndedAt
		}
		total += int(end.Sub(s.StartedAt).Seconds())
	}
	active, hasActive := putterActiveShift(staffID)
	shift := gin.H{"active": hasActive, "today_seconds": total, "started_at": nil}
	if hasActive {
		shift["started_at"] = active.StartedAt
	}

	c.JSON(http.StatusOK, gin.H{
		"staff": gin.H{
			"id":          staff.ID,
			"name":        staff.Name,
			"phone":       staff.Phone,
			"employee_id": fmt.Sprintf("PUT-%04d", staff.ID),
		},
		"store":       gin.H{"name": staff.Warehouse.Name, "city": staff.Warehouse.City},
		"shift":       shift,
		"counts":      gin.H{"pending": len(pending), "in_progress": len(inProgress), "completed_today": len(done)},
		"pending":     opsPutawayRows(pending),
		"in_progress": opsPutawayRows(inProgress),
		"completed":   opsPutawayRows(done),
	})
}

// StartPutterShift POST /warehouse/putter/shift/start
func StartPutterShift(c *gin.Context) {
	ensurePutterTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	code := http.StatusInternalServerError
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var st models.WarehouseStaff
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&st, staffID).Error; e != nil {
			code = http.StatusNotFound
			return errors.New("staff not found")
		}
		var n int64
		tx.Model(&models.PutterShift{}).Where("staff_id = ? AND ended_at IS NULL", staffID).Count(&n)
		if n > 0 {
			code = http.StatusConflict
			return errors.New("Your shift is already running")
		}
		return tx.Create(&models.PutterShift{StaffID: staffID, WarehouseID: st.WarehouseID, StartedAt: time.Now()}).Error
	})
	if err != nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// EndPutterShift POST /warehouse/putter/shift/end
func EndPutterShift(c *gin.Context) {
	ensurePutterTables()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	res := database.DB.Model(&models.PutterShift{}).
		Where("staff_id = ? AND ended_at IS NULL", staffID).
		Update("ended_at", time.Now())
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not end the shift"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "No running shift to end"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
