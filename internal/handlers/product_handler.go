package handlers

import (
	"fmt"
	"math"
	"net/http"
	"time"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/cache"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetProducts godoc
// GET /api/v1/products
// Supports: ?search=&category_id=&min_price=&max_price=&in_stock=&sort=&page=&limit=
func GetProducts(c *gin.Context) {
	var query models.ProductListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
        if query.Search == "" && query.Q != "" {
                query.Search = query.Q
        }

	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}

	inStockStr := "any"
	if query.InStock != nil {
		inStockStr = fmt.Sprintf("%v", *query.InStock)
	}
	latStr, lngStr := "none", "none"
    if query.Lat != nil {
        latStr = fmt.Sprintf("%.4f", *query.Lat)
    }
    if query.Lng != nil {
        lngStr = fmt.Sprintf("%.4f", *query.Lng)
    }
    cacheKey := fmt.Sprintf("products:list:page=%d:limit=%d:search=%s:cat=%d:min=%.2f:max=%.2f:instock=%s:sort=%s:lat=%s:lng=%s",
        query.Page, query.Limit, query.Search, query.CategoryID, query.MinPrice, query.MaxPrice, inStockStr, query.Sort, latStr, lngStr)

	var cachedResponse models.ProductListResponse
	if found, _ := cache.Get(c.Request.Context(), cacheKey, &cachedResponse); found {
		c.JSON(http.StatusOK, cachedResponse)
		return
	}

	db := database.DB.Model(&models.Product{}).Preload("Category").Preload("Subcategory").Preload("Inventories")

	// Search by name or description
	if strings.TrimSpace(query.Search) != "" {
		like := "%" + strings.TrimSpace(query.Search) + "%"
		db = db.Where("name ILIKE ? OR description ILIKE ?", like, like)
	}

	// Filter by category
	if query.CategoryID > 0 {
		db = db.Where("category_id = ?", query.CategoryID)
	}

	// Filter by price range
	if query.MinPrice > 0 {
		db = db.Where("price >= ?", query.MinPrice)
	}
	if query.MaxPrice > 0 {
		db = db.Where("price <= ?", query.MaxPrice)
	}

// Filter by stock availability (correlated subquery avoids duplicate rows from a JOIN)
	if query.InStock != nil {
db = db.Where("EXISTS (SELECT 1 FROM inventories WHERE inventories.product_id = products.id AND inventories.in_stock = ?)", *query.InStock)

	}

	// Count total before pagination
	var total int64
	if err := db.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
		return
	}

	// Sorting
	switch query.Sort {
	case "price_asc":
		db = db.Order("price ASC")
	case "price_desc":
		db = db.Order("price DESC")
	case "name_asc":
		db = db.Order("name ASC")
	case "name_desc":
		db = db.Order("name DESC")
	case "newest":
		db = db.Order("created_at DESC")
	default:
		db = db.Order("created_at DESC")
	}

	// Pagination
	offset := (query.Page - 1) * query.Limit
	var products []models.Product
	if err := db.Offset(offset).Limit(query.Limit).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(query.Limit)))

    // If the caller supplied their location, resolve stock against the
    // warehouse that would actually serve them (same logic checkout uses),
    // rather than "in stock somewhere" which is misleading when the
    // nearest warehouse is empty but a distant one still has stock.
    if query.Lat != nil && query.Lng != nil {
        nearestWarehouse, _, err := FindNearestWarehouse(*query.Lat, *query.Lng)
        if err == nil && nearestWarehouse != nil {
            for i := range products {
                stock := 0
                inStock := false
                for _, inv := range products[i].Inventories {
                    if inv.WarehouseID == nearestWarehouse.ID {
                        stock = inv.Stock
                        inStock = inv.InStock
                        break
                    }
                }
                products[i].NearestStock = &stock
                products[i].NearestInStock = &inStock
            }
        }
    }

    response := models.ProductListResponse{
		Products:   products,
		Page:       query.Page,
		Limit:      query.Limit,
		Total:      total,
		TotalPages: totalPages,
	}

	_ = cache.Set(c.Request.Context(), cacheKey, response, 3*time.Minute)
	c.JSON(http.StatusOK, response)
}

// GetProductByID godoc
// GET /api/v1/products/:id
func GetProductByID(c *gin.Context) {
	id := c.Param("id")
	cacheKey := "products:id:" + id

	var product models.Product
	if found, _ := cache.Get(c.Request.Context(), cacheKey, &product); found {
		c.JSON(http.StatusOK, product)
		return
	}

	if err := database.DB.Preload("Category").Preload("Subcategory").Preload("Inventories").First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	_ = cache.Set(c.Request.Context(), cacheKey, product, 10*time.Minute)
	c.JSON(http.StatusOK, product)
}

// GetCategories godoc
// GET /api/v1/categories
func GetCategories(c *gin.Context) {
	cacheKey := "categories:all"
	var categories []models.Category

	if found, _ := cache.Get(c.Request.Context(), cacheKey, &categories); found {
		c.JSON(http.StatusOK, gin.H{"categories": categories})
		return
	}

	if err := database.DB.Order("name ASC").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	_ = cache.Set(c.Request.Context(), cacheKey, categories, 30*time.Minute)
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}
