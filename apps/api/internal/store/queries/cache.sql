-- name: GetFeatures :one
SELECT features_json, features_hash, created_at, expires_at FROM feature_cache
WHERE target_id = sqlc.arg(target_id) AND params_hash = sqlc.arg(params_hash)
  AND shaula_version = sqlc.arg(shaula_version)
  AND superseded_at IS NULL AND (expires_at IS NULL OR expires_at > sqlc.arg(now));

-- name: LatestFeatures :one
SELECT shaula_version, params_hash, features_json, features_hash, created_at, expires_at FROM feature_cache
WHERE target_id = ?
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: PutFeatures :exec
INSERT INTO feature_cache
  (target_id, params_hash, shaula_version, features_json, features_hash, created_at, expires_at, superseded_at)
VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
ON CONFLICT (target_id, params_hash, shaula_version) DO UPDATE SET
  features_json = excluded.features_json, features_hash = excluded.features_hash,
  created_at = excluded.created_at, expires_at = excluded.expires_at, superseded_at = NULL;

-- name: SupersedeFeatures :execrows
UPDATE feature_cache SET superseded_at = sqlc.arg(now)
WHERE shaula_version != sqlc.arg(current_version) AND superseded_at IS NULL;

-- name: GetPredictions :one
SELECT predictions_json, schema_version, target_id, created_at FROM prediction_cache
WHERE features_hash = ? AND model_run_id = ?;

-- name: PutPredictions :exec
INSERT INTO prediction_cache (features_hash, model_run_id, schema_version, predictions_json, target_id, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (features_hash, model_run_id) DO UPDATE SET
  schema_version = excluded.schema_version, predictions_json = excluded.predictions_json;
