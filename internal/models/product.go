package models

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"not null;index" json:"name"`
	Description string     `json:"description"`
	Price       float64    `gorm:"not null;index" json:"price"`
// MRP is the printed maximum retail price shown struck-through next to
// Price on the customer app (Blinkit-style). 0 means "no MRP set" -
// callers should treat 0 as "same as Price, no discount to show" rather
// than a real MRP of zero.
MRP         float64    `gorm:"not null;default:0" json:"mrp"`
// CostPrice is what we paid to acquire the product (per unit). Used for
// COGS / gross profit calculations in the Finance panel. 0 means not yet
// set - callers should treat 0 as "unknown cost", not "free product".
CostPrice   float64    `gorm:"not null;default:0" json:"cost_price"`
	GSTPercent  float64    `gorm:"not null;default:0" json:"gst_percent"`
	HSNCode     string     `json:"hsn_code,omitempty"`
	ImageURL    string     `json:"image_url"`
Barcode     string     `gorm:"index" json:"barcode,omitempty"`
	CategoryID  uint       `gorm:"index" json:"category_id"`
        SubcategoryID *uint       `gorm:"index" json:"subcategory_id,omitempty"`
	Category    Category   `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
        Subcategory Subcategory `gorm:"foreignKey:SubcategoryID" json:"subcategory,omitempty"`
	Inventories []Inventory `gorm:"foreignKey:ProductID" json:"inventories,omitempty"`
	NearestStock *int `gorm:"-" json:"nearest_stock,omitempty"`
	NearestInStock *bool `gorm:"-" json:"nearest_in_stock,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ProductRequest is the body for POST/PUT /admin/products (admin only).
// Stock is only used on create, to seed the product's Inventory row ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â‚¬Å¾Ã‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â¦ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â
// use PUT /admin/products/:id/inventory to adjust stock afterwards.
type ProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required,gt=0"`
MRP         float64 `json:"mrp" binding:"gte=0"`
	GSTPercent  float64 `json:"gst_percent" binding:"gte=0,lte=100"`
CostPrice   float64 `json:"cost_price" binding:"gte=0"`
	HSNCode     string  `json:"hsn_code"`
	ImageURL    string  `json:"image_url"`
	CategoryID  uint    `json:"category_id" binding:"required"`
        SubcategoryID *uint  `json:"subcategory_id"`
	Stock       int     `json:"stock" binding:"gte=0"`
}

// ProductListQuery binds query params for GET /products (filter, sort, search, paginate).
type ProductListQuery struct {
	Search     string  `form:"search"`
    Q          string  `form:"q"` // alias for Search, used by the mobile app's /products/search endpoint
	CategoryID uint    `form:"category_id"`
	MinPrice   float64 `form:"min_price"`
	MaxPrice   float64 `form:"max_price"`
	InStock    *bool   `form:"in_stock"`
	Sort       string  `form:"sort"` // price_asc, price_desc, name_asc, name_desc, newest
	Page       int     `form:"page,default=1"`
	Limit      int     `form:"limit,default=20"`
    Lat        *float64 `form:"lat"`
    Lng        *float64 `form:"lng"`
}

// ProductListResponse wraps paginated product results.
type ProductListResponse struct {
	Products   []Product `json:"products"`
	Page       int       `json:"page"`
	Limit      int       `json:"limit"`
	Total      int64     `json:"total"`
	TotalPages int       `json:"total_pages"`
}


