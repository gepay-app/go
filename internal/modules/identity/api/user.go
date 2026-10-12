package api

import (
	"uuid"
)

type UserStatus string

const (
	StatusActive   UserStatus = "ACTIVE"
	StatusDisabled UserStatus = "DISABLED"
)

// Valid dipakai saat status datang dari input client (endpoint admin).
func (s UserStatus) Valid() bool {
	switch s {
	case StatusActive, StatusDisabled:
		return true
	}
	return false
}

type User struct {
	ID     uuid.UUID  `json:"id"`
	Email  string     `json:"email"`
	Name   string     `json:"name"`
	Status UserStatus `json:"status"`
	Roles  []Role     `json:"roles"`
}

// HasRole dipakai untuk otorisasi (cek role actor).
func (u User) HasRole(r Role) bool {
	for _, role := range u.Roles {
		if role == r {
			return true
		}
	}
	return false
}
