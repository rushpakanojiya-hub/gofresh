package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/cache"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

// CreateCategory godoc
// POST /api/v1/admin/categories (admin only)
func CreateCategory(c *gin.Context) {
	var req models.CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category := models.Category{Name: req.Name, ImageURL: req.ImageURL}
	if err := database.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Category already exists or could not be created"})
		return
	}
	_ = cache.Delete(c.Request.Context(), "categories:all")
	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "create_category", "category", fmt.Sprint(category.ID), "name: "+category.Name)

	c.JSON(http.StatusCreated, category)
}

// UpdateCategory godoc
// PUT /api/v1/admin/categories/:id (admin only)
func UpdateCategory(c *gin.Context) {
	id := c.Param("id")

	var category models.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	var req models.CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category.Name = req.Name
	category.ImageURL = req.ImageURL
	if err := database.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update category"})
		return
	}
	_ = cache.Delete(c.Request.Context(), "categories:all")

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "update_category", "category", id, "name: "+category.Name)

	c.JSON(http.StatusOK, category)
}

// DeleteCategory godoc
// DELETE /api/v1/admin/categories/:id (admin only)
// Blocked if any product still references this category, so products never
// end up pointing at a category that no longer exists.
func DeleteCategory(c *gin.Context) {
	id := c.Param("id")

	var category models.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	var productCount int64
	database.DB.Model(&models.Product{}).Where("category_id = ?", category.ID).Count(&productCount)
	if productCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete category with existing products. Reassign or delete those products first."})
		return
	}

	if err := database.DB.Delete(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
		return
	}
	_ = cache.Delete(c.Request.Context(), "categories:all")

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "delete_category", "category", id, "name: "+category.Name)

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted"})
}

// ---------------------------------------------------------------------------
// Products
// ---------------------------------------------------------------------------

// CreateProduct godoc
// POST /api/v1/admin/products (admin only)
// Creates the product and its Inventory row (seeded with req.Stock) together.
func CreateProduct(c *gin.Context) {
	var req models.ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var category models.Category
	if err := database.DB.First(&category, req.CategoryID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category not found"})
		return
	}

	product := models.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		MRP:         req.MRP,
		CostPrice:   req.CostPrice,
               GSTPercent:  req.GSTPercent,
		HSNCode:     req.HSNCode,
		ImageURL:    req.ImageURL,
		CategoryID:  req.CategoryID,
		SubcategoryID: req.SubcategoryID,
	}
    txErr := database.DB.Transaction(func(tx *gorm.DB) error {
            if err := tx.Create(&product).Error; err != nil {
                    return err
            }
            var warehouses []models.Warehouse
            if err := tx.Where("is_active = ?", true).Find(&warehouses).Error; err != nil {
                    return err
            }
            if len(warehouses) == 0 {
                    // No warehouses configured yet - fall back to the old
                    // single-warehouse-1 behavior so product creation still works.
                    inventory := models.Inventory{
                            ProductID:   product.ID,
                            WarehouseID: 1,
                            Stock:       req.Stock,
                            InStock:     req.Stock > 0,
                    }
                    return tx.Create(&inventory).Error
            }
            // Every product gets an inventory row at every active warehouse
            // from day one. Only the first (primary) warehouse gets the
            // stock figure entered on creation; others start at 0 and are
            // stocked separately by the admin later.
            for i, wh := range warehouses {
                    stock := 0
                    if i == 0 {
                            stock = req.Stock
                    }
                    inventory := models.Inventory{
                            ProductID:   product.ID,
                            WarehouseID: wh.ID,
                            Stock:       stock,
                            InStock:     stock > 0,
                    }
                    if err := tx.Create(&inventory).Error; err != nil {
                            return err
                    }
            }
            return nil
    })

	if txErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}
	_ = cache.DeleteByPrefix(c.Request.Context(), "products:list:")

	database.DB.Preload("Category").Preload("Inventories").First(&product, product.ID)

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "create_product", "product", fmt.Sprint(product.ID), "name: "+product.Name)
	c.JSON(http.StatusCreated, product)
}

// UpdateProduct godoc
// PUT /api/v1/admin/products/:id (admin only)
// Updates product fields only
func UpdateProduct(c *gin.Context) {
	id := c.Param("id")

	var product models.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var req models.ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var category models.Category
	if err := database.DB.First(&category, req.CategoryID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category not found"})
		return
	}

	product.Name = req.Name
	product.Description = req.Description
	product.Price = req.Price
	product.MRP = req.MRP
	product.CostPrice = req.CostPrice
       product.GSTPercent = req.GSTPercent
	product.HSNCode = req.HSNCode
	product.ImageURL = req.ImageURL
	product.CategoryID = req.CategoryID
	product.SubcategoryID = req.SubcategoryID

	if err := database.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}
	_ = cache.DeleteByPrefix(c.Request.Context(), "products:list:")
	_ = cache.Delete(c.Request.Context(), "products:id:"+id)

	database.DB.Preload("Category").Preload("Inventories").First(&product, product.ID)

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "update_product", "product", id, "name: "+product.Name)
	c.JSON(http.StatusOK, product)
}

// GenerateProductBarcode godoc
// POST /api/v1/admin/products/:id/barcode (admin only)
// Assigns a deterministic, guaranteed-unique barcode to a product that
// doesn't have one yet (format PRD######, built from the product's own
// primary key so there's no collision-retry needed). Idempotent - calling
// it again on a product that already has a barcode just returns the
// existing value unchanged, so it's safe to click more than once.
func GenerateProductBarcode(c *gin.Context) {
id := c.Param("id")

var product models.Product
if err := database.DB.First(&product, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
return
}

if product.Barcode == "" {
product.Barcode = fmt.Sprintf("PRD%06d", product.ID)
if err := database.DB.Model(&product).Update("barcode", product.Barcode).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate barcode"})
return
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "generate_barcode", "product", id, "barcode: "+product.Barcode)
}

c.JSON(http.StatusOK, gin.H{"id": product.ID, "barcode": product.Barcode})
}

// DeleteProduct godoc
// DELETE /api/v1/admin/products/:id (admin only)
func DeleteProduct(c *gin.Context) {
	id := c.Param("id")

	var product models.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", product.ID).Delete(&models.Inventory{}).Error; err != nil {
			return err
		}
		return tx.Delete(&product).Error
	})

	if txErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}
	_ = cache.DeleteByPrefix(c.Request.Context(), "products:list:")
	_ = cache.Delete(c.Request.Context(), "products:id:"+id)

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "delete_product", "product", id, "name: "+product.Name)

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted"})
}

// UpdateInventory godoc
// PUT /api/v1/admin/products/:id/inventory (admin only)
// Sets stock to an absolute value and recomputes in_stock from it.
func UpdateInventory(c *gin.Context) {
	id := c.Param("id")

	var product models.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var req models.InventoryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var warehouse models.Warehouse
	if err := database.DB.First(&warehouse, req.WarehouseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Warehouse not found"})
		return
	}

	var inventory models.Inventory
	if err := database.DB.Where("product_id = ? AND warehouse_id = ?", product.ID, req.WarehouseID).First(&inventory).Error; err != nil {
		inventory = models.Inventory{ProductID: product.ID, WarehouseID: req.WarehouseID}
	}

	inventory.Stock = req.Stock
	inventory.InStock = req.Stock > 0

	if err := database.DB.Save(&inventory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update inventory"})
		return
	}
	if err := cache.DeleteByPrefix(c.Request.Context(), "products:list:"); err != nil { }
	_ = cache.Delete(c.Request.Context(), "products:id:"+id)

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "update_inventory", "product", id, fmt.Sprintf("stock: %d", inventory.Stock))

	c.JSON(http.StatusOK, inventory)
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

// validOrderTransitions defines which status changes an admin may make.
// pending and confirmed orders can also be cancelled (stock is restored).
var validOrderTransitions = map[string]map[string]bool{
	models.OrderStatusPending:   {models.OrderStatusConfirmed: true, models.OrderStatusCancelled: true},
	models.OrderStatusConfirmed: {models.OrderStatusShipped: true, models.OrderStatusCancelled: true},
	models.OrderStatusShipped:   {models.OrderStatusDelivered: true},
}

// GetAllOrders godoc
// GET /api/v1/admin/orders (admin only)
func GetAllOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	db := database.DB.Model(&models.Order{})
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}

	var total int64
	db.Count(&total)

	var orders []models.Order
	if err := db.
		Preload("Items.Product").
		Preload("Address").
		Preload("DeliveryPartner").
		Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load orders"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, models.OrderListResponse{
		Orders:     orders,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// UpdateOrderStatus godoc
// PUT /api/v1/admin/orders/:id/status (admin only)
// Enforces the pending -> confirmed -> shipped -> delivered flow (plus
// cancellation from pending/confirmed) and restores stock on cancellation,
// same as the customer-facing CancelOrder.
func UpdateOrderStatus(c *gin.Context) {
	orderID := c.Param("id")

	var order models.Order
	if err := database.DB.Preload("Items.Product").Preload("Address").First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	var req models.OrderStatusUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Status == models.OrderStatusDelivered && order.PaymentMethod == models.PaymentMethodCOD {
        c.JSON(http.StatusBadRequest, gin.H{"error": "COD orders must be marked delivered by the delivery partner (cash collection and ledger are recorded there)"})
        return
    }
    allowed := validOrderTransitions[order.Status]
	if !allowed[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot change status from '" + order.Status + "' to '" + req.Status + "'",
		})
		return
	}

	if (req.Status == models.OrderStatusConfirmed || req.Status == models.OrderStatusShipped || req.Status == models.OrderStatusDelivered) &&
		order.PaymentMethod == models.PaymentMethodOnline &&
		order.PaymentStatus != models.OrderPaymentStatusPaid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot move an online-payment order to '" + req.Status + "' until payment is verified (payment_status is currently '" + order.PaymentStatus + "')",
		})
		return
	}

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if req.Status == models.OrderStatusCancelled {
			for _, item := range order.Items {
				var inventory models.Inventory
				if err := tx.Where("product_id = ?", item.ProductID).Order("id").First(&inventory).Error; err == nil {
					inventory.Stock += item.Quantity
					inventory.InStock = true
					if err := tx.Save(&inventory).Error; err != nil {
						return err
					}
				}
			}
		}
		return tx.Model(&order).Update("status", req.Status).Error
	})

	if txErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	order.Status = req.Status
    // If this status change just confirmed the order (e.g. admin verifying
    // an online payment), try to auto-assign the nearest available
    // delivery partner right away instead of waiting for warehouse
    // packing to complete.
    if req.Status == models.OrderStatusConfirmed {
        go services.AutoAssignDeliveryPartner(order.ID)
    }

	message := "Your order #" + orderID + " status is now: " + req.Status
	utils.SendNotification(order.Address.Phone, message, "order_status_"+req.Status, &order.ID)
	services.SendPushToUser(order.UserID, "Order Update", message)

	adminID := c.MustGet("user_id").(uint)
	adminPhone := c.MustGet("phone").(string)
	utils.LogAudit(adminID, adminPhone, "update_order_status", "order", orderID, "new status: "+req.Status)

	c.JSON(http.StatusOK, order)
}


