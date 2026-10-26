package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// GetCustomers godoc
// GET /api/v1/admin/customers (admin only)
func GetCustomers(c *gin.Context) {
	var query models.CustomerListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}

	db := database.DB.Model(&models.User{}).Where("role = ?", "customer")

	if strings.TrimSpace(query.Search) != "" {
		like := "%" + strings.TrimSpace(query.Search) + "%"
		db = db.Where("name ILIKE ? OR phone ILIKE ?", like, like)
	}
	if query.Status == "blocked" {
		db = db.Where("is_blocked = ?", true)
	} else if query.Status == "active" {
		db = db.Where("is_blocked = ?", false)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count customers"})
		return
	}

	switch query.Sort {
	case "oldest":
		db = db.Order("created_at ASC")
	default:
		db = db.Order("created_at DESC")
	}

	offset := (query.Page - 1) * query.Limit
	var users []models.User
	if err := db.Offset(offset).Limit(query.Limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch customers"})
		return
	}

	summaries := make([]models.CustomerSummary, 0, len(users))
	for _, u := range users {
		var totalOrders int64
		var totalSpent float64
		var lastOrderAt *string

		database.DB.Model(&models.Order{}).Where("user_id = ?", u.ID).Count(&totalOrders)
		database.DB.Model(&models.Order{}).Where("user_id = ? AND payment_status = ?", u.ID, "paid").
			Select("COALESCE(SUM(total_amount), 0)").Scan(&totalSpent)

		var lastOrder models.Order
		var lastOrderPtr *models.Order
		if err := database.DB.Where("user_id = ?", u.ID).Order("created_at DESC").First(&lastOrder).Error; err == nil {
			lastOrderPtr = &lastOrder
		}
		_ = lastOrderAt

		summary := models.CustomerSummary{
			ID:          u.ID,
			Name:        u.Name,
			Phone:       u.Phone,
			IsBlocked:   u.IsBlocked,
			CreatedAt:   u.CreatedAt,
			TotalOrders: totalOrders,
			TotalSpent:  totalSpent,
		}
		if lastOrderPtr != nil {
			summary.LastOrderAt = &lastOrderPtr.CreatedAt
		}
		summaries = append(summaries, summary)
	}

	c.JSON(http.StatusOK, models.CustomerListResponse{
		Customers:  summaries,
		Page:       query.Page,
		Limit:      query.Limit,
		Total:      total,
		TotalPages: int((total + int64(query.Limit) - 1) / int64(query.Limit)),
	})
}

// GetCustomerByID godoc
// GET /api/v1/admin/customers/:id (admin only)
func GetCustomerByID(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := database.DB.Where("role = ?", "customer").First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	var orders []models.Order
	database.DB.Preload("Items.Product").Preload("Address").Where("user_id = ?", user.ID).Order("created_at DESC").Find(&orders)

	var addresses []models.Address
	database.DB.Where("user_id = ?", user.ID).Find(&addresses)

	var wallet models.Wallet
	var walletPtr *models.Wallet
	if err := database.DB.Where("user_id = ?", user.ID).First(&wallet).Error; err == nil {
		walletPtr = &wallet
	}

	var transactions []models.WalletTransaction
	if walletPtr != nil {
		database.DB.Where("wallet_id = ?", walletPtr.ID).Order("created_at DESC").Limit(50).Find(&transactions)
	}

	var totalOrders int64 = int64(len(orders))
	var totalSpent float64
	database.DB.Model(&models.Order{}).Where("user_id = ? AND payment_status = ?", user.ID, "paid").
		Select("COALESCE(SUM(total_amount), 0)").Scan(&totalSpent)

	c.JSON(http.StatusOK, models.CustomerDetail{
		ID:           user.ID,
		Name:         user.Name,
		Phone:        user.Phone,
		IsBlocked:    user.IsBlocked,
		CreatedAt:    user.CreatedAt,
		TotalOrders:  totalOrders,
		TotalSpent:   totalSpent,
		Orders:       orders,
		Addresses:    addresses,
		Wallet:       walletPtr,
		Transactions: transactions,
	})
}

// BlockCustomer godoc
// PUT /api/v1/admin/customers/:id/block (admin only)
func BlockCustomer(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := database.DB.Where("role = ?", "customer").First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}
	user.IsBlocked = true
	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to block customer"})
		return
	}
	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "block_customer", "customer", id, "blocked")
	c.JSON(http.StatusOK, gin.H{"success": true, "is_blocked": true})
}

// UnblockCustomer godoc
// PUT /api/v1/admin/customers/:id/unblock (admin only)
func UnblockCustomer(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := database.DB.Where("role = ?", "customer").First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}
	user.IsBlocked = false
	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unblock customer"})
		return
	}
	adminID := c.MustGet("user_id").(uint)
	adminPhone := ctxPhone(c)
	utils.LogAudit(adminID, adminPhone, "unblock_customer", "customer", id, "unblocked")
	c.JSON(http.StatusOK, gin.H{"success": true, "is_blocked": false})
}
