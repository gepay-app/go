-- Query tabel identity.user_roles.

-- name: ListRolesByUserID :many
SELECT role
FROM identity.user_roles
WHERE user_id = $1
ORDER BY role;

-- name: GrantRole :exec
INSERT INTO identity.user_roles (id, user_id, role, granted_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, role) DO NOTHING;
