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

// PredictionEntry is one cached scoring of a feature set by one model run. TargetID is nil for rows
// posted to /ml/predict, which belong to no stored target.
type PredictionEntry struct {
	FeaturesHash  string
	ModelRunID    string
	SchemaVersion int
	Rows          []map[string]any
	TargetID      *int64
	CreatedAt     time.Time
}

// PredictionCache is the prediction_cache table, keyed by FeaturesHash and model run id. The run
// id, not Fang's library version, is the key: a run names one fixed set of trained artifacts, so it
// alone decides the scores.
type PredictionCache struct{ DB *sql.DB }

// Get returns the cached predictions for the key; the bool reports a hit and a miss is not an
// error.
func (c PredictionCache) Get(ctx context.Context, featuresHash, modelRunID string) (PredictionEntry, bool, error) {
	row, err := db.New(c.DB).GetPredictions(ctx, db.GetPredictionsParams{FeaturesHash: featuresHash, ModelRunID: modelRunID})
	if errors.Is(err, sql.ErrNoRows) {
		return PredictionEntry{}, false, nil
	}
	if err != nil {
		return PredictionEntry{}, false, err
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(row.PredictionsJson), &rows); err != nil {
		return PredictionEntry{}, false, fmt.Errorf("decode cached predictions: %w", err)
	}
	entry := PredictionEntry{FeaturesHash: featuresHash, ModelRunID: modelRunID, SchemaVersion: int(row.SchemaVersion), Rows: rows}
	if row.TargetID.Valid {
		id := row.TargetID.Int64
		entry.TargetID = &id
	}
	entry.CreatedAt, _ = store.ParseTime(row.CreatedAt)
	return entry, true, nil
}

// Put sanitizes and stores e; for an existing key it replaces the rows and schema version only.
func (c PredictionCache) Put(ctx context.Context, e PredictionEntry) error {
	body, err := json.Marshal(SanitizeRows(e.Rows))
	if err != nil {
		return err
	}
	var target sql.NullInt64
	if e.TargetID != nil {
		target = sql.NullInt64{Int64: *e.TargetID, Valid: true}
	}
	return db.New(c.DB).PutPredictions(ctx, db.PutPredictionsParams{
		FeaturesHash: e.FeaturesHash, ModelRunID: e.ModelRunID, SchemaVersion: int64(e.SchemaVersion),
		PredictionsJson: string(body), TargetID: target, CreatedAt: store.FormatTime(e.CreatedAt),
	})
}
