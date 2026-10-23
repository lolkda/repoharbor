-- name: CountAdmins :one
SELECT COUNT(*) FROM admin_users;

-- name: GetAdminByUsername :one
SELECT * FROM admin_users WHERE username = ? LIMIT 1;

-- name: GetAdminByID :one
SELECT * FROM admin_users WHERE id = ? LIMIT 1;

-- name: CreateAdmin :one
INSERT INTO admin_users (username, password_hash, created_at, updated_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateAdminPassword :exec
UPDATE admin_users SET password_hash = ?, updated_at = ? WHERE id = ?;
