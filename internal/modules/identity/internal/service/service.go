// Package service berisi business logic modul identity.
package service

import (
	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/cache"
	"app/internal/modules/identity/internal/repository"
	"app/internal/platform/apperr"
	"app/internal/platform/firebase"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service adalah use case modul identity. Satu struct; method dipecah per file:
//
//	session.go -> Ensure, Resolve
//	user.go    -> UpdateStatus, DeleteUser, CreateUser
//	role.go    -> GrantRole, RevokeRole
//	authz.go   -> otorisasi admin
type Service struct {
	db        *pgxpool.Pool
	repo      *repository.Queries
	userCache *cache.UserCache
	admin     firebase.UserAdmin
}

func New(db *pgxpool.Pool, userCache *cache.UserCache, admin firebase.UserAdmin) *Service {
	return &Service{db: db, repo: repository.New(db), userCache: userCache, admin: admin}
}

func (s *Service) checkStatus(u api.User) (api.User, error) {
	if u.Status != api.StatusActive {
		return api.User{}, apperr.Forbidden("account_disabled", "account is disabled")
	}
	return u, nil
}
