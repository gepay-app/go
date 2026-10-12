-- Query tabel identity.users.

-- name: GetUserByAuthID :one
SELECT id, auth_id, email, name, status, created_at, updated_at
FROM identity.users
WHERE auth_id = $1;

-- name: GetUserByEmail :one
SELECT id, auth_id, email, name, status, created_at, updated_at
FROM identity.users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO identity.users (id, auth_id, email, name, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, auth_id, email, name, status, created_at, updated_at;

-- name: LinkAuthID :one
UPDATE identity.users
SET auth_id = $2, updated_at = now()
WHERE id = $1
RETURNING id, auth_id, email, name, status, created_at, updated_at;

-- name: UpdateUserStatus :one
UPDATE identity.users
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, auth_id, email, name, status, created_at, updated_at;
