package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
	"gorm.io/gorm"
)

// getOrCreateCart returns the user's cart, creating one if it doesn't exist yet
// (signup auto-creates a cart, but this keeps the API resilient for older users/data).
func getOrCreateCart(userID uint) (*models.Cart, error) {
	var cart models.Cart
	err := database.DB.Where("user_id = ?", userID).First(&cart).Error
	if err == nil {
		return &cart, nil
	}

	cart = models.Cart{UserID: userID}
	if err := database.DB.Create(&cart).Error; err != nil {
		if queryErr := database.DB.Where("user_id = ?", userID).First(&cart).Error; queryErr == nil {
			return &cart, nil
		}
		return nil, err
	}
	return &cart, nil
}

// resolveWarehouseForUser returns the nearest warehouse to the user's
// default saved address, or nil (no error) if the user has no default
// address yet - callers should fall back to the old combined-stock check
// in that case, since we can't reserve at a specific warehouse without
// knowing which one the user would actually be served from.
func resolveWarehouseForUser(userID uint) (*models.Warehouse, error) {
	var address models.Address
	err := database.DB.Where("user_id = ? AND is_default = ?", userID, true).First(&address).Error
	if err != nil || address.Lat == nil || address.Lng == nil {
		return nil, nil
	}
	warehouse, _, err := FindNearestWarehouse(*address.Lat, *address.Lng)
	if err != nil {
		return nil, err
	}
	return warehouse, nil
}

// buildCartResponse loads items (with product + inventory) and computes totals.
func buildCartResponse(cart *models.Cart) (models.CartResponse, error) {
	var items []models.CartItem
	if err := database.DB.
		Preload("Product").
		Preload("Product.Category").
		Preload("Product.Inventories").
		Where("cart_id = ?", cart.ID).
		Find(&items).Error; err != nil {
		return models.CartResponse{}, err
	}

	totalItems := 0
	totalAmount := 0.0
	for _, item := range items {
		totalItems += item.Quantity
		totalAmount += item.Product.Price * float64(item.Quantity)
	}

	return models.CartResponse{
		ID:          cart.ID,
		Items:       items,
		TotalItems:  totalItems,
		TotalAmount: totalAmount,
	}, nil
}

// GetCart godoc
// GET /api/v1/cart (protected)
func GetCart(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	cart, err := getOrCreateCart(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
		return
	}

	resp, err := buildCartResponse(cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart items"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// AddToCart godoc
// POST /api/v1/cart (protected)
// If the product is already in the cart, quantity is incremented instead of duplicated.
func AddToCart(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var req models.AddToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure product exists
	var product models.Product
	if err := database.DB.First(&product, req.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	cart, err := getOrCreateCart(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
		return
	}

	// Resolve the user's nearest warehouse (from their default address) so
	// the stock check/hold is warehouse-specific and accounts for other
	// shoppers' active 10-minute reservations. Users without a saved
	// default address yet fall back to the old combined-stock check,
	// since we don't know which warehouse would actually serve them.
	warehouse, err := resolveWarehouseForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check delivery serviceability"})
		return
	}

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		// Re-check for an existing cart item under the transaction so two
		// concurrent AddToCart requests for the same product cannot both
		// decide the item is new and both attempt an insert, creating
		// duplicate rows for the same (cart_id, product_id) pair.
		var existingItem models.CartItem
		hasExisting := tx.Where("cart_id = ? AND product_id = ?", cart.ID, req.ProductID).First(&existingItem).Error == nil
		newQuantity := req.Quantity
		if hasExisting {
			newQuantity = existingItem.Quantity + req.Quantity
		}

		if warehouse != nil {
			if err := services.ReserveStock(tx, userID, req.ProductID, warehouse.ID, newQuantity); err != nil {
				return err
			}
		} else {
			totalStock, err := database.GetTotalStock(req.ProductID)
			if err != nil {
				return err
			}
			if totalStock < newQuantity {
				return services.ErrInsufficientStock
			}
		}

		if hasExisting {
			existingItem.Quantity = newQuantity
			return tx.Save(&existingItem).Error
		}
		newItem := models.CartItem{
			CartID:    cart.ID,
			ProductID: req.ProductID,
			Quantity:  newQuantity,
		}
		return tx.Create(&newItem).Error
	})
	if txErr != nil {
		if txErr == services.ErrInsufficientStock {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient stock for this product"})
		} else if txErr == services.ErrNotAvailableAtWarehouse {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This product is not available in your area yet"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add item to cart"})
		}
		return
	}
	resp, err := buildCartResponse(cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// UpdateCartItem godoc
// PUT /api/v1/cart/:item_id (protected)
func UpdateCartItem(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	itemID := c.Param("item_id")

	var req models.UpdateCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var item models.CartItem
	if err := database.DB.First(&item, itemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart item not found"})
		return
	}

	// Ownership check Ã¢â‚¬â€ item's cart must belong to the requesting user
	var cart models.Cart
	if err := database.DB.First(&cart, item.CartID).Error; err != nil || cart.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this cart item"})
		return
	}

	// Resolve warehouse the same way AddToCart does, so the stock/hold
	// check accounts for other shoppers' active reservations.
	warehouse, err := resolveWarehouseForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check delivery serviceability"})
		return
	}

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if warehouse != nil {
			if err := services.ReserveStock(tx, userID, item.ProductID, warehouse.ID, req.Quantity); err != nil {
				return err
			}
		} else {
			totalStock, err := database.GetTotalStock(item.ProductID)
			if err != nil {
				return err
			}
			if totalStock < req.Quantity {
				return services.ErrInsufficientStock
			}
		}
		item.Quantity = req.Quantity
		return tx.Save(&item).Error
	})
	if txErr != nil {
		if txErr == services.ErrInsufficientStock {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient stock for this product"})
		} else if txErr == services.ErrNotAvailableAtWarehouse {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This product is not available in your area yet"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart item"})
		}
		return
	}

	resp, err := buildCartResponse(&cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// RemoveFromCart godoc
// DELETE /api/v1/cart/:item_id (protected)
func RemoveFromCart(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	itemID := c.Param("item_id")

	var item models.CartItem
	if err := database.DB.First(&item, itemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart item not found"})
		return
	}

	var cart models.Cart
	if err := database.DB.First(&cart, item.CartID).Error; err != nil || cart.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this cart item"})
		return
	}

	if err := database.DB.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove cart item"})
		return
	}

	// Release any hold on this product too, regardless of which warehouse
	// it was reserved at - the item is gone from the cart, so nothing
	// should keep the stock tied up until the TTL runs out on its own.
	database.DB.Where("user_id = ? AND product_id = ?", userID, item.ProductID).
		Delete(&models.CartReservation{})

	resp, err := buildCartResponse(&cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
