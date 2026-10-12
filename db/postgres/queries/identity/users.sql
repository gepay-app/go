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

-- name: DeleteUser :exec
DELETE FROM identity.users
WHERE id = $1;

-- Keyset pagination by id (UUIDv7 = urut waktu). Kirim uuid.Max() untuk halaman
-- pertama; ambil limit+1 supaya pemanggil tahu masih ada berikutnya.
-- name: ListUsers :many
SELECT id, auth_id, email, name, status, created_at, updated_at
FROM identity.users
WHERE id < $1
ORDER BY id DESC
LIMIT $2;
