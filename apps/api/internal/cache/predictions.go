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
