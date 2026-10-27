package handlers

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// BUG-14: every default-address change runs in one transaction that first locks
// the user's row, so concurrent create/update/set-default calls for the same user
// are serialized and the user always ends with exactly one default address.
func lockUserForAddressWrite(tx *gorm.DB, userID uint) error {
    var u models.User
    return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&u, userID).Error
}

// CreateAddress godoc
// POST /api/v1/addresses (protected)
// The first address a user adds is automatically made the default one.
// BUG-69: deleted addresses are ignored by the count, update, delete and set-default paths.
func CreateAddress(c *gin.Context) {
    userID := c.MustGet("user_id").(uint)

    var req models.AddressRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    address := models.Address{
        UserID:   userID,
        Label:    req.Label,
        FullName: req.FullName,
        Phone:    req.Phone,
        Line1:    req.Line1,
        Line2:    req.Line2,
        City:     req.City,
        State:    req.State,
        Pincode:  req.Pincode,
        Lat:      req.Lat,
        Lng:      req.Lng,
    }
    err := database.DB.Transaction(func(tx *gorm.DB) error {
        if err := lockUserForAddressWrite(tx, userID); err != nil {
            return err
        }
        var existingCount int64
        if err := tx.Model(&models.Address{}).Where("user_id = ? AND is_deleted = false", userID).Count(&existingCount).Error; err != nil {
            return err
        }
        makeDefault := req.IsDefault || existingCount == 0
        if makeDefault {
            if err := tx.Model(&models.Address{}).Where("user_id = ? AND is_default = ?", userID, true).Update("is_default", false).Error; err != nil {
                return err
            }
        }
        address.IsDefault = makeDefault
        return tx.Create(&address).Error
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
        return
    }

    c.JSON(http.StatusCreated, address)
}

// UpdateAddress godoc
// PUT /api/v1/addresses/:id (protected)
func UpdateAddress(c *gin.Context) {
    userID := c.MustGet("user_id").(uint)
    addressID := c.Param("id")

    var address models.Address
    if err := database.DB.Where("is_deleted = false").First(&address, addressID).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
        return
    }
    if address.UserID != userID {
        c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this address"})
        return
    }

    var req models.AddressRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var updated models.Address
    err := database.DB.Transaction(func(tx *gorm.DB) error {
        if err := lockUserForAddressWrite(tx, userID); err != nil {
            return err
        }
        // Re-read under the lock: the default flag or deletion may have changed.
        if err := tx.Where("user_id = ? AND is_deleted = false", userID).First(&updated, addressID).Error; err != nil {
            return err
        }
        if req.IsDefault && !updated.IsDefault {
            if err := tx.Model(&models.Address{}).Where("user_id = ? AND is_default = ?", userID, true).Update("is_default", false).Error; err != nil {
                return err
            }
        }
        updated.Label = req.Label
        updated.FullName = req.FullName
        updated.Phone = req.Phone
        updated.Line1 = req.Line1
        updated.Line2 = req.Line2
        updated.City = req.City
        updated.State = req.State
        updated.Pincode = req.Pincode
        if req.Lat != nil {
            updated.Lat = req.Lat
        }
        if req.Lng != nil {
            updated.Lng = req.Lng
        }
        updated.IsDefault = req.IsDefault || updated.IsDefault
        return tx.Save(&updated).Error
    })
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
        return
    }

    c.JSON(http.StatusOK, updated)
}

// SetDefaultAddress godoc
// PUT /api/v1/addresses/:id/default (protected)
func SetDefaultAddress(c *gin.Context) {
    userID := c.MustGet("user_id").(uint)
    addressID := c.Param("id")

    var address models.Address
    if err := database.DB.Where("is_deleted = false").First(&address, addressID).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
        return
    }
    if address.UserID != userID {
        c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this address"})
        return
    }

    var updated models.Address
    err := database.DB.Transaction(func(tx *gorm.DB) error {
        if err := lockUserForAddressWrite(tx, userID); err != nil {
            return err
        }
        if err := tx.Where("user_id = ? AND is_deleted = false", userID).First(&updated, addressID).Error; err != nil {
            return err
        }
        if err := tx.Model(&models.Address{}).Where("user_id = ? AND is_default = ?", userID, true).Update("is_default", false).Error; err != nil {
            return err
        }
        if err := tx.Model(&updated).Update("is_default", true).Error; err != nil {
            return err
        }
        updated.IsDefault = true
        return nil
    })
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default address"})
        return
    }

    c.JSON(http.StatusOK, updated)
}