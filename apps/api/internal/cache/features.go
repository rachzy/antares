package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"antares/api/internal/db"
	"antares/api/internal/store"
)

// FeatureEntry is one cached extraction: the feature rows Shaula produced for a target under one
// set of parameters (ParamsHash is ExtractParams.Hash) and one Shaula version. Put computes
// FeaturesHash and ignores the field; a nil ExpiresAt never expires.
type FeatureEntry struct {
	TargetID      int64
	ParamsHash    string
	ShaulaVersion string
	Rows          []map[string]any
	FeaturesHash  string
	CreatedAt     time.Time
	ExpiresAt     *time.Time
}

// FeatureCache is the feature_cache table, keyed by target, parameters hash and Shaula version, so
// a new Shaula release misses rather than serving older features.
type FeatureCache struct{ DB *sql.DB }

// Get returns the entry for the key unless it is superseded or expired at now. The bool reports a
// hit; a miss is not an error.
func (c FeatureCache) Get(ctx context.Context, targetID int64, paramsHash, shaulaVersion string, now time.Time) (FeatureEntry, bool, error) {
	row, err := db.New(c.DB).GetFeatures(ctx, db.GetFeaturesParams{
		TargetID: targetID, ParamsHash: paramsHash, ShaulaVersion: shaulaVersion, Now: store.NullTime(now),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return FeatureEntry{}, false, nil
	}
	if err != nil {
		return FeatureEntry{}, false, err
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(row.FeaturesJson), &rows); err != nil {
		return FeatureEntry{}, false, fmt.Errorf("decode cached features: %w", err)
	}
	entry := FeatureEntry{
		TargetID: targetID, ParamsHash: paramsHash, ShaulaVersion: shaulaVersion,
		Rows: rows, FeaturesHash: row.FeaturesHash,
	}
	entry.CreatedAt, _ = store.ParseTime(row.CreatedAt)
	if row.ExpiresAt.Valid {
		if t, err := store.ParseTime(row.ExpiresAt.String); err == nil {
			entry.ExpiresAt = &t
		}
	}
	return entry, true, nil
}

// Put sanitizes e.Rows, computes their hash and inserts or replaces the entry, clearing any earlier
// supersession.
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
	var expires sql.NullString
	if e.ExpiresAt != nil {
		expires = store.NullTime(*e.ExpiresAt)
	}
	return db.New(c.DB).PutFeatures(ctx, db.PutFeaturesParams{
		TargetID: e.TargetID, ParamsHash: e.ParamsHash, ShaulaVersion: e.ShaulaVersion,
		FeaturesJson: string(body), FeaturesHash: hash, CreatedAt: store.FormatTime(e.CreatedAt), ExpiresAt: expires,
	})
}

// Supersede marks every live entry from a Shaula version other than currentVersion as superseded,
// so Get never serves it again, and returns how many it marked.
func (c FeatureCache) Supersede(ctx context.Context, currentVersion string, now time.Time) (int64, error) {
	return db.New(c.DB).SupersedeFeatures(ctx, db.SupersedeFeaturesParams{
		Now: store.NullTime(now), CurrentVersion: currentVersion,
	})
}
