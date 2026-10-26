package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ListPayroll godoc
// GET /api/v1/admin/finance/payroll?staff_id=&status=&month=&year=&page=&limit=
func ListPayroll(c *gin.Context) {
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	db := database.DB.Model(&models.Payroll{}).Preload("Staff")
	if staffID := c.Query("staff_id"); staffID != "" {
		db = db.Where("staff_id = ?", staffID)
	}
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	if month := c.Query("month"); month != "" {
		db = db.Where("month = ?", month)
	}
	if year := c.Query("year"); year != "" {
		db = db.Where("year = ?", year)
	}

	var total int64
	db.Count(&total)

	var payrolls []models.Payroll
	offset := (page - 1) * limit
	if err := db.Order("year DESC, month DESC, created_at DESC").Offset(offset).Limit(limit).Find(&payrolls).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll"})
		return
	}

	// BUG-13: totals honour the same staff/month/year filters as the list.
    sumFor := func(st string) float64 {
        q := database.DB.Model(&models.Payroll{}).Where("status = ?", st)
        if v := c.Query("staff_id"); v != "" {
            q = q.Where("staff_id = ?", v)
        }
        if v := c.Query("month"); v != "" {
            q = q.Where("month = ?", v)
        }
        if v := c.Query("year"); v != "" {
            q = q.Where("year = ?", v)
        }
        var s float64
        q.Select("COALESCE(SUM(amount),0)").Scan(&s)
        return s
    }
    totalPending := sumFor("pending")
    totalPaid := sumFor("paid")

	c.JSON(http.StatusOK, gin.H{
		"payroll":       payrolls,
		"page":          page,
		"limit":         limit,
		"total":         total,
		"total_pages":   int((total + int64(limit) - 1) / int64(limit)),
		"total_pending": totalPending,
		"total_paid":    totalPaid,
	})
}

// CreatePayroll godoc
// POST /api/v1/admin/finance/payroll
func CreatePayroll(c *gin.Context) {
	var req models.PayrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, req.StaffID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Staff member not found"})
		return
	}

	status := req.Status
	if status == "" {
		status = "pending"
	}
	if !models.ValidPayrollStatuses[status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}
	if req.PaymentMethod != "" && !models.ValidPaymentMethods[req.PaymentMethod] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment_method"})
		return
	}

	payroll := models.Payroll{
		StaffID:       req.StaffID,
		Amount:        req.Amount,
		Month:         req.Month,
		Year:          req.Year,
		Status:        status,
		PaymentMethod: req.PaymentMethod,
		Note:          req.Note,
	}

	adminID := c.MustGet("user_id").(uint)
	if status == "paid" {
		now := time.Now()
		payroll.PaidByID = &adminID
		payroll.PaidAt = &now
	}

	if err := database.DB.Create(&payroll).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payroll record"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "create_payroll", "payroll", strconv.Itoa(int(payroll.ID)), "created")

	database.DB.Preload("Staff").First(&payroll, payroll.ID)
	c.JSON(http.StatusCreated, payroll)
}

// UpdatePayroll godoc
// PUT /api/v1/admin/finance/payroll/:id
func UpdatePayroll(c *gin.Context) {
	id := c.Param("id")
	var payroll models.Payroll
	if err := database.DB.First(&payroll, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll record not found"})
		return
	}

	var req models.PayrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var staff models.WarehouseStaff
	if err := database.DB.First(&staff, req.StaffID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Staff member not found"})
		return
	}

	status := req.Status
	if status == "" {
		status = "pending"
	}
	if !models.ValidPayrollStatuses[status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}
	if req.PaymentMethod != "" && !models.ValidPaymentMethods[req.PaymentMethod] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment_method"})
		return
	}

	wasPending := payroll.Status != "paid"
	payroll.StaffID = req.StaffID
	payroll.Amount = req.Amount
	payroll.Month = req.Month
	payroll.Year = req.Year
	payroll.Status = status
	payroll.PaymentMethod = req.PaymentMethod
	payroll.Note = req.Note

	adminID := c.MustGet("user_id").(uint)
	if status == "paid" && wasPending {
		now := time.Now()
		payroll.PaidByID = &adminID
		payroll.PaidAt = &now
	}

	if err := database.DB.Save(&payroll).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payroll record"})
		return
	}

	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "update_payroll", "payroll", id, "updated")

	database.DB.Preload("Staff").First(&payroll, payroll.ID)
	c.JSON(http.StatusOK, payroll)
}

// DeletePayroll godoc
// DELETE /api/v1/admin/finance/payroll/:id
func DeletePayroll(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&models.Payroll{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete payroll record"})
		return
	}

	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "delete_payroll", "payroll", id, "deleted")

	c.JSON(http.StatusOK, gin.H{"success": true})
}
