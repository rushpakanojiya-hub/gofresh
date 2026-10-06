package models

import (
	"time"

	"gorm.io/gorm"
)

// DeliveryPartner is a delivery person the admin can create/manage.
// The partner logs in via phone + OTP (same flow as customers) and can
// then push their live location while out for delivery.
type DeliveryPartner struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	Name                 string     `gorm:"not null" json:"name"`
	Phone                string     `gorm:"not null;uniqueIndex" json:"phone"`
	VehicleNumber        string     `json:"vehicle_number"`
	IsActive             bool       `json:"is_active"`
	IsOnline             bool       `gorm:"default:false" json:"is_online"`
	CurrentLat           *float64   `json:"current_lat,omitempty"`
	CurrentLng           *float64   `json:"current_lng,omitempty"`
	LastLocationUpdate   *time.Time `json:"last_location_update,omitempty"`
	CheckedInWarehouseID *uint      `gorm:"index" json:"checked_in_warehouse_id,omitempty"`
	CheckedInAt          *time.Time `json:"checked_in_at,omitempty"`
	ProfilePhotoURL      string     `gorm:"not null;default:''" json:"profile_photo_url"`

	// Onboarding (self-signup). Existing admin-created partners default to
	// onboarding_step=5 / approval_status=approved. Bank fields are never
	// serialised (json:"-"); admin gets a dedicated endpoint later.
	VehicleType       string `json:"vehicle_type"`
	WarehouseID       *uint  `json:"warehouse_id,omitempty"`
	LicenceURL        string `json:"licence_url,omitempty"`
	RCURL             string `json:"rc_url,omitempty"`
	AadhaarURL        string `json:"aadhaar_url,omitempty"`
	SelfieURL         string `json:"selfie_url,omitempty"`
	IDDocType         string `gorm:"column:id_doc_type" json:"id_doc_type"`
	VoterURL          string `gorm:"column:voter_url" json:"voter_url,omitempty"`
	PANURL            string `gorm:"column:pan_url" json:"pan_url,omitempty"`
	BankAccountHolder string `json:"-"`
	BankAccountNo     string `json:"-"`
	BankIFSC          string `json:"-"`
	UPIID             string `json:"-"`
	OnboardingStep    int    `gorm:"not null;default:5" json:"onboarding_step"`
	ApprovalStatus    string `gorm:"not null;default:approved;index" json:"approval_status"`
	// MaxActiveOrders caps how many confirmed/shipped orders this partner
	// can be carrying at once. Auto-assign (and manual assign) must skip a
	// partner whose current active order count has reached this limit.
	// Configurable per partner; defaults to config.DefaultMaxActiveOrdersPerPartner
	// when not explicitly set on create.
	MaxActiveOrders int            `gorm:"not null;default:1" json:"max_active_orders"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// DeliveryPartnerRequest is the body for POST/PUT /admin/delivery-partners
type DeliveryPartnerRequest struct {
	Name          string `json:"name" binding:"required"`
	Phone         string `json:"phone" binding:"required,len=10,numeric"`
	VehicleNumber string `json:"vehicle_number"`
	IsActive      *bool  `json:"is_active"`
	// MaxActiveOrders is optional; when omitted, create falls back to the
	// configured default and update leaves the existing value untouched.
	MaxActiveOrders *int `json:"max_active_orders" binding:"omitempty,min=1"`
}

// AssignDeliveryPartnerRequest is the body for PUT /admin/orders/:id/assign-delivery
type AssignDeliveryPartnerRequest struct {
	DeliveryPartnerID uint `json:"delivery_partner_id" binding:"required"`
}

// UpdateLocationRequest is the body for PUT /delivery/location (delivery partner only)
type UpdateLocationRequest struct {
	Lat float64 `json:"lat" binding:"required"`
	Lng float64 `json:"lng" binding:"required"`
}

// UpdateOnlineStatusRequest is the body for PUT /delivery/status (delivery partner only)
type UpdateOnlineStatusRequest struct {
	IsOnline *bool `json:"is_online" binding:"required"`
}

// UpdateDeliveryProfileRequest is the body for PUT /delivery/profile
type UpdateDeliveryProfileRequest struct {
	Name          string `json:"name" binding:"required"`
	VehicleNumber string `json:"vehicle_number"`
}

// UpdateAvailabilityRequest is the body for PUT /delivery/availability
type UpdateAvailabilityRequest struct {
	Status string `json:"status" binding:"required,oneof=online offline"`
}
