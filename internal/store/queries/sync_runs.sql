-- name: CreateSyncRun :one
INSERT INTO sync_runs (repository_id, trigger_type, status, attempt, started_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: FinishSyncRun :exec
UPDATE sync_runs
SET status = ?, finished_at = ?, duration_ms = ?, error_code = ?, error_summary = ?, log_excerpt = ?
WHERE id = ?;

-- name: ListSyncRuns :many
SELECT * FROM sync_runs WHERE repository_id = ? ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: MarkRunningSyncRunsFailed :exec
UPDATE sync_runs
SET status = 'failed', finished_at = ?, error_code = 'INTERRUPTED',
    error_summary = 'Interrupted by service restart'
WHERE status = 'running';
