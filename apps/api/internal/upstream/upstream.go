package upstream

import (
	"context"
	"errors"
)

type Kind int

const (
	KindInternal Kind = iota
	KindNotFound
	KindInvalid
	KindPrecondition
	KindUnavailable
)

type Error struct {
	Kind    Kind
	Message string
}

func (e *Error) Error() string { return e.Message }

func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

type Resolved struct {
	Catalog     string
	CatalogID   string
	RA          float64
	Dec         float64
	DisplayName string
	Designation string
}

type ResolveRequest struct {
	Query   string
	RA      *float64
	Dec     *float64
	Mission string
}

type ExtractRequest struct {
	Target       string
	Mission      string
	SigmaClip    float64
	DownloadAll  bool
	Author       string
	Exptime      string
	UseTLS       bool
	MaskEclipses bool
}

type Progress struct {
	Stage    string
	Message  string
	Fraction *float64
}

type FeatureSet struct {
	Target         string
	Mission        string
	Rows           []map[string]any
	LibraryVersion string
}

type Shaula interface {
	Version(ctx context.Context) (string, error)
	Resolve(ctx context.Context, req ResolveRequest) (Resolved, error)
	Extract(ctx context.Context, req ExtractRequest, onProgress func(Progress)) (FeatureSet, error)
}

type Run struct {
	RunID         string  `json:"run_id"`
	CreatedAt     string  `json:"created_at"`
	SchemaVersion int     `json:"schema_version"`
	SelectedModel string  `json:"selected_model"`
	Threshold     float64 `json:"threshold"`
}

type PredictRequest struct {
	Rows   []map[string]any
	RunID  string
	StarID string
}

type Prediction struct {
	Rows          []map[string]any
	ModelRunID    string
	SchemaVersion int
}

type Fang interface {
	Version(ctx context.Context) (string, error)
	ListRuns(ctx context.Context) ([]Run, error)
	Predict(ctx context.Context, req PredictRequest) (Prediction, error)
}
