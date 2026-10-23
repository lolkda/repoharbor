-- name: ListRepositories :many
SELECT * FROM repositories ORDER BY name COLLATE NOCASE, owner COLLATE NOCASE;

-- name: GetRepository :one
SELECT * FROM repositories WHERE id = ? LIMIT 1;

-- name: GetRepositoryByURL :one
SELECT * FROM repositories WHERE url = ? LIMIT 1;

-- name: CreateRepository :one
INSERT INTO repositories (
  url, owner, name, display_name, notes, cron_expression, enabled, status,
  default_branch, last_sync_at, next_sync_at, last_error_code, last_error_message,
  size_bytes, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateRepositoryConfig :one
UPDATE repositories
SET display_name = ?, notes = ?, cron_expression = ?, enabled = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: UpdateRepositoryStatus :exec
UPDATE repositories
SET status = ?, last_error_code = ?, last_error_message = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateRepositoryAfterSync :exec
UPDATE repositories
SET status = ?, default_branch = ?, last_sync_at = ?, next_sync_at = ?,
    last_error_code = ?, last_error_message = ?, size_bytes = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateRepositoryNextSync :exec
UPDATE repositories SET next_sync_at = ?, updated_at = ? WHERE id = ?;

-- name: DeleteRepository :exec
DELETE FROM repositories WHERE id = ?;

-- name: ListDueRepositories :many
SELECT * FROM repositories
WHERE enabled = 1 AND status != 'syncing' AND next_sync_at != '' AND next_sync_at <= ?
ORDER BY next_sync_at;

-- name: ListEnabledRepositories :many
SELECT * FROM repositories WHERE enabled = 1;

-- name: MarkSyncingRepositoriesFailed :exec
UPDATE repositories
SET status = 'failed', last_error_code = 'INTERRUPTED',
    last_error_message = 'Interrupted by service restart', updated_at = ?
WHERE status = 'syncing';

