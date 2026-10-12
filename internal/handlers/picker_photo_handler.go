package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

var pickerPhotoOnce sync.Once

func ensurePickerPhotoTable() {
	pickerPhotoOnce.Do(func() {
		_ = database.DB.AutoMigrate(&models.PickerPhoto{})
	})
}

func pickerPhotoURL(staffID uint) string {
	ensurePickerPhotoTable()
	var p models.PickerPhoto
	database.DB.Where("staff_id = ?", staffID).Limit(1).Find(&p)
	return p.URL
}

// UploadPickerPhoto POST /warehouse/picker/profile-photo (multipart field "image")
func UploadPickerPhoto(c *gin.Context) {
	ensurePickerPhotoTable()
	staffID, ok := pickerStaffID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image provided or file is larger than 5 MB"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type. Allowed: jpg, jpeg, png, webp"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not read the image"})
		return
	}
	buf := make([]byte, 512)
	n, _ := src.Read(buf)
	src.Close()
	ct := http.DetectContentType(buf[:n])
	if ct != "image/jpeg" && ct != "image/png" && ct != "image/webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This file is not a valid image"})
		return
	}

	cfg := config.AppConfig
	url := ""
	if cfg != nil && cfg.CloudinaryCloudName != "" && cfg.CloudinaryAPIKey != "" && cfg.CloudinaryAPISecret != "" {
		url, err = uploadToCloudinary(file, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
			return
		}
	} else {
		filename := fmt.Sprintf("picker_%d_%d%s", staffID, time.Now().UnixNano(), ext)
		if err := c.SaveUploadedFile(file, filepath.Join("uploads", filename)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
			return
		}
		base := ""
		if cfg != nil {
			base = cfg.PublicBaseURL
		}
		url = base + "/uploads/" + filename
	}

	if err := database.DB.Save(&models.PickerPhoto{StaffID: staffID, URL: url}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save profile photo"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image_url": url})
}
