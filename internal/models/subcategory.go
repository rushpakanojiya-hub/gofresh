package models

import (
    "time"

    "gorm.io/gorm"
)

// Subcategory belongs to a Category and groups products within it
// (e.g. Category "Vegetables & Fruits" -> Subcategories "Fresh Vegetables",
// "Fresh Fruits", "Exotic Fruits").
type Subcategory struct {
    ID         uint           `gorm:"primaryKey" json:"id"`
    Name       string         `gorm:"not null;index" json:"name"`
    CategoryID uint           `gorm:"not null;index" json:"category_id"`
    Category   Category       `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
    ImageURL   string         `json:"image_url"`
    CreatedAt  time.Time      `json:"created_at"`
    UpdatedAt  time.Time      `json:"updated_at"`
    DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// SubcategoryRequest is the body for POST/PUT /admin/subcategories (admin only).
type SubcategoryRequest struct {
    Name       string `json:"name" binding:"required"`
    CategoryID uint   `json:"category_id" binding:"required"`
    ImageURL   string `json:"image_url"`
}