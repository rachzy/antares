package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rachzy/antares/apps/api/internal/store"
)

type FeatureEntry struct {
	TargetID      int64
	ParamsHash    string
	ShaulaVersion string
	Rows          []map[string]any
	FeaturesHash  string
	CreatedAt     time.Time
	ExpiresAt     *time.Time
}

type FeatureCache struct{ DB *sql.DB }

func (c FeatureCache) Get(ctx context.Context, targetID int64, paramsHash, shaulaVersion string, now time.Time) (FeatureEntry, bool, error) {
	var (
		body, hash, created string
		expires             sql.NullString
	)
	err := c.DB.QueryRowContext(ctx, `
		SELECT features_json, features_hash, created_at, expires_at FROM feature_cache
		WHERE target_id = ? AND params_hash = ? AND shaula_version = ?
		  AND superseded_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`,
		targetID, paramsHash, shaulaVersion, store.FormatTime(now),
	).Scan(&body, &hash, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return FeatureEntry{}, false, nil
	}
	if err != nil {
		return FeatureEntry{}, false, err
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(body), &rows); err != nil {
		return FeatureEntry{}, false, fmt.Errorf("decode cached features: %w", err)
	}
	entry := FeatureEntry{
		TargetID: targetID, ParamsHash: paramsHash, ShaulaVersion: shaulaVersion,
		Rows: rows, FeaturesHash: hash,
	}
	entry.CreatedAt, _ = store.ParseTime(created)
	if expires.Valid {
		if t, err := store.ParseTime(expires.String); err == nil {
			entry.ExpiresAt = &t
		}
	}
	return entry, true, nil
}

func (c FeatureCache) Put(ctx context.Context, e FeatureEntry) error {
	rows := SanitizeRows(e.Rows)
	body, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	hash, err := FeaturesHash(rows)
	if err != nil {
		return err
	}
	var expires any
	if e.ExpiresAt != nil {
		expires = store.FormatTime(*e.ExpiresAt)
	}
	_, err = c.DB.ExecContext(ctx, `
		INSERT INTO feature_cache
		  (target_id, params_hash, shaula_version, features_json, features_hash, created_at, expires_at, superseded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
		ON CONFLICT (target_id, params_hash, shaula_version) DO UPDATE SET
		  features_json = excluded.features_json, features_hash = excluded.features_hash,
		  created_at = excluded.created_at, expires_at = excluded.expires_at, superseded_at = NULL`,
		e.TargetID, e.ParamsHash, e.ShaulaVersion, string(body), hash, store.FormatTime(e.CreatedAt), expires)
	return err
}

func (c FeatureCache) Supersede(ctx context.Context, currentVersion string, now time.Time) (int64, error) {
	res, err := c.DB.ExecContext(ctx,
		`UPDATE feature_cache SET superseded_at = ? WHERE shaula_version != ? AND superseded_at IS NULL`,
		store.FormatTime(now), currentVersion)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
