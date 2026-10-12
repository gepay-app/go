# Identity — Login/Register Flawless (Implementation Guide)

> Dokumen ini adalah blueprint implementasi, bukan kode final. Isinya langkah,
> SQL, dan sketsa kode yang tinggal kamu tulis/rapikan di modul `identity`.
> Konvensi mengikuti repo ini: `apperr`, `logging.FromContext`, repository hasil
> `sqlc`, dan batas modul lewat `internal/`.

---

## 0. Ringkasan & keputusan desain

Register/login terjadi **di Firebase, bukan di backend**. Backend hanya melihat
`Identity{UID, Email, EmailVerified, Name}` setelah token diverifikasi
`RequireAuth`. Karena itu backend butuh satu operasi **ensure / get-or-create
yang idempoten**, aman dipanggil berkali-kali dan bersamaan.

Keputusan yang sudah dikunci:

| Topik | Keputusan |
|---|---|
| Trigger provisioning | Endpoint eksplisit `POST /identities/session` (idempoten) |
| Aggregate modul lain (ledger, dll) | **Lazy per-modul** — modulnya sendiri yang `Ensure` saat pertama dipakai. Tidak ada orkestrasi lintas modul |
| Cache | **Semuanya** (termasuk status) di Redis + invalidasi eksplisit saat mutasi |
| `email_verified` | Tolak (`403`) **hanya** saat linking ke user lama yang `auth_id IS NULL` |

Prinsip yang dipegang: **`auth_id` (UID Firebase) = kunci kebenaran.** Email hanya
dipakai sekali untuk linking. Status & role dibaca dari DB (via cache), **bukan**
dari custom claims — supaya disable/role change langsung berlaku tanpa nunggu
token expired.

---

## 1. Flow

```
FE register/login ke Firebase  ->  dapat ID token
        |
FE POST /identities/session   (Authorization: Bearer <token>)
        |
   RequireAuth (verifikasi token, sudah ada)  ->  Identity di context
        |
   service.Ensure(Identity)
        |   tx: cari by auth_id -> link by email -> / create -> grant role USER
        v
   200 { data: { id, email, name, status, roles } }
        |   403 email_not_verified  (linking ditolak)
        |   403 account_disabled
        |   409 email_already_registered

FE request lain  ->  GET /identities/me
        |
   service.Resolve(uid)      [Redis -> DB, READ-ONLY, tidak menulis]
        v
   200 { data: {...} }    |    401 user_not_provisioned (FE harus panggil /session)
```

`RequireAuth` **tidak berubah** — tetap hanya memverifikasi token. Yang berubah:
`GET /identities/me` berhenti meng-echo token, sekarang resolve dari DB/Redis.

---

## 2. File yang terlibat

```
db/postgres/queries/identity/users.sql        (ubah + tambah query)
db/postgres/queries/identity/user_roles.sql   (tetap, nanti + RevokeRole)
internal/modules/identity/api/api.go          (DTO + kontrak UserReader)
internal/modules/identity/internal/service/service.go           (Ensure, Resolve, cache)
internal/modules/identity/internal/delivery/http/routes.go      (daftarkan /session)
internal/modules/identity/internal/delivery/http/auth.go        (handler session & me)
internal/modules/identity/module.go           (wiring: sudah cukup, cek saja)
```

---

## 3. Query (SQL first — jalankan `make sqlc` setelahnya)

### 3.1 `db/postgres/queries/identity/users.sql`

Ubah `LinkAuthID` -> `LinkAuthIDIfNull` (race-safe), tambah `GetUserByID`,
tambah `CreateUserIfAbsent`, dan rename `CreateUser` -> `CreateUserIfAbsent`.

```sql
-- Query tabel identity.users.

-- name: GetUserByAuthID :one
SELECT id, auth_id, email, name, status, created_at, updated_at
FROM identity.users
WHERE auth_id = $1;

-- name: GetUserByID :one
SELECT id, auth_id, email, name, status, created_at, updated_at
FROM identity.users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, auth_id, email, name, status, created_at, updated_at
FROM identity.users
WHERE email = $1;

-- Idempoten: ON CONFLICT DO NOTHING -> kalau bentrok, RETURNING kosong
-- (pgx.ErrNoRows) dan pemanggil re-select.
-- name: CreateUserIfAbsent :one
INSERT INTO identity.users (id, auth_id, email, name, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING
RETURNING id, auth_id, email, name, status, created_at, updated_at;

-- Hanya menautkan kalau masih NULL, supaya tidak menimpa link yang sudah jadi.
-- name: LinkAuthIDIfNull :one
UPDATE identity.users
SET auth_id = $2, updated_at = now()
WHERE id = $1 AND auth_id IS NULL
RETURNING id, auth_id, email, name, status, created_at, updated_at;

-- name: UpdateUserStatus :one
UPDATE identity.users
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, auth_id, email, name, status, created_at, updated_at;
```

### 3.2 `db/postgres/queries/identity/user_roles.sql`

Tetap seperti sekarang (`ListRolesByUserID`, `GrantRole`). `GrantRole` sudah
`ON CONFLICT DO NOTHING` sehingga aman dipanggil berulang (role lama tidak
ter-reset).

> **Setelah edit SQL:** `make sqlc`. Kalau kamu **rename** file query atau
> menghapus query, file `.sql.go` lama bisa tertinggal dan bikin error
> "duplicate method" — hapus manual lalu generate ulang.

---

## 4. Kontrak `api` (yang boleh dipakai modul lain)

`internal/modules/identity/api/api.go`:

```go
// Package api adalah kontrak modul identity: satu-satunya bagian yang boleh
// dipakai modul lain. Package ini tidak boleh mengimport ./internal.
package api

import (
	"context"
	"uuid"
)

// Nilai kontrak yang dipakai lintas layer. Jangan ganti stringnya tanpa migrasi.
const (
	StatusActive   = "ACTIVE"
	StatusDisabled = "DISABLED"

	RoleUser       = "USER"
	RoleSuperAdmin = "SUPER_ADMIN"
)

// User adalah bentuk publik user untuk modul lain dan untuk response HTTP.
type User struct {
	ID     uuid.UUID `json:"id"`
	Email  string    `json:"email"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Roles  []string  `json:"roles"`
}

// UserReader dipakai modul lain (mis. ledger) lewat kontrak ini, bukan internal.
type UserReader interface {
	UserByID(ctx context.Context, id uuid.UUID) (User, error)
}
```

Catatan: `uuid` di sini adalah **stdlib Go 1.27** (`import "uuid"`), sama seperti
yang dipakai `repository` dan middleware request id.

---

## 5. Service — `Ensure`, `Resolve`, cache

### 5.1 Struktur & cache helper

```go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"uuid"

	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/repository"
	"app/internal/platform/apperr"
	"app/internal/platform/firebase"
	"app/internal/platform/logging"
)

const cacheTTL = 60 * time.Second

type Service struct {
	db   *pgxpool.Pool          // untuk transaksi
	repo *repository.Queries    // query di luar transaksi
	rdb  *redis.Client
}

func New(db *pgxpool.Pool, rdb *redis.Client) *Service {
	return &Service{db: db, repo: repository.New(db), rdb: rdb}
}

func cacheKey(uid string) string { return "identity:v1:user:" + uid }

func (s *Service) cacheGet(ctx context.Context, uid string) (api.User, bool) {
	raw, err := s.rdb.Get(ctx, cacheKey(uid)).Bytes()
	if err != nil {
		return api.User{}, false
	}
	var u api.User
	if err := json.Unmarshal(raw, &u); err != nil {
		return api.User{}, false
	}
	return u, true
}

func (s *Service) cacheSet(ctx context.Context, uid string, u api.User) {
	raw, err := json.Marshal(u)
	if err != nil {
		return
	}
	if err := s.rdb.Set(ctx, cacheKey(uid), raw, cacheTTL).Err(); err != nil {
		// Cache tidak boleh menggagalkan request; DB tetap sumber kebenaran.
		logging.FromContext(ctx).Warn("identity cache set failed", "error", err)
	}
}

func (s *Service) cacheDel(ctx context.Context, uid string) {
	if err := s.rdb.Del(ctx, cacheKey(uid)).Err(); err != nil {
		logging.FromContext(ctx).Warn("identity cache del failed", "error", err)
	}
}
```

### 5.2 `Ensure` (get-or-create idempoten)

```go
// Ensure menjamin user ada dan mengembalikan profilnya. Idempoten: aman
// dipanggil berkali-kali, termasuk bersamaan.
func (s *Service) Ensure(ctx context.Context, id firebase.Identity) (api.User, error) {
	if u, ok := s.cacheGet(ctx, id.UID); ok {
		return s.checkStatus(u)
	}

	u, err := s.ensureDB(ctx, id)
	if err != nil {
		return api.User{}, err
	}

	s.cacheSet(ctx, id.UID, u)
	return s.checkStatus(u)
}

func (s *Service) ensureDB(ctx context.Context, id firebase.Identity) (api.User, error) {
	// Skema saat ini: email NOT NULL. Auth tanpa email (phone/passwordless)
	// belum didukung - lihat bagian 8.
	if id.Email == "" {
		return api.User{}, apperr.Forbidden("email_required",
			"account without email is not supported")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}
	// Rollback setelah Commit adalah no-op; aman di-defer.
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.repo.WithTx(tx)

	user, err := q.GetUserByAuthID(ctx, text(id.UID))
	switch {
	case err == nil:
		// sudah pernah login -> lanjut grant role & baca role
	case errors.Is(err, pgx.ErrNoRows):
		user, err = s.provision(ctx, q, id)
		if err != nil {
			return api.User{}, err
		}
	default:
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}

	// Idempoten: user baru dapat role USER; user lama role-nya tidak di-reset.
	if err := q.GrantRole(ctx, repository.GrantRoleParams{
		ID:        uuid.NewV7(),
		UserID:    user.ID,
		Role:      api.RoleUser,
		GrantedBy: pgtype.UUID{}, // NULL = granted oleh sistem
	}); err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}

	roles, err := q.ListRolesByUserID(ctx, user.ID)
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}

	if err := tx.Commit(ctx); err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}

	return toAPIUser(user, roles), nil
}

// provision menangani kasus "belum ada user dengan auth_id ini".
func (s *Service) provision(
	ctx context.Context,
	q *repository.Queries,
	id firebase.Identity,
) (repository.IdentityUser, error) {
	byEmail, err := q.GetUserByEmail(ctx, id.Email)
	switch {
	case err == nil:
		// Sudah ada user dengan email ini (mis. seeder SUPER_ADMIN yang
		// auth_id-nya masih NULL).
		if !id.EmailVerified {
			return repository.IdentityUser{}, apperr.Forbidden("email_not_verified",
				"verify your email before signing in")
		}
		if byEmail.AuthID.Valid && byEmail.AuthID.String != id.UID {
			return repository.IdentityUser{}, apperr.Conflict("email_already_registered",
				"email is already registered")
		}
		linked, err := q.LinkAuthIDIfNull(ctx, repository.LinkAuthIDIfNullParams{
			ID:     byEmail.ID,
			AuthID: text(id.UID),
		})
		if err == nil {
			return linked, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			// Kalah balapan: request lain sudah menautkan. Ambil yang menang.
			return q.GetUserByAuthID(ctx, text(id.UID))
		}
		return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")

	case errors.Is(err, pgx.ErrNoRows):
		created, err := q.CreateUserIfAbsent(ctx, repository.CreateUserIfAbsentParams{
			ID:     uuid.NewV7(),
			AuthID: text(id.UID),
			Email:  id.Email,
			Name:   nullable(id.Name),
			Status: api.StatusActive,
		})
		if err == nil {
			return created, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			// Bentrok: uid sama (balapan) ATAU uid beda dengan email sama.
			if u, err := q.GetUserByAuthID(ctx, text(id.UID)); err == nil {
				return u, nil
			}
			return repository.IdentityUser{}, apperr.Conflict("email_already_registered",
				"email is already registered")
		}
		return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")

	default:
		return repository.IdentityUser{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}
}
```

Helper kecil (taruh di service):

```go
func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

func nullable(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func toAPIUser(u repository.IdentityUser, roles []string) api.User {
	return api.User{
		ID:     u.ID,
		Email:  u.Email,
		Name:   u.Name.String,
		Status: u.Status,
		Roles:  roles,
	}
}

func (s *Service) checkStatus(u api.User) (api.User, error) {
	if u.Status != api.StatusActive {
		return api.User{}, apperr.Forbidden("account_disabled", "account is disabled")
	}
	return u, nil
}
```

### 5.3 `Resolve` (read-only, dipakai `GET /me`)

```go
func (s *Service) Resolve(ctx context.Context, uid string) (api.User, error) {
	if u, ok := s.cacheGet(ctx, uid); ok {
		return s.checkStatus(u)
	}

	user, err := s.repo.GetUserByAuthID(ctx, text(uid))
	if errors.Is(err, pgx.ErrNoRows) {
		// FE harus panggil POST /identities/session dulu.
		return api.User{}, apperr.Unauthorized("user_not_provisioned",
			"session is not provisioned")
	}
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}

	roles, err := s.repo.ListRolesByUserID(ctx, user.ID)
	if err != nil {
		return api.User{}, apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}

	u := toAPIUser(user, roles)
	s.cacheSet(ctx, uid, u)
	return s.checkStatus(u)
}
```

### 5.4 Mutasi + invalidasi cache (satu tempat, jangan di handler)

```go
func (s *Service) UpdateStatus(ctx context.Context, userID uuid.UUID, status string) error {
	if _, err := s.repo.UpdateUserStatus(ctx, repository.UpdateUserStatusParams{
		ID: userID, Status: status,
	}); err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}
	s.invalidateByUserID(ctx, userID)
	return nil
}

func (s *Service) GrantRole(ctx context.Context, userID uuid.UUID, role string, grantedBy pgtype.UUID) error {
	if err := s.repo.GrantRole(ctx, repository.GrantRoleParams{
		ID: uuid.NewV7(), UserID: userID, Role: role, GrantedBy: grantedBy,
	}); err != nil {
		return apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
	}
	s.invalidateByUserID(ctx, userID)
	return nil
}

// Cache di-key oleh uid (auth_id), sedangkan mutasi tahu userID -> cari auth_id
// dulu lalu DEL. Mutasi jarang, jadi satu query tambahan di sini tidak masalah.
func (s *Service) invalidateByUserID(ctx context.Context, userID uuid.UUID) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err == nil && u.AuthID.Valid {
		s.cacheDel(ctx, u.AuthID.String)
	}
}
```

### 5.5 (Opsional) hardening konkurensi

Constraint `auth_id UNIQUE` + `email UNIQUE` + `ON CONFLICT` + `LinkAuthIDIfNull`
sudah membuat `Ensure` benar tanpa lock. Kalau nanti kamu **mengukur** kontensi
tinggi pada uid yang sama, tambahkan serialisasi per-uid di awal transaksi:

```sql
-- name: LockUserProvision :exec
SELECT pg_advisory_xact_lock(hashtextextended($1, 0));
```

> Ini **opsional**, bukan syarat kebenaran. Coba dulu tanpa lock; tambahkan hanya
> kalau ada bukti kontensi. (Anotasi sqlc untuk SELECT fungsi void mungkin perlu
> disesuaikan antara `:exec` / `:execrows`.)

---

## 6. Delivery

### 6.1 `routes.go`

```go
func RegisterRoutes(e *echo.Echo, verifier firebase.Verifier, svc *service.Service) {
	h := newHandler(svc)
	requireAuth := middleware.RequireAuth(verifier)
	g := e.Group("/identities", requireAuth)
	g.POST("/session", h.session)
	g.GET("/me", h.me)
}
```

### 6.2 `auth.go`

```go
func (h *handler) session(c *echo.Context) error {
	id, ok := middleware.Identity(c.Request().Context())
	if !ok {
		return apperr.Unauthorized("unauthorized", "unauthenticated")
	}
	u, err := h.svc.Ensure(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": u})
}

func (h *handler) me(c *echo.Context) error {
	id, ok := middleware.Identity(c.Request().Context())
	if !ok {
		return apperr.Unauthorized("unauthorized", "unauthenticated")
	}
	u, err := h.svc.Resolve(c.Request().Context(), id.UID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": u})
}
```

Handler cukup `return err` — `ErrorHandler` merender `apperr` jadi JSON. **Jangan**
pakai `c.JSON(401, ...)` mentah seperti versi `me` sekarang.

### 6.3 `module.go`

Wiring sudah benar (`service.New(deps.DB, deps.Redis)`), tidak perlu diubah.
Pastikan `RegisterRoutes` meneruskan `verifier` (sudah).

---

## 7. Kontrak untuk frontend

| Endpoint | Sukses | Error penting |
|---|---|---|
| `POST /identities/session` | `200 {data:{...}}` | `403 email_not_verified`, `403 account_disabled`, `403 email_required`, `409 email_already_registered` |
| `GET /identities/me` | `200 {data:{...}}` | `401 user_not_provisioned` (panggil `/session`), `403 account_disabled` |

Alur FE:
1. Sign-in/sign-up Firebase.
2. Ambil ID token (refresh otomatis oleh SDK).
3. `POST /identities/session` — sekali per app boot / setelah login.
4. Selanjutnya pakai `/me` & endpoint lain.

`session` idempoten → aman dipanggil tiap boot tanpa mengecek "sudah pernah atau belum".

---

## 8. Edge case & kebijakan

- **Email `NOT NULL` di skema.** Auth tanpa email (phone) tidak bisa diprovision →
  `403 email_required`. Kalau nanti perlu, migrasi: buat `email` nullable atau
  tambah kolom identitas lain.
- **User ganti email di Firebase.** `Ensure`/`Resolve` tetap ketemu via `auth_id`;
  kolom `email` di DB bisa basi. Opsional: `UpdateUserEmail` saat sync (awas
  bentrok unique). Untuk sekarang, `auth_id` tetap kunci kebenaran.
- **Dua akun Firebase beda, email sama** → `409`, tidak membuat baris kedua.
- **User dibuat di Firebase tapi tak pernah panggil backend** → tidak ada di DB;
  normal, muncul saat pertama butuh backend.
- **Status DISABLED** → `403`; tidak mengirim apa pun ke Firebase (kendali di DB).
- **Disable/role change** langsung berlaku karena cache di-invalidate, bukan karena
  token berubah.

---

## 9. Test plan

### 9.1 Unit (tanpa DB) — fake verifier + fake repo

Buat `Service` bisa diuji dengan repository interface, atau uji lewat cabang
murni. Minimal cover:
- user sudah ada by `auth_id` → tidak buat ulang, role tidak di-reset.
- link by email + `email_verified=true` → sukses.
- link by email + `email_verified=false` → `403 email_not_verified`.
- email ada tapi `auth_id` milik uid lain → `409`.
- email tidak ada → create + grant role.
- status `DISABLED` → `403 account_disabled`.
- cache hit → tidak menyentuh DB (borrow a spy).

### 9.2 Konkurensi

- 2 goroutine `Ensure` uid sama → assert 1 baris user.
- 2 uid berbeda, email sama → assert 1 baris + salah satu `409`.

### 9.3 Integrasi (butuh DB nyata)

Guard dengan env `TEST_DB_URL`; `t.Skip` kalau kosong — supaya `make test` tetap
hijau tanpa DB (tanpa menambah dependency testcontainers).

```go
func TestEnsureIntegration(t *testing.T) {
	url := os.Getenv("TEST_DB_URL")
	if url == "" {
		t.Skip("TEST_DB_URL not set")
	}
	// connect, migrate, jalankan Ensure, assert constraint & idempotensi.
}
```

---

## 10. Checklist implementasi

- [ ] Edit `users.sql`: `GetUserByID`, `CreateUserIfAbsent`, `LinkAuthIDIfNull`,
      `UpdateUserStatus`. Hapus `CreateUser`/`LinkAuthID` lama.
- [ ] `make sqlc` → cek `internal/repository/*.sql.go`; hapus file generated usang
      kalau ada.
- [ ] `api/api.go`: `User`, `UserReader`, konstanta status/role.
- [ ] `service.go`: helper cache, `Ensure`, `ensureDB`, `provision`, `Resolve`,
      `UpdateStatus`, `GrantRole`, `invalidateByUserID`.
- [ ] `delivery/http/routes.go`: daftarkan `POST /session`.
- [ ] `delivery/http/auth.go`: handler `session` & `me` (pakai `apperr`, bukan JSON mentah).
- [ ] Test unit + konkurensi; `make test`.
- [ ] (Opsional) integrasi DB behind `TEST_DB_URL`.

---

## 11. Yang belum diputuskan

- **`Resolve` self-heal?** Rekomendasi: **tidak**. `Resolve` read-only; grant role
  cukup di `Ensure`. Kalau user lama tanpa role, panggil `/session` sekali.
- **TTL cache.** 60s. Turunkan ke 30s kalau disable harus lebih cepat terasa.
- **`UserReader`** — belum ada pemakainya; tambahkan saat modul ledger butuh.
