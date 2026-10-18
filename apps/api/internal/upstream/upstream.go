// Package upstream is the API's client for the private Shaula (feature extraction) and Fang
// (classification) gRPC services. Their failures surface as an *Error with a Kind, or as the
// caller's own context error.
package upstream

import (
	"context"
	"errors"
)

// Kind classifies an upstream failure independently of gRPC status codes.
type Kind int

const (
	// KindInternal is any unexpected failure.
	KindInternal Kind = iota
	// KindNotFound means the target or model run does not exist.
	KindNotFound
	// KindInvalid means the service rejected the request's arguments.
	KindInvalid
	// KindPrecondition means the request is valid but the service cannot serve it in its current
	// state.
	KindPrecondition
	// KindUnavailable means the service is down, overloaded or timed out on its side; retrying may
	// help.
	KindUnavailable
)

// Error is an upstream failure; Message is the service's own message.
type Error struct {
	Kind    Kind
	Message string
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

// KindOf returns err's Kind, or KindInternal if err does not wrap an *Error.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

// Resolved is the star Shaula resolved a query or position to; RA and Dec are ICRS degrees. Pass
// Designation to Extract, so the star extracted is the star resolved.
type Resolved struct {
	Catalog     string
	CatalogID   string
	RA          float64
	Dec         float64
	DisplayName string
	Designation string
}

// ResolveRequest names a star by Query, or by both RA and Dec (ICRS degrees), never both. The
// coordinates are pointers because 0 is a valid position.
type ResolveRequest struct {
	Query   string
	RA      *float64
	Dec     *float64
	Mission string
}

// ExtractRequest mirrors Shaula's ExtractFeaturesRequest; Target should be a Resolved.Designation.
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

// Progress is a progress event of an extraction; Fraction is nil when Shaula reports none.
type Progress struct {
	Stage    string
	Message  string
	Fraction *float64
}

// FeatureSet is an extraction's result: the feature rows and the Shaula library version that
// produced them.
type FeatureSet struct {
	Target         string
	Mission        string
	Rows           []map[string]any
	LibraryVersion string
}

// Shaula is the feature-extraction service. Extract calls onProgress on the calling goroutine, once
// per progress event, before it returns.
type Shaula interface {
	Version(ctx context.Context) (string, error)
	Resolve(ctx context.Context, req ResolveRequest) (Resolved, error)
	Extract(ctx context.Context, req ExtractRequest, onProgress func(Progress)) (FeatureSet, error)
}

// Run is one trained model run Fang can serve.
type Run struct {
	RunID         string  `json:"run_id"`
	CreatedAt     string  `json:"created_at"`
	SchemaVersion int     `json:"schema_version"`
	SelectedModel string  `json:"selected_model"`
	Threshold     float64 `json:"threshold"`
}

// PredictRequest asks Fang to score feature rows. An empty RunID selects the newest run; StarID is
// copied into every scored row.
type PredictRequest struct {
	Rows   []map[string]any
	RunID  string
	StarID string
}

// Prediction is the scored rows and the run that scored them.
type Prediction struct {
	Rows          []map[string]any
	ModelRunID    string
	SchemaVersion int
}

// Fang is the classification service and its registry of trained runs.
type Fang interface {
	Version(ctx context.Context) (string, error)
	ListRuns(ctx context.Context) ([]Run, error)
	Predict(ctx context.Context, req PredictRequest) (Prediction, error)
}
