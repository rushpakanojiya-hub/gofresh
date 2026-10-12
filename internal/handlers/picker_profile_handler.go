package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var (
	pickerPayoutOnce sync.Once
	pickerReUPI      = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,256}@[a-zA-Z]{2,64}$`)
	pickerReIFSC     = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
	pickerReAcct     = regexp.MustCompile(`^[0-9]{9,18}$`)
)

func ensurePickerPayoutTable() {
	pickerPayoutOnce.Do(func() {
		_ = database.DB.AutoMigrate(&models.PickerPayoutDetail{})
	})
}

func pickerLast4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

func pickerRangeStats(staffID uint, from time.Time) gin.H {
	var tasks []models.PickingTask
	database.DB.Preload("Items").
		Where("picker_id = ? AND status = ? AND completed_at >= ?", staffID, "completed", from).
		Limit(2000).Find(&tasks)
	orders, items, onTime := 0, 0, 0
	for _, t := range tasks {
		needed, picked := 0, 0
		for _, it := range t.Items {
			needed += it.QuantityNeeded
			picked += it.QuantityPicked
		}
		orders++
		items += picked
		if t.StartedAt != nil && t.CompletedAt != nil {
			d := int(t.CompletedAt.Sub(*t.StartedAt).Seconds())
			if pickerSpeedBonus(pickerAllottedSeconds(needed), d) > 0 {
				onTime++
			}
		}
	}
	pct := 0
	if orders > 0 {
		pct = int(float64(onTime)/float64(orders)*100 + 0.5)
	}
	return gin.H{"orders": orders, "items_picked": items, "on_time_pct": pct}
}

// GetPickerProfileDetails GET /warehouse/picker/profile
func GetPickerProfileDetails(c *gin.Context) {
	ensurePickerPayoutTable()
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
	mgrName, mgrPhone := "", ""
	var mgr models.WarehouseStaff
	if err := database.DB.
		Where("warehouse_id = ? AND role = ? AND is_active = ?", staff.WarehouseID, "warehouse_manager", true).
		Order("id ASC").First(&mgr).Error; err == nil {
		mgrName, mgrPhone = mgr.Name, mgr.Phone
	}
	var pay models.PickerPayoutDetail
	database.DB.Where("staff_id = ?", staffID).Limit(1).Find(&pay)

	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Now().In(loc)
	weekStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -6)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	c.JSON(http.StatusOK, gin.H{
		"name":         staff.Name,
		"phone":        staff.Phone,
		"member_since": staff.CreatedAt,
		"store": gin.H{
			"name":    staff.Warehouse.Name,
			"city":    staff.Warehouse.City,
			"address": staff.Warehouse.Address,
		},
		"support": gin.H{"manager_name": mgrName, "manager_phone": mgrPhone},
		"payout": gin.H{
			"upi_id":         pay.UPIID,
			"account_holder": pay.AccountHolder,
			"account_last4":  pickerLast4(pay.AccountNumber),
			"ifsc":           pay.IFSC,
		},
		"week":  pickerRangeStats(staffID, weekStart),
		"month": pickerRangeStats(staffID, monthStart),
	})
}

// SavePickerPayoutDetails PUT /warehouse/picker/payout-details
func SavePickerPayoutDetails(c *gin.Context) {
	ensurePickerPayoutTable()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req struct {
		UPIID         string `json:"upi_id"`
		AccountHolder string `json:"account_holder"`
		AccountNumber string `json:"account_number"`
		IFSC          string `json:"ifsc"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	req.UPIID = strings.TrimSpace(req.UPIID)
	req.AccountHolder = strings.TrimSpace(req.AccountHolder)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	req.IFSC = strings.ToUpper(strings.TrimSpace(req.IFSC))

	var cur models.PickerPayoutDetail
	database.DB.Where("staff_id = ?", staffID).Limit(1).Find(&cur)
	if req.AccountNumber == "" {
		req.AccountNumber = cur.AccountNumber
	}
	if len(req.AccountHolder) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account holder name is too long"})
		return
	}
	if req.UPIID != "" && !pickerReUPI.MatchString(req.UPIID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enter a valid UPI ID like name@bank"})
		return
	}
	if req.AccountNumber != "" {
		if !pickerReAcct.MatchString(req.AccountNumber) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Enter a valid bank account number"})
			return
		}
		if !pickerReIFSC.MatchString(req.IFSC) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Enter a valid IFSC code"})
			return
		}
		if req.AccountHolder == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Enter the account holder name"})
			return
		}
	}
	if req.UPIID == "" && req.AccountNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Add a UPI ID or bank details"})
		return
	}
	rec := models.PickerPayoutDetail{
		StaffID:       staffID,
		UPIID:         req.UPIID,
		AccountHolder: req.AccountHolder,
		AccountNumber: req.AccountNumber,
		IFSC:          req.IFSC,
	}
	if err := database.DB.Save(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save payout details"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Saved"})
}
