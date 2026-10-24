package models

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// ApplyProfileUpdate copies the optional profile fields from req onto u.
// A nil field means "not sent" and leaves the stored value unchanged.
func (u *User) ApplyProfileUpdate(req UpdateProfileRequest) error {
	if req.Email != nil {
		e := strings.TrimSpace(*req.Email)
		if e != "" {
			if _, err := mail.ParseAddress(e); err != nil || len(e) > 255 {
				return errors.New("Invalid email")
			}
		}
		u.Email = e
	}
	if req.Gender != nil {
		g := strings.TrimSpace(*req.Gender)
		if len(g) > 20 {
			return errors.New("Invalid gender")
		}
		u.Gender = g
	}
	if req.Dob != nil {
		d := strings.TrimSpace(*req.Dob)
		if d != "" {
			if len(d) < 10 || len(d) > 32 {
				return errors.New("Invalid dob")
			}
			if _, err := time.Parse("2006-01-02", d[:10]); err != nil {
				return errors.New("Invalid dob")
			}
		}
		u.Dob = d
	}
	if req.AvatarID != nil {
		a := strings.TrimSpace(*req.AvatarID)
		if len(a) > 64 {
			return errors.New("Invalid avatar_id")
		}
		u.AvatarID = a
	}
	return nil
}
