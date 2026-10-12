package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"uuid"

	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/repository"
	"app/internal/platform/apperr"
	"app/internal/platform/firebase"
	"app/internal/platform/utils"
)

// UpdateStatus mengubah status user. Hanya admin, dan admin tidak boleh
// menyentuh user SUPER_ADMIN.
func (s *Service) UpdateStatus(ctx context.Context, actor api.User, userID uuid.UUID, status api.UserStatus) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if err := protectSelf(actor, userID); err != nil {
		return err
	}
	if err := s.protectSuperAdmin(ctx, actor, userID); err != nil {
		return err
	}

	if _, err := s.repo.UpdateUserStatus(ctx, repository.UpdateUserStatusParams{
		ID: userID, Status: string(status),
	}); errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("user_not_found", "user not found")
	} else if err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	s.invalidateByUserID(ctx, userID)
	return nil
}

// DeleteUser menghapus user di Firebase lalu di DB. Hanya admin, dan admin tidak
// boleh menghapus user SUPER_ADMIN.
func (s *Service) DeleteUser(ctx context.Context, actor api.User, userID uuid.UUID) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if err := protectSelf(actor, userID); err != nil {
		return err
	}

	target, err := s.repo.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("user_not_found", "user not found")
	}
	if err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	if err := s.protectSuperAdmin(ctx, actor, userID); err != nil {
		return err
	}

	// Hapus di Firebase dulu: kalau gagal, jangan hapus DB supaya user tidak
	// langsung ter-provision ulang saat login lagi.
	if target.AuthID.Valid {
		if err := s.admin.DeleteUser(ctx, target.AuthID.String); err != nil {
			return apperr.Wrap(err, apperr.KindInternal, "firebase_error", "internal server error")
		}
	}

	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	if target.AuthID.Valid {
		s.userCache.Invalidate(ctx, target.AuthID.String)
	}
	return nil
}

// CreateUserInput adalah input admin untuk membuat user baru.
type CreateUserInput struct {
	Email string
	Name  string
}

// CreateUser mendaftarkan user ke Firebase lalu ke DB (role default USER).
// Password tidak disentuh di sini; user mengurusnya sendiri lewat Firebase/FE.
func (s *Service) CreateUser(ctx context.Context, actor api.User, in CreateUserInput) (api.User, error) {
	if err := requireAdmin(actor); err != nil {
		return api.User{}, err
	}

	uid, err := s.admin.CreateUser(ctx, in.Email, in.Name)
	if err != nil {
		if errors.Is(err, firebase.ErrEmailExists) {
			return api.User{}, apperr.Conflict("email_already_registered", "email is already registered")
		}
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "firebase_error", "internal server error")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		// Kompensasi: jangan tinggalkan user Firebase yatim.
		_ = s.admin.DeleteUser(ctx, uid)
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.repo.WithTx(tx)

	user, err := q.CreateUserIfAbsent(ctx, repository.CreateUserIfAbsentParams{
		ID:     uuid.NewV7(),
		AuthID: utils.ToPgText(uid),
		Email:  in.Email,
		Name:   utils.ToPgTextPtr(&in.Name),
		Status: string(api.StatusActive),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.admin.DeleteUser(ctx, uid)
		return api.User{}, apperr.Conflict("email_already_registered", "email is already registered")
	}
	if err != nil {
		_ = s.admin.DeleteUser(ctx, uid)
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	if err := q.GrantRole(ctx, repository.GrantRoleParams{
		UserID:    user.ID,
		Role:      string(api.RoleUser),
		GrantedBy: pgtype.UUID{},
	}); err != nil {
		_ = s.admin.DeleteUser(ctx, uid)
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	roles, err := q.ListRolesByUserID(ctx, user.ID)
	if err != nil {
		_ = s.admin.DeleteUser(ctx, uid)
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	if err := tx.Commit(ctx); err != nil {
		_ = s.admin.DeleteUser(ctx, uid)
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	return toAPIUser(user, roles), nil
}

// Cache di-key oleh uid (auth_id), sedangkan mutasi tahu userID -> cari auth_id
// dulu lalu invalidate. Mutasi jarang, jadi satu query tambahan di sini tidak masalah.
func (s *Service) invalidateByUserID(ctx context.Context, userID uuid.UUID) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err == nil && u.AuthID.Valid {
		s.userCache.Invalidate(ctx, u.AuthID.String)
	}
}

// UserByID mengimplementasi api.UserApi untuk dipakai modul lain. Read-only dan
// TIDAK mengecek status — pemanggil yang memutuskan apa arti status itu.
func (s *Service) UserByID(ctx context.Context, id uuid.UUID) (api.User, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.User{}, apperr.NotFound("user_not_found", "user not found")
	}
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	roles, err := s.repo.ListRolesByUserID(ctx, user.ID)
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}
	return toAPIUser(user, roles), nil
}

// ListUsers mengembalikan halaman user terurut id DESC (keyset). cursorID =
// uuid.Max() berarti halaman pertama; limit+1 diambil untuk tahu masih ada lagi.
// Roles sengaja tidak diisi di list (hindari N+1) — ambil detail lewat UserByID.
func (s *Service) ListUsers(ctx context.Context, actor api.User, cursorID uuid.UUID, limit int) ([]api.User, bool, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, false, err
	}

	rows, err := s.repo.ListUsers(ctx, repository.ListUsersParams{
		ID:    cursorID,
		Limit: int32(limit + 1),
	})
	if err != nil {
		return nil, false, apperr.Wrap(err, apperr.KindInternal, "internal", "internal server error")
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	users := make([]api.User, 0, len(rows))
	for _, r := range rows {
		users = append(users, toAPIUser(r, nil))
	}
	return users, hasMore, nil
}
