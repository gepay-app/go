package http

import (
	"net/mail"
	"unicode/utf8"

	"app/internal/modules/identity/api"
	"app/internal/platform/apperr"
)

// Validasi di sini HANYA untuk bentuk input: presence, format, dan keanggotaan
// enum. Aturan bisnis (hak admin, hierarki role, keberadaan user, cross-field
// yang butuh state) tetap di service.

const maxNameLen = 100 // samakan dengan VARCHAR(100) di migration

type createUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (r createUserRequest) validate() error {
	if r.Email == "" {
		return apperr.Validation(apperr.FieldError{Field: "email", Message: "is required"})
	}
	if addr, err := mail.ParseAddress(r.Email); err != nil || addr.Address != r.Email {
		return apperr.Validation(apperr.FieldError{Field: "email", Message: "must be a valid email"})
	}
	if utf8.RuneCountInString(r.Name) > maxNameLen {
		return apperr.Validation(apperr.FieldError{Field: "name", Message: "must be at most 100 characters"})
	}
	return nil
}

type updateStatusRequest struct {
	Status api.UserStatus `json:"status"`
}

func (r updateStatusRequest) validate() error {
	if !r.Status.Valid() {
		return apperr.Validation(apperr.FieldError{Field: "status", Message: "invalid status"})
	}
	return nil
}

type grantRoleRequest struct {
	Role api.Role `json:"role"`
}

func (r grantRoleRequest) validate() error {
	if !r.Role.Valid() {
		return apperr.Validation(apperr.FieldError{Field: "role", Message: "invalid role"})
	}
	return nil
}
