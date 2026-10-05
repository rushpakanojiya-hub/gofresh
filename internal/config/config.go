package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                    string
	GinMode                 string
	DBHost                  string
	DBPort                  string
	DBUser                  string
	DBPassword              string
	DBName                  string
	DBSSLMode               string
	FirebaseCredentialsPath string
	JWTSecret               string
	JWTExpiryHours          string
	RazorpayKeyID           string
	RazorpayKeySecret       string
	AllowedOrigins          string
	CloudinaryCloudName     string
	CloudinaryAPIKey        string
	CloudinaryAPISecret     string
	RedisURL                string

	// PublicBaseURL is this server's own public https origin, used to
	// build absolute URLs for files saved to local disk (the Cloudinary
	// fallback path) so links work correctly from other domains like the
	// admin panel (which is hosted separately on CloudFront/S3).
	PublicBaseURL string

	// Seller/invoice details. All blank by default - never invented. Set
	// these env vars to the real registered business details before
	// invoices need to be GST-compliant; until then the PDF/API clearly
	// state "Not configured" for anything unset rather than showing a
	// fabricated placeholder.
	SellerCompanyName   string
	SellerAddress       string
	SellerGSTIN         string
	SellerContactNumber string
	SellerEmail         string
	SellerState         string
	SellerStateCode     string
	SellerFSSAINumber   string

	// DefaultMaxActiveOrdersPerPartner is the fallback capacity (max
	// concurrent confirmed/shipped orders) used for a delivery partner
	// when one isn't explicitly set on that partner record.
	DefaultMaxActiveOrdersPerPartner int

	// DeliveryAssignmentTimeoutMinutes is how long a delivery partner has
	// to accept or reject a new assignment before it automatically
	// expires and is offered to the next eligible partner.
	DeliveryAssignmentTimeoutMinutes int

	// DeliveryOTPExpiryMinutes is how long a delivery-completion OTP stays
	// valid after being generated (when the order moves to
	// OUT_FOR_DELIVERY). Once expired, the partner must not be able to
	// complete delivery with it - see services.UpdateDeliveryStatus.
	DeliveryOTPExpiryMinutes int

	// DeliveryOTPMaxAttempts caps how many wrong OTP guesses a partner can
	// make for a single delivery before the OTP is locked and a fresh one
	// is required (re-issued by re-entering OUT_FOR_DELIVERY is not
	// possible, so an admin/support re-trigger is needed - see
	// services.UpdateDeliveryStatus).
	DeliveryOTPMaxAttempts int

	// CustomerOTPMaxAttempts caps how many wrong login-OTP guesses a phone
	// number can make against a single OTP before it is locked out and a
	// fresh OTP must be requested via /auth/send-otp.
	CustomerOTPMaxAttempts int
	// DeliveryGeofenceRadiusMeters is how close (in meters) the delivery
	// partner's last known GPS location must be to the customer's saved
	// delivery address before a DELIVERED transition is allowed.
	DeliveryGeofenceRadiusMeters float64
	// WarehouseHandoverGeofenceRadiusMeters is how close (in meters) the
	// delivery partner's last known GPS location must be to the warehouse
	// before warehouse staff can confirm an order handover to them.
	WarehouseHandoverGeofenceRadiusMeters float64
}

var AppConfig *Config

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	cfg := &Config{
		Port:                                  getEnv("PORT", "8080"),
		GinMode:                               getEnv("GIN_MODE", "debug"),
		DBHost:                                getEnv("DB_HOST", "localhost"),
		DBPort:                                getEnv("DB_PORT", "5432"),
		DBUser:                                getEnv("DB_USER", "postgres"),
		DBPassword:                            getEnv("DB_PASSWORD", "postgres"),
		DBName:                                getEnv("DB_NAME", "ecommerce_db"),
		DBSSLMode:                             getEnv("DB_SSLMODE", "disable"),
		FirebaseCredentialsPath:               getEnv("FIREBASE_CREDENTIALS_PATH", "secrets/firebase-service-account.json"),
		JWTSecret:                             getEnv("JWT_SECRET", "default_secret_change_me"),
		JWTExpiryHours:                        getEnv("JWT_EXPIRY_HOURS", "72"),
		RazorpayKeyID:                         getEnv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:                     getEnv("RAZORPAY_KEY_SECRET", ""),
		AllowedOrigins:                        getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:19006"),
		CloudinaryCloudName:                   getEnv("CLOUDINARY_CLOUD_NAME", ""),
		CloudinaryAPIKey:                      getEnv("CLOUDINARY_API_KEY", ""),
		CloudinaryAPISecret:                   getEnv("CLOUDINARY_API_SECRET", ""),
		RedisURL:                              getEnv("REDIS_URL", ""),
		PublicBaseURL:                         getEnv("PUBLIC_BASE_URL", "https://32-196-3-31.sslip.io"),
		SellerCompanyName:                     getEnv("SELLER_COMPANY_NAME", ""),
		SellerAddress:                         getEnv("SELLER_ADDRESS", ""),
		SellerGSTIN:                           getEnv("SELLER_GSTIN", ""),
		SellerContactNumber:                   getEnv("SELLER_CONTACT_NUMBER", ""),
		SellerEmail:                           getEnv("SELLER_EMAIL", ""),
		SellerState:                           getEnv("SELLER_STATE", ""),
		SellerStateCode:                       getEnv("SELLER_STATE_CODE", ""),
		SellerFSSAINumber:                     getEnv("SELLER_FSSAI_NUMBER", ""),
		DefaultMaxActiveOrdersPerPartner:      getEnvInt("MAX_ACTIVE_ORDERS_PER_PARTNER", 1),
		DeliveryAssignmentTimeoutMinutes:      getEnvInt("DELIVERY_ASSIGNMENT_TIMEOUT_MINUTES", 5),
		DeliveryOTPExpiryMinutes:              getEnvInt("DELIVERY_OTP_EXPIRY_MINUTES", 15),
		DeliveryOTPMaxAttempts:                getEnvInt("DELIVERY_OTP_MAX_ATTEMPTS", 5),
		CustomerOTPMaxAttempts:                getEnvInt("CUSTOMER_OTP_MAX_ATTEMPTS", 5),
		DeliveryGeofenceRadiusMeters:          getEnvFloat("DELIVERY_GEOFENCE_RADIUS_METERS", 350),
		WarehouseHandoverGeofenceRadiusMeters: getEnvFloat("WAREHOUSE_HANDOVER_GEOFENCE_RADIUS_METERS", 200),
	}

	if cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "" {
		log.Println("CLOUDINARY_* not set - uploaded images will be saved to local disk, which does NOT survive redeploys on most hosts (e.g. Render free tier). Set CLOUDINARY_CLOUD_NAME / CLOUDINARY_API_KEY / CLOUDINARY_API_SECRET for persistent image storage.")
	}

	if cfg.RazorpayKeyID == "" || cfg.RazorpayKeySecret == "" {
		log.Println("RAZORPAY_KEY_ID / RAZORPAY_KEY_SECRET not set - online payment endpoints will return an error until configured. COD checkout is unaffected.")
	}

	if cfg.GinMode == "release" && cfg.JWTSecret == "default_secret_change_me" {
		log.Fatal("JWT_SECRET must be set to a strong random value before running with GIN_MODE=release")
	}

	if cfg.GinMode == "release" && isLocalOnlyOrigins(cfg.AllowedOrigins) {
		log.Println("WARNING: GIN_MODE=release but ALLOWED_ORIGINS still only contains localhost entries. " +
			"Set ALLOWED_ORIGINS to your real frontend domain(s), e.g. https://myshop.com")
	}

	AppConfig = cfg
	return cfg
}

func isLocalOnlyOrigins(origins string) bool {
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if !strings.Contains(o, "localhost") && !strings.Contains(o, "127.0.0.1") {
			return false
		}
	}
	return true
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvInt reads an integer env var, falling back (and logging a warning)
// if it's unset or not a valid positive integer.
func getEnvInt(key string, fallback int) int {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Printf("invalid %s=%q, falling back to %d", key, value, fallback)
		return fallback
	}
	return parsed
}

// getEnvFloat reads a float env var, falling back (and logging a warning)
// if it's unset or not a valid positive number.
func getEnvFloat(key string, fallback float64) float64 {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		log.Printf("invalid %s=%q, falling back to %v", key, value, fallback)
		return fallback
	}
	return parsed
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}
