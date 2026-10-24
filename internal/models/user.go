package models

import "time"

// User is identified by phone number only â€” no email/password.
// Login happens via OTP sent to the phone (see OTP model + auth handlers).
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Phone     string    `gorm:"uniqueIndex;not null" json:"phone"`
	Role      string    `gorm:"default:customer" json:"role"` // customer / admin
	IsBlocked bool      `gorm:"default:false" json:"is_blocked"`
	AdminRole string    `gorm:"default:''" json:"admin_role,omitempty"` // super_admin/admin/ops_manager/warehouse_manager/support_agent/finance_manager (only meaningful when Role=admin)
	Email     string    `gorm:"size:255" json:"email"`
	Gender    string    `gorm:"size:20" json:"gender"`
	Dob       string    `gorm:"size:32" json:"dob"`
	AvatarID  string    `gorm:"size:64" json:"avatar_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Request/response DTOs

// SendOTPRequest is the body for POST /auth/send-otp
// Phone is expected as a 10-digit Indian mobile number (no country code, no spaces).
type SendOTPRequest struct {
	Phone string `json:"phone" binding:"required,len=10,numeric"`
}

// VerifyOTPRequest is the body for POST /auth/verify-otp
type VerifyOTPRequest struct {
	Phone string `json:"phone" binding:"required,len=10,numeric"`
	OTP   string `json:"otp" binding:"required,len=6,numeric"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// UpdateProfileRequest is the body for PUT /auth/me. Phone is intentionally
// excluded â€” it's the login identity, so changing it goes through a
// separate OTP-verified flow, not a plain profile edit.
type UpdateProfileRequest struct {
	Name     string  `json:"name" binding:"required"`
	Email    *string `json:"email"`
	Gender   *string `json:"gender"`
	Dob      *string `json:"dob"`
	AvatarID *string `json:"avatar_id"`
}
