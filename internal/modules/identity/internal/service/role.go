package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"uuid"

	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/repository"
	"app/internal/platform/apperr"
)

// GrantRole memberi role ke user. Hanya admin; role SUPER_ADMIN hanya boleh
// diberikan oleh SUPER_ADMIN.
func (s *Service) GrantRole(ctx context.Context, actor api.User, userID uuid.UUID, role api.Role, grantedBy pgtype.UUID) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if role == api.RoleSuperAdmin && !isSuperAdmin(actor) {
		return apperr.Forbidden("forbidden", "only super admin can grant super admin role")
	}
	if err := s.userExists(ctx, userID); err != nil {
		return err
	}
	if err := s.protectSuperAdmin(ctx, actor, userID); err != nil {
		return err
	}

	if err := s.repo.GrantRole(ctx, repository.GrantRoleParams{
		UserID:    userID,
		Role:      string(role),
		GrantedBy: grantedBy,
	}); err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	s.invalidateByUserID(ctx, userID)
	return nil
}

// RevokeRole mencabut role dari user. Hanya admin; role SUPER_ADMIN hanya boleh
// dicabut oleh SUPER_ADMIN.
func (s *Service) RevokeRole(ctx context.Context, actor api.User, userID uuid.UUID, role api.Role) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if role == api.RoleSuperAdmin && !isSuperAdmin(actor) {
		return apperr.Forbidden("forbidden", "only super admin can revoke super admin role")
	}
	if err := s.userExists(ctx, userID); err != nil {
		return err
	}
	if err := s.protectSuperAdmin(ctx, actor, userID); err != nil {
		return err
	}

	if err := s.repo.RevokeRole(ctx, repository.RevokeRoleParams{
		UserID: userID,
		Role:   string(role),
	}); err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	s.invalidateByUserID(ctx, userID)
	return nil
}
