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

type PredictionEntry struct {
	FeaturesHash  string
	ModelRunID    string
	SchemaVersion int
	Rows          []map[string]any
	TargetID      *int64
	CreatedAt     time.Time
}

type PredictionCache struct{ DB *sql.DB }

func (c PredictionCache) Get(ctx context.Context, featuresHash, modelRunID string) (PredictionEntry, bool, error) {
	var (
		body, created string
		schema        int
		target        sql.NullInt64
	)
	err := c.DB.QueryRowContext(ctx, `
		SELECT predictions_json, schema_version, target_id, created_at FROM prediction_cache
		WHERE features_hash = ? AND model_run_id = ?`, featuresHash, modelRunID,
	).Scan(&body, &schema, &target, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return PredictionEntry{}, false, nil
	}
	if err != nil {
		return PredictionEntry{}, false, err
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(body), &rows); err != nil {
		return PredictionEntry{}, false, fmt.Errorf("decode cached predictions: %w", err)
	}
	entry := PredictionEntry{FeaturesHash: featuresHash, ModelRunID: modelRunID, SchemaVersion: schema, Rows: rows}
	if target.Valid {
		id := target.Int64
		entry.TargetID = &id
	}
	entry.CreatedAt, _ = store.ParseTime(created)
	return entry, true, nil
}

func (c PredictionCache) Put(ctx context.Context, e PredictionEntry) error {
	body, err := json.Marshal(SanitizeRows(e.Rows))
	if err != nil {
		return err
	}
	var target any
	if e.TargetID != nil {
		target = *e.TargetID
	}
	_, err = c.DB.ExecContext(ctx, `
		INSERT INTO prediction_cache (features_hash, model_run_id, schema_version, predictions_json, target_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (features_hash, model_run_id) DO UPDATE SET
		  schema_version = excluded.schema_version, predictions_json = excluded.predictions_json`,
		e.FeaturesHash, e.ModelRunID, e.SchemaVersion, string(body), target, store.FormatTime(e.CreatedAt))
	return err
}
