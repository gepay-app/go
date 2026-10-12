# AGENTS.md

Panduan kerja untuk repo **modular monolith** Go ini: arsitektur, konvensi, dan
alur menambah modul. Baca ini sebelum mengubah kode.

---

## 1. Apa ini

Satu binary, banyak modul dengan batas yang **ditegakkan compiler** lewat aturan
`internal/`. Bukan microservice, bukan monolith berlapis global.

Stack: [Echo v5](https://echo.labstack.com) · `log/slog` · [pgx/v5](https://github.com/jackc/pgx) ·
[go-redis/v9](https://github.com/redis/go-redis) · [sqlc](https://sqlc.dev) (repository) ·
[goose](https://github.com/pressly/goose) (migrasi) · `caarlos0/env` (config) ·
[air](https://github.com/air-verse/air) (dev).

## 2. Perintah

```bash
make run          # jalankan server
make dev          # live reload (air)
make build        # bin/app
make test         # semua test
make vet / fmt / tidy
make sqlc         # generate repository dari query SQL
make migrate-up / migrate-down / migrate-status
make migrate-reset        # rollback semua lalu apply ulang
make migrate-create name=create_users
```

`make` membaca `.env`. Server **fail-fast**: Postgres & Redis di-`Ping` saat
startup; kalau mati, proses berhenti.

## 3. Struktur

```
cmd/app/                     entry point (tipis -> bootstrap)
internal/bootstrap/          composition root: config -> logger -> deps -> server
internal/platform/           infrastruktur bersama (bukan domain)
  config/ logging/ apperr/ postgres/ redis/ firebase/
  server/                    Echo + middleware
  server/middleware/         request id, access log, recover, error, auth
internal/modules/<module>/   fitur bisnis
db/postgres/migrations/      goose
db/postgres/queries/<module>/  query sqlc, satu file per tabel
```

## 4. Aturan wajib

**1. Log selalu lewat context.** `ctx` membawa `request_id`.

```go
logging.FromContext(ctx).Info("user created", "user_id", id) // BENAR
slog.Info("user created")                                    // SALAH (id hilang)
```

**2. Error selalu `apperr`.** Handler cukup `return err`; `ErrorHandler` merender
JSON. Jangan `echo.NewHTTPError` atau `c.JSON(500, ...)` di business logic.

```go
return apperr.NotFound("user_not_found", "user not found")
return apperr.Validation(apperr.FieldError{Field: "email", Message: "is required"})
return apperr.Wrap(err, apperr.KindInternal, "db_error", "internal error")
```

`Kind` → status: `Invalid`→400, `Unauthorized`→401, `Forbidden`→403,
`NotFound`→404, `Conflict`→409, `Internal`→500. Untuk 5xx, `message` generik dan
`cause` hanya masuk log.

**3. Stdlib dulu (Go 1.27).** Jangan tambah dependency untuk hal yang sudah ada di
stdlib.

```go
import "uuid"                  // BUKAN github.com/google/uuid
id := uuid.NewV7()             // stdlib: uuid.UUID ([16]byte)

import "encoding/json/v2"      // BUKAN "encoding/json"
raw, err := json.Marshal(v)    // v2: lebih cepat, lebih hemat alokasi
err = json.Unmarshal(raw, &v)
```

## 5. Arah dependensi (ditegakkan compiler)

| Package | Boleh import |
|---|---|
| `api` | stdlib + `platform/*` — **tidak** `internal` |
| `internal/...` | `api` + `platform/*` + internal lain |
| `module.go` | `api` + `internal/...` + `platform/*` |
| modul lain | **hanya `api`** modul itu |
| `bootstrap` | semuanya |

Direktori `internal/` membuat kode di dalamnya **hanya** bisa diimport oleh package
di bawah modul yang sama — gagal compile, bukan konvensi. Karena itu repository,
cache, dan service selalu privat ke modulnya.

## 6. Anatomi modul

```
internal/modules/<module>/
├── module.go                 # SATU-SATUNYA titik wiring
├── api/                      # KONTRAK (satu package, file per konsep)
│   ├── <module>.go           #   package doc + interface yang dipakai modul lain
│   ├── user.go               #   tipe + enum per konsep
│   └── role.go
└── internal/
    ├── service/              # use case; satu struct, file per concern
    ├── repository/           # HASIL sqlc (generated, jangan diedit)
    ├── cache/                # cache per agregat (opsional, saat butuh)
    └── delivery/http/        # handler + RegisterRoutes + request DTO
```

### Aturan tiap lapisan

| Lapisan | Boleh tahu | Tidak boleh tahu |
|---|---|---|
| `delivery/http` | HTTP, service | SQL, Redis |
| `service` | repository, cache, `api` modul lain | HTTP, detail key Redis |
| `cache` | Redis, tipe `api` | SQL, aturan bisnis |
| `repository` | SQL/DB | Redis, HTTP |

**Service memegang transaksi** (dialah yang tahu atomisitas), repository cukup
menerima `Queries`/tx (`repo.WithTx(tx)`).

### `api`: kontrak modul

- Satu package, **file per konsep**; jangan subpackage (`api/user`) sampai
  kontraknya benar-benar besar.
- `<module>.go` = pintu masuk: package doc + interface yang dipakai modul lain.
- Enum (mis. `UserStatus`, `Role`) = **typed string** + konstanta, tinggal bareng
  tipenya, plus `Valid()` untuk nilai yang datang dari input. Nilai string HARUS
  sama dengan yang di DB; ubah = migrasi.

### `service`: use case

- **Satu struct** dulu; pecah jadi beberapa struct hanya kalau satu concern punya
  dependensi sendiri. Method dipisah per file concern (`session.go`, `user.go`, ...).
- Dependensi disuntik lewat `New(...)`; tidak ada `global`/singleton.
- Urutan panggilan satu arah: `delivery -> service -> {repository, cache, api modul lain}`.

### `delivery/http`: request DTO & validasi

- DTO request = struct + method `validate()` di file `request.go`, **package yang
  sama** (`http`), bukan subpackage. Tipe unexported.
- `validate()` HANYA bentuk input: presence, format, keanggotaan enum. Balikan
  `apperr.Validation(apperr.FieldError{Field, Message})` supaya FE dapat `details`.
- Aturan bisnis / cross-field yang butuh state (user ada, saldo, hierarki role)
  tetap di **service** (atau constructor command). Jangan query DB di `validate()`.
- Handler memetakan DTO -> input service (`service.XxxInput`); **service tidak boleh
  import** DTO http.
- Kalau service dipanggil dari >1 entrypoint (HTTP + worker + gRPC), pindahkan
  aturan ke constructor command supaya tidak bisa dilewati.

## 7. Data layer: sqlc

- SQL-first. Query ditulis di `db/postgres/queries/<module>/<table>.sql`.
- **Satu entry `sqlc.yaml` per modul** → output ke
  `internal/modules/<module>/internal/repository`. Boundary terjaga: modul lain
  tidak bisa mengimpor query ini.
- Setelah edit SQL: `make sqlc`.
- Generated (`db.go`, `models.go`, `*.sql.go`) **jangan diedit** (`DO NOT EDIT`).

```sql
-- name: GetUserByID :one      -> satu baris, pgx.ErrNoRows kalau kosong
-- name: ListRolesByUserID :many -> slice
-- name: GrantRole :exec       -> tanpa baris
```

Jebakan yang sering kena:

- **File generated usang tidak dihapus sqlc.** Kalau rename/hapus file query,
  `*.sql.go` lama tertinggal → error "duplicate method". Hapus manual, generate ulang.
- No-rows = **`pgx.ErrNoRows`**, bukan `sql.ErrNoRows` (karena `sql_package: pgx/v5`).
  Cek dengan `errors.Is(err, pgx.ErrNoRows)`.
- Sebut kolom eksplisit (bukan `SELECT *`) supaya struct generated stabil.
- Idempotensi tulis: pakai `ON CONFLICT DO NOTHING` / `WHERE ... IS NULL`,
  lalu re-select kalau `ErrNoRows` — jangan andalkan cek-lalu-tulis (race).
- Kolom `GENERATED ALWAYS AS IDENTITY`: jangan insert/assign `id` manual.
- **Pagination keyset (cursor), bukan `OFFSET`.** Keyset by `id` (UUIDv7 /
  `GENERATED ALWAYS AS IDENTITY` = urut total): `WHERE id < $cursor ORDER BY id DESC
  LIMIT n+1`. Halaman pertama pakai `uuid.Max()`. Sandikan cursor dengan
  `utils.EncodeCursor`/`utils.DecodeCursor`, batasi limit dengan `utils.ParseLimit`.

## 8. Cache

- **Per agregat, bukan per tabel, dan bukan generik.** Selama belum ada 3+ agregat
  yang benar-benar mengulang, buat tipe cache eksplisit (`cache/user.go`).
  Readability lebih penting dari abstraksi.
- Cache menyimpan **read-model** yang dipanaskan jalur request (mis. `user+roles`),
  bukan baris mentah tiap tabel.
- Key ber-namespace: `identity:user:<uid>`.
- TTL itu **jaring pengaman**, bukan penentu kesegaran — **invalidasi eksplisit**
  yang utama. Pilih 5–15 menit (mis. 10m); jangan menaruh TTL sebagai andalan
  kalau ada jalur mutasi.
- **Invalidasi di service** (method mutasi), bukan di handler. Satu tempat, jangan
  tersebar. Setiap method yang mengubah agregat wajib meng-invalidate.
- Cache tidak boleh menggagalkan request: log warning, lanjut (DB = sumber kebenaran).

## 9. Config & env

- Prefix per grup: `APP_`, `DB_`, `REDIS_`, `FIREBASE_`.
- Divalidasi saat startup di `internal/platform/config`; nilai wajib gagal-fast.
- Tambah field di `config.App` + `.env.example`. Jangan baca `os.Getenv` di
  luar `platform/config`.

## 10. Migrasi (goose)

- File di `db/postgres/migrations/`, SQL-first, ada `-- +goose Up` / `-- +goose Down`.
- Nama tabel `snake_case`; skema terpisah per modul (mis. `identity.users`).
- Buat: `make migrate-create name=create_users` lalu isi Up **dan** Down.
- `make migrate-reset` (`goose reset` + `up`) butuh **setiap** migration punya Down;
  kalau tidak, reset gagal di migration itu.

## 11. Server & shutdown

- Middleware (luar→dalam): body limit/CORS → request id → context logger → access
  log → timeout → recover.
- Body hanya `application/json` (lainnya `415`).
- Shutdown urut: sinyal → drain request in-flight (`gracefulTimeout`) → tutup
  Redis → tutup Postgres. `gracefulTimeout` **harus > `requestTimeout`**
  (sekarang `requestTimeout + 5s`) supaya tidak ada request yang masih jalan saat
  dependency ditutup.

## 12. Testing

- `make test` harus selalu hijau.
- Test paket yang sama (`package x`, bukan `x_test`) bila perlu akses internal.
- Integrasi DB: guard dengan `TEST_DB_URL` (+ `TEST_REDIS_ADDR` kalau butuh cache);
  `t.Skip` kalau kosong — jangan tambah dependency testcontainers untuk sekarang.
- Perubahan pada `apperr`/`logging` wajib lewat `go test ./...` (ada tes regresinya).

## 13. Dokumen lain

- `README.md` — gambaran umum + mulai cepat.
