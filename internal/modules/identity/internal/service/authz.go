package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"uuid"

	"app/internal/modules/identity/api"
	"app/internal/platform/apperr"
)

// requireAdmin: actor harus punya role ADMIN atau SUPER_ADMIN.
func requireAdmin(actor api.User) error {
	if !actor.HasRole(api.RoleAdmin) && !actor.HasRole(api.RoleSuperAdmin) {
		return apperr.Forbidden("forbidden", "admin role required")
	}
	return nil
}

func isSuperAdmin(u api.User) bool { return u.HasRole(api.RoleSuperAdmin) }

// protectSelf: admin tidak bisa mengubah status atau menghapus akunnya sendiri
// (mencegah lockout tak sengaja).
func protectSelf(actor api.User, targetID uuid.UUID) error {
	if actor.ID == targetID {
		return apperr.Forbidden("forbidden", "cannot modify your own account")
	}
	return nil
}

// userExists memetakan user yang tidak ada ke 404 (dipakai sebelum mutasi role).
func (s *Service) userExists(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.repo.GetUserByID(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("user_not_found", "user not found")
	} else if err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	return nil
}

// protectSuperAdmin: hanya SUPER_ADMIN yang boleh menyentuh user SUPER_ADMIN.
func (s *Service) protectSuperAdmin(ctx context.Context, actor api.User, targetID uuid.UUID) error {
	if isSuperAdmin(actor) {
		return nil
	}
	roles, err := s.repo.ListRolesByUserID(ctx, targetID)
	if err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	for _, r := range roles {
		if api.Role(r) == api.RoleSuperAdmin {
			return apperr.Forbidden("forbidden", "cannot modify a super admin")
		}
	}
	return nil
}
