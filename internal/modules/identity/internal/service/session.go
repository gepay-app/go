package service

import (
	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/repository"
	"app/internal/platform/apperr"
	"app/internal/platform/firebase"
	"app/internal/platform/logging"
	"app/internal/platform/utils"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"uuid"
)

// Ensure menjamin user ada dan mengembalikan profilnya. Idempoten: aman
// dipanggil berkali-kali, termasuk bersamaan.

func (s *Service) Ensure(ctx context.Context, identity firebase.Identity) (api.User, error) {
	if u, ok := s.userCache.Get(ctx, identity.UID); ok {
		return s.checkStatus(u)
	}

	user, err := s.ensureDB(ctx, identity)
	if err != nil {
		return api.User{}, err
	}

	s.userCache.Set(ctx, identity.UID, user)

	return s.checkStatus(user)
}

func (s *Service) Resolve(ctx context.Context, uid string) (api.User, error) {
	if u, ok := s.userCache.Get(ctx, uid); ok {
		return s.checkStatus(u)
	}

	user, err := s.repo.GetUserByAuthID(ctx, utils.ToPgText(uid))
	if errors.Is(err, pgx.ErrNoRows) {
		// FE harus panggil POST /identities/session dulu.
		return api.User{}, apperr.Unauthorized("user_not_provisioned",
			"session is not provisioned")
	}
	if err != nil {
		logging.FromContext(ctx).Warn("failed to resolve user", "uid", uid, "error", err)
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	roles, err := s.repo.ListRolesByUserID(ctx, user.ID)
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")

	}

	u := toAPIUser(user, roles)
	s.userCache.Set(ctx, uid, u)
	return s.checkStatus(u)

}

// ! helper

// cek ke db karna ensure udah cek redis ga daept
func (s *Service) ensureDB(ctx context.Context, identity firebase.Identity) (api.User, error) {
	// Skema saat ini: email NOT NULL. Auth tanpa email (phone/passwordless) ga di dukung
	if identity.Email == "" {
		return api.User{}, apperr.Forbidden("email_required", "account without email is not supported")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	// defer setelah commit no-op santuy aja aman di defer
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.repo.WithTx(tx)

	user, err := q.GetUserByAuthID(ctx, utils.ToPgText(identity.UID))
	switch {
	case err == nil:
	case errors.Is(err, pgx.ErrNoRows):
		user, err = s.provision(ctx, q, identity)
		if err != nil {
			return api.User{}, err
		}
	default:
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	roles, err := q.ListRolesByUserID(ctx, user.ID)
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	if err := tx.Commit(ctx); err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	return toAPIUser(user, roles), nil
}

// provision menangani kasus "belum ada user dengan auth_id ini"
func (s *Service) provision(
	ctx context.Context,
	q *repository.Queries,
	identity firebase.Identity,
) (repository.IdentityUser, error) {
	byEmail, err := q.GetUserByEmail(ctx, identity.Email)
	switch {
	case err == nil:
		// Sudah ada user dengan email ini (mis. seeder SUPER_ADMIN yang
		// auth_id-nya masih NULL atau di buat manual lewat firebase console).
		if !identity.EmailVerified {
			return repository.IdentityUser{}, apperr.Forbidden("email_not_verified", "verify your email before signing in")
		}
		if byEmail.AuthID.Valid && byEmail.AuthID.String != identity.UID {
			return repository.IdentityUser{}, apperr.Conflict("email_already_registerd", "email is already registered")
		}

		linked, err := q.LinkAuthIDIfNull(ctx, repository.LinkAuthIDIfNullParams{
			ID:     byEmail.ID,
			AuthID: utils.ToPgText(identity.UID),
		})
		if err == nil {
			return s.grantDefaultRole(ctx, q, linked)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			// Kalah balapan: request lain sudah menautkan. Ambil yang menang.
			u, err := q.GetUserByAuthID(ctx, utils.ToPgText(identity.UID))
			if err != nil {
				return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
			}
			return s.grantDefaultRole(ctx, q, u)
		}

		return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")

	case errors.Is(err, pgx.ErrNoRows):
		created, err := q.CreateUserIfAbsent(ctx, repository.CreateUserIfAbsentParams{
			ID:     uuid.NewV7(),
			AuthID: utils.ToPgText(identity.UID),
			Email:  identity.Email,
			Name:   utils.ToPgTextPtr(&identity.Name),
			Status: string(api.StatusActive),
		})
		if err == nil {
			return s.grantDefaultRole(ctx, q, created)
		}

		if errors.Is(err, pgx.ErrNoRows) {
			// Bentrok: uid sama (balapan) ATAU uid beda dengan email sama.
			if u, err := q.GetUserByAuthID(ctx, utils.ToPgText(identity.UID)); err == nil {
				return s.grantDefaultRole(ctx, q, u)
			}
			return repository.IdentityUser{}, apperr.Conflict("email_already_registered",
				"email is already registered")
		}

		return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	default:
		return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
}

func toAPIUser(u repository.IdentityUser, roles []string) api.User {
	return api.User{
		ID:     u.ID,
		Email:  u.Email,
		Name:   u.Name.String,
		Status: api.UserStatus(u.Status),
		Roles:  toRoles(roles),
	}
}

// repository mengembalikan []string -> kontrak api memakai []Role.
func toRoles(roles []string) []api.Role {
	out := make([]api.Role, len(roles))
	for i, r := range roles {
		out[i] = api.Role(r)
	}
	return out
}

// grantDefaultRole memberi role USER saat user BARU di-provision. Sengaja TIDAK
// dipanggil untuk user lama, supaya pencabutan role USER oleh admin tidak
// dibatalkan tiap kali /session. Idempoten (ON CONFLICT DO NOTHING).
func (s *Service) grantDefaultRole(ctx context.Context, q *repository.Queries, user repository.IdentityUser) (repository.IdentityUser, error) {
	if err := q.GrantRole(ctx, repository.GrantRoleParams{
		UserID:    user.ID,
		Role:      string(api.RoleUser),
		GrantedBy: pgtype.UUID{}, // null, oleh sistem
	}); err != nil {
		return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	return user, nil
}
