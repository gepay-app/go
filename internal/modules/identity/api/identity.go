// Package api adalah kontrak modul identity: satu-satunya bagian yang boleh
// dipakai modul lain. Package ini tidak boleh mengimport ./internal.
package api

import (
	"context"
	"uuid"
)

type UserApi interface {
	UserByID(ctx context.Context, id uuid.UUID) (User, error)
}
