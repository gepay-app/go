# app

Starter **modular monolith** di Go: satu binary, modul-modul dengan batas yang ditegakkan compiler, logging terstruktur ber-`request_id`, error response seragam, dan persistensi Postgres/Redis siap pakai.

Stack: [Echo v5](https://echo.labstack.com) · `log/slog` · [pgx/v5](https://github.com/jackc/pgx) · [go-redis/v9](https://github.com/redis/go-redis) · [sqlc](https://sqlc.dev) (repository) · [goose](https://github.com/pressly/goose) (migrasi) · `caarlos0/env` (config).

Go 1.27: pakai stdlib dulu — `uuid` (`import "uuid"`) dan `encoding/json/v2` (`import "encoding/json/v2"`), bukan `github.com/google/uuid` atau `encoding/json`.

## Prasyarat

- Go 1.27+
- PostgreSQL dan Redis (bisa via Docker)

## Mulai cepat

```bash
cp .env.example .env          # lalu sesuaikan DB_URL dan REDIS_*
make migrate-up               # terapkan migrasi
make run                      # jalankan server
```

```bash
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8080/readyz    # cek Postgres & Redis
```

Server **fail-fast**: kalau Postgres atau Redis tidak bisa di-`Ping` saat startup, proses berhenti. `/healthz` = liveness, `/readyz` = readiness (200 kalau semua dependency `ok`, 503 kalau ada yang `down`).

## Make targets

| Target | Fungsi |
|---|---|
| `make run` | Jalankan server (`go run ./cmd/app`) |
| `make dev` | Jalankan dengan live reload ([air](https://github.com/air-verse/air)) |
| `make build` | Build binary ke `bin/app` |
| `make test` | Jalankan semua test |
| `make vet` / `make fmt` / `make tidy` | `go vet` / `gofmt` / `go mod tidy` |
| `make sqlc` | Generate kode repository dari query SQL (`sqlc generate`) |
| `make migrate-up` / `migrate-down` / `migrate-status` | Migrasi goose |
| `make migrate-reset` | Rollback semua migrasi lalu apply ulang (`goose reset && goose up`) |
| `make migrate-create name=create_users` | Buat file migrasi baru |

`make` membaca `.env` otomatis, jadi app dan goose memakai config yang sama.

## Dua aturan yang bikin semuanya nyambung

**1. Log selalu lewat context:**
```go
logging.FromContext(ctx).Info("user created", "user_id", id)
```
`ctx` membawa `request_id`, jadi setiap baris log otomatis bisa dilacak. Jangan pakai `slog.Info(...)` package-level atau logger base — request id-nya hilang diam-diam.

**2. Error selalu berupa `apperr`:**
```go
return apperr.NotFound("user_not_found", "user not found")
return apperr.Validation(apperr.FieldError{Field: "email", Message: "is required"})
return apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
```
Handler cukup `return err` — `ErrorHandler` merendernya jadi JSON. Jangan pakai `echo.NewHTTPError` atau `c.JSON(500, ...)` di business logic.

### Bentuk response error

```json
{"error":{"code":"user_not_found","message":"user not found"}}
{"error":{"code":"validation_failed","message":"request validation failed",
          "details":[{"field":"email","message":"is required"}]}}
```

`Kind` → status: `Invalid`→400, `Unauthorized`→401, `Forbidden`→403, `NotFound`→404, `Conflict`→409, `Internal`→500. Untuk 5xx, `message` generik dan `cause` hanya masuk log (tidak bocor ke client). Detail tambahan di luar validasi: `.WithDetails(any)`.

## Struktur proyek

```
cmd/app/                 entry point (tipis, hanya memanggil bootstrap)
internal/bootstrap/      composition root: config → logger → dependency → server
internal/platform/       infrastruktur bersama (bukan domain)
  config/                baca + validasi env
  logging/               logger slog + request_id di context
  apperr/                tipe error aplikasi (kind, code, message, details)
  postgres/ redis/       koneksi
  firebase/              Firebase Admin: verify ID token + create/delete user
  server/                server Echo + middleware (request id, access log, recover,
                         body limit, CORS, timeout, error handler, binder JSON-only)
internal/modules/        fitur bisnis (lihat di bawah)
db/postgres/migrations/  migrasi goose
db/postgres/queries/     query sqlc, satu file per tabel
```

Aplikasi memakai satu logger yang di-`SetDefault`, dan middleware `ContextLogger` menaruh logger ber-`request_id` ke context tiap request.

## Menambah modul

Salin pola `internal/modules/identity`:

```
internal/modules/<nama>/
├── module.go                     wiring: Dependencies + New + RegisterRoutes
├── api/                          KONTRAK: DTO + enum + interface untuk modul lain
└── internal/                     IMPLEMENTASI
    ├── service/                  business logic (satu struct, file per concern)
    ├── repository/               akses data (sqlc)
    ├── cache/                    cache per agregat (opsional)
    └── delivery/http/            handler + RegisterRoutes + request DTO (validate())
```

Lalu daftarkan di `internal/bootstrap/app.go`:
```go
mod := mymodule.New(mymodule.Dependencies{Logger: logger, DB: pool, Redis: rdb})
mod.RegisterRoutes(srv)
```

Modul lain yang butuh modul ini **hanya** menerima kontraknya (`api`), disuntikkan lewat bootstrap:
```go
// users: implementasi identityapi.UserApi
order.New(order.Dependencies{Users: users})
```

### Aturan arah dependensi

| Package | Boleh import |
|---|---|
| `api` | stdlib + `platform/*` — **tidak** `internal` |
| `internal/...` | `api` + `platform/*` + package internal lain |
| `module.go` | `api` + `internal/...` + `platform/*` |
| modul lain | **hanya `api`** modul itu |
| `bootstrap` | semuanya |

Direktori bernama `internal/` **ditegakkan compiler**: `.../identity/internal/...` hanya bisa diimport oleh package di bawah `.../identity/`. Jadi modul lain tidak bisa menyentuh implementasimu — gagal compile, bukan sekadar konvensi.

## Modul identity

Auth lewat Firebase: register/login dilakukan di frontend; backend hanya memverifikasi ID token (`Authorization: Bearer <firebase-id-token>`).

- `POST /identities/session` — provisioning idempoten setelah login Firebase (panggil sekali per app boot).
- `GET /identities/me` — profil user saat ini.

Endpoint admin (butuh role `ADMIN` atau `SUPER_ADMIN`):

| Method | Path | Body |
|---|---|---|
| GET | `/identities/admin/users?limit=&cursor=` | — (list user, keyset cursor) |
| POST | `/identities/admin/users` | `{email, name?}` — daftar ke Firebase + DB (role `USER`) |
| DELETE | `/identities/admin/users/:id` | — |
| PATCH | `/identities/admin/users/:id/status` | `{status: ACTIVE\|DISABLED}` |
| POST | `/identities/admin/users/:id/roles` | `{role: USER\|ADMIN\|SUPER_ADMIN}` |
| DELETE | `/identities/admin/users/:id/roles/:role` | — |

Admin tidak bisa memberi/mencabut role `SUPER_ADMIN`, mengubah/menghapus user `SUPER_ADMIN`, atau mengubah status/menghapus akunnya sendiri — semua itu hanya bisa oleh/di atas `SUPER_ADMIN`.

## Migrasi

SQL-first dengan goose (dijalankan via `go run` supaya `go.mod` tetap ramping):

```sql
-- +goose Up
CREATE TABLE users (
    id   uuid PRIMARY KEY,
    email text NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE users;
```

Query ditulis di `db/postgres/queries/<module>/<table>.sql`, lalu `make sqlc` meng-generate-nya ke `internal/modules/<module>/internal/repository/` (satu entry `sqlc.yaml` per modul, supaya repository tetap privat ke modulnya). File generated (`db.go`, `models.go`, `*.sql.go`) jangan diedit; setelah mengubah file query jalankan `make sqlc` — file generated yang usang tidak dihapus otomatis, jadi hapus manual kalau ada.

## Catatan

- **Config** divalidasi saat startup (`internal/platform/config`). Level/format log otomatis: production → info/json, lainnya → debug/text.
- **CORS nonaktif** secara default; isi `APP_CORS_ORIGINS` untuk mengaktifkan.
- **Body request hanya JSON** — media type lain ditolak `415` (`unsupported_media_type`).
- Batas bawaan: body 1 MiB, timeout request 15s, graceful shutdown 20s (selalu > timeout request).
- **Signup Firebase terbuka:** siapa pun yang bisa sign-up di project Firebase bisa `POST /identities/session` dan otomatis dapat akun `USER`. Kalau registrasi harus admin-only, matikan signup publik di Firebase atau tambahkan gerbang di `/session`.

## Dokumentasi lanjutan

Konvensi lengkap (aturan wajib, arah dependensi, sqlc, cache, migrasi, dll) ada di [`AGENTS.md`](./AGENTS.md).

## Go module

`make test` dan `go build ./...` seharusnya selalu hijau. Sebelum mengubah kontrak `apperr` atau `logging`, jalankan `go test ./...` — bagian tracing dan bentuk error punya tes regresinya.
