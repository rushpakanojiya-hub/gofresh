package handlers

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
)

// allowedImageExts restricts uploads to common image formats.
var allowedImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

const maxUploadSize = 5 << 20 // 5 MB

// UploadImage godoc
// POST /api/v1/upload (protected)
// Accepts a multipart/form-data "image" field and returns a URL that can be
// stored on a Product/Category's image_url field.
//
// If CLOUDINARY_CLOUD_NAME / CLOUDINARY_API_KEY / CLOUDINARY_API_SECRET are
// set, the file is uploaded to Cloudinary so it survives redeploys/restarts.
// Otherwise it falls back to local disk under ./uploads (fine for local dev,
// but most hosts including Render's free tier wipe this on every
// restart/redeploy - the returned URL will 404 later if you rely on it).
func UploadImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image file provided (expected form field 'image')"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type. Allowed: jpg, jpeg, png, webp"})
		return
	}

	cfg := config.AppConfig
	if cfg != nil && cfg.CloudinaryCloudName != "" && cfg.CloudinaryAPIKey != "" && cfg.CloudinaryAPISecret != "" {
		url, err := uploadToCloudinary(file, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image: " + err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"image_url": url})
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join("uploads", filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"image_url": cfg.PublicBaseURL + "/uploads/" + filename,
	})
}

// uploadToCloudinary sends the file to Cloudinary's signed upload endpoint
// using only the standard library (no SDK dependency) and returns the
// permanent https secure_url.
func uploadToCloudinary(fileHeader *multipart.FileHeader, cfg *config.Config) (string, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Cloudinary signature: sha1("<sorted params to sign>" + api_secret),
	// hex-encoded. We only send "timestamp", so signing that alone is enough.
	toSign := "timestamp=" + timestamp + cfg.CloudinaryAPISecret
	sum := sha1.Sum([]byte(toSign))
	signature := hex.EncodeToString(sum[:])

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, src); err != nil {
		return "", err
	}
	_ = writer.WriteField("api_key", cfg.CloudinaryAPIKey)
	_ = writer.WriteField("timestamp", timestamp)
	_ = writer.WriteField("signature", signature)
	if err := writer.Close(); err != nil {
		return "", err
	}

	uploadURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", cfg.CloudinaryCloudName)
	req, err := http.NewRequest(http.MethodPost, uploadURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		SecureURL string `json:"secure_url"`
		Error     struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK || result.SecureURL == "" {
		if result.Error.Message != "" {
			return "", fmt.Errorf("cloudinary: %s", result.Error.Message)
		}
		return "", fmt.Errorf("cloudinary upload failed with status %d", resp.StatusCode)
	}

	return optimizeCloudinaryURL(result.SecureURL), nil
}

// optimizeCloudinaryURL inserts a transformation segment right after
// "/upload/" so images are served resized (max width 600px), auto-compressed,
// and in the best format for the requesting device (webp/avif when
// supported). This makes product images load much faster on the app without
// any change needed on the frontend - it's baked into the stored URL.
func optimizeCloudinaryURL(url string) string {
	const marker = "/upload/"
	idx := strings.Index(url, marker)
	if idx == -1 {
		return url
	}
	insertAt := idx + len(marker)
	return url[:insertAt] + "w_600,q_auto,f_auto/" + url[insertAt:]
}
