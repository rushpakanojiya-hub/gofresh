package handlers

import (
    "fmt"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/cache"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// ---------------------------------------------------------------------------
// Subcategories
// ---------------------------------------------------------------------------

// GetSubcategories godoc
// GET /api/v1/subcategories?category_id= (public)
// Returns all subcategories, or only those under a given category when
// category_id is provided (used by the admin "add product" form, which
// must only offer subcategories belonging to the selected category).
func GetSubcategories(c *gin.Context) {
    categoryID := c.Query("category_id")
    cacheKey := "subcategories:all"
    if categoryID != "" {
        cacheKey = "subcategories:cat:" + categoryID
    }

    var subcategories []models.Subcategory
    if found, _ := cache.Get(c.Request.Context(), cacheKey, &subcategories); found {
        c.JSON(http.StatusOK, gin.H{"subcategories": subcategories})
        return
    }

    query := database.DB.Order("name ASC")
    if categoryID != "" {
        query = query.Where("category_id = ?", categoryID)
    }
    if err := query.Find(&subcategories).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subcategories"})
        return
    }

    _ = cache.Set(c.Request.Context(), cacheKey, subcategories, 30*60*1000000000) // 30 minutes
    c.JSON(http.StatusOK, gin.H{"subcategories": subcategories})
}

// CreateSubcategory godoc
// POST /api/v1/admin/subcategories (admin only)
func CreateSubcategory(c *gin.Context) {
    var req models.SubcategoryRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var category models.Category
    if err := database.DB.First(&category, req.CategoryID).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Category not found"})
        return
    }

    subcategory := models.Subcategory{Name: req.Name, CategoryID: req.CategoryID, ImageURL: req.ImageURL}
    if err := database.DB.Create(&subcategory).Error; err != nil {
        c.JSON(http.StatusConflict, gin.H{"error": "Subcategory already exists or could not be created"})
        return
    }
    _ = cache.Delete(c.Request.Context(), "subcategories:all")
    _ = cache.Delete(c.Request.Context(), fmt.Sprintf("subcategories:cat:%d", req.CategoryID))

    adminID := c.MustGet("user_id").(uint)
    adminPhone := c.MustGet("phone").(string)
    utils.LogAudit(adminID, adminPhone, "create_subcategory", "subcategory", fmt.Sprint(subcategory.ID), "name: "+subcategory.Name)

    c.JSON(http.StatusCreated, subcategory)
}

// UpdateSubcategory godoc
// PUT /api/v1/admin/subcategories/:id (admin only)
func UpdateSubcategory(c *gin.Context) {
    id := c.Param("id")

    var subcategory models.Subcategory
    if err := database.DB.First(&subcategory, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Subcategory not found"})
        return
    }

    var req models.SubcategoryRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var category models.Category
    if err := database.DB.First(&category, req.CategoryID).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Category not found"})
        return
    }

    oldCategoryID := subcategory.CategoryID
    subcategory.Name = req.Name
    subcategory.CategoryID = req.CategoryID
    subcategory.ImageURL = req.ImageURL
    if err := database.DB.Save(&subcategory).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subcategory"})
        return
    }
    _ = cache.Delete(c.Request.Context(), "subcategories:all")
    _ = cache.Delete(c.Request.Context(), fmt.Sprintf("subcategories:cat:%d", oldCategoryID))
    _ = cache.Delete(c.Request.Context(), fmt.Sprintf("subcategories:cat:%d", req.CategoryID))

    adminID := c.MustGet("user_id").(uint)
    adminPhone := c.MustGet("phone").(string)
    utils.LogAudit(adminID, adminPhone, "update_subcategory", "subcategory", id, "name: "+subcategory.Name)

    c.JSON(http.StatusOK, subcategory)
}

// DeleteSubcategory godoc
// DELETE /api/v1/admin/subcategories/:id (admin only)
// Blocked if any product still references this subcategory.
func DeleteSubcategory(c *gin.Context) {
    id := c.Param("id")

    var subcategory models.Subcategory
    if err := database.DB.First(&subcategory, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Subcategory not found"})
        return
    }

    var productCount int64
    database.DB.Model(&models.Product{}).Where("subcategory_id = ?", subcategory.ID).Count(&productCount)
    if productCount > 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete subcategory with existing products. Reassign or delete those products first."})
        return
    }

    if err := database.DB.Delete(&subcategory).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subcategory"})
        return
    }
    _ = cache.Delete(c.Request.Context(), "subcategories:all")
    _ = cache.Delete(c.Request.Context(), fmt.Sprintf("subcategories:cat:%d", subcategory.CategoryID))

    adminID := c.MustGet("user_id").(uint)
    adminPhone := c.MustGet("phone").(string)
    utils.LogAudit(adminID, adminPhone, "delete_subcategory", "subcategory", id, "name: "+subcategory.Name)

    c.JSON(http.StatusOK, gin.H{"message": "Subcategory deleted"})
}