-- name: ActiveJobByKey :one
SELECT id, dedupe_key, kind, state, target_id, request_json, result_json, error, created_at, started_at, finished_at
FROM jobs WHERE dedupe_key = ? AND state IN ('queued', 'running');

-- name: InsertJob :exec
INSERT INTO jobs (id, dedupe_key, kind, state, target_id, request_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: InsertSubscription :exec
INSERT INTO subscriptions (token_hash, job_id, created_at, expires_at) VALUES (?, ?, ?, ?);

-- name: GetJob :one
SELECT id, dedupe_key, kind, state, target_id, request_json, result_json, error, created_at, started_at, finished_at
FROM jobs WHERE id = ?;

-- name: JobByToken :one
SELECT j.id, j.dedupe_key, j.kind, j.state, j.target_id, j.request_json, j.result_json, j.error,
       j.created_at, j.started_at, j.finished_at
FROM subscriptions s JOIN jobs j ON j.id = s.job_id
WHERE s.token_hash = sqlc.arg(token_hash) AND s.expires_at > sqlc.arg(now);

-- name: MarkJobRunning :execrows
UPDATE jobs SET state = sqlc.arg(running), started_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = sqlc.arg(queued);

-- name: FinishJob :exec
UPDATE jobs SET state = ?, result_json = ?, error = ?, finished_at = ?
WHERE id = ? AND state IN ('queued', 'running');

-- name: AddMilestone :exec
INSERT INTO job_milestones (job_id, seq, stage, message, at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (job_id, seq) DO NOTHING;

-- name: Milestones :many
SELECT seq, stage, message, at FROM job_milestones WHERE job_id = ? ORDER BY seq;

-- name: FailActiveJobs :execrows
UPDATE jobs SET state = ?, error = ?, finished_at = ? WHERE state IN ('queued', 'running');

-- name: DeleteExpiredSubscriptions :exec
DELETE FROM subscriptions WHERE expires_at <= ?;

-- name: DeleteUnsubscribedFinishedJobs :execrows
DELETE FROM jobs WHERE state IN ('done', 'failed', 'dropped')
AND NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.job_id = jobs.id);
