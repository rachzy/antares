-- name: UpsertTarget :one
INSERT INTO targets (catalog, catalog_id, ra_deg, dec_deg, display_name, designation, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (catalog, catalog_id) DO UPDATE SET designation = excluded.designation
RETURNING id, ra_deg, dec_deg, display_name;

-- name: UpsertAlias :exec
INSERT INTO target_aliases (alias, catalog, target_id) VALUES (?, ?, ?)
ON CONFLICT (alias, catalog) DO UPDATE SET target_id = excluded.target_id;

-- name: TargetByAlias :one
SELECT t.id, t.catalog, t.catalog_id, t.ra_deg, t.dec_deg, t.display_name, t.designation
FROM target_aliases a JOIN targets t ON t.id = a.target_id
WHERE a.alias = ? AND a.catalog = ?;

-- name: TargetByID :one
SELECT id, catalog, catalog_id, ra_deg, dec_deg, display_name, designation
FROM targets
WHERE id = ?;
