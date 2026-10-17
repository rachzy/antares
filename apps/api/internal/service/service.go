// Package service implements the API's operations on top of the stores, caches, scheduler and
// upstream clients. Failures are *Error values whose Kind the transport maps to a status; every
// message is written for clients except KindInternal ones, which are only logged.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"antares/api/internal/cache"
	"antares/api/internal/config"
	"antares/api/internal/hub"
	"antares/api/internal/jobs"
	"antares/api/internal/store"
	"antares/api/internal/upstream"
)

// Kind classifies a service failure for the transport layer.
type Kind int

const (
	// KindInternal is an unexpected failure; its message may hold internal detail and is never
	// shown.
	KindInternal Kind = iota
	// KindInvalid is bad client input (400).
	KindInvalid
	// KindNotFound is an unknown target, model run or subscription (404).
	KindNotFound
	// KindUnprocessable is valid input the pipeline cannot process (422).
	KindUnprocessable
	// KindUnavailable is an upstream service that is down, slow or not ready (503).
	KindUnavailable
	// KindBusy is a full job queue (503 with Retry-After).
	KindBusy
)

// Error is a service failure with a Kind and a message.
type Error struct {
	Kind    Kind
	Message string
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

func errf(kind Kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// MaxQueryLength (in characters) and MaxPredictRows bound client input; the timeouts bound each
// upstream call.
const (
	MaxQueryLength  = 200
	MaxPredictRows  = 100
	maxStarIDLength = 200

	resolveTimeout  = 60 * time.Second
	predictTimeout  = 30 * time.Second
	listRunsTimeout = 10 * time.Second
)

// Service is the API's application layer. Set every field except Log and VersionTimeout before use
// and do not change them afterwards; the methods are safe for concurrent use.
type Service struct {
	Targets     store.Targets
	Features    cache.FeatureCache
	Predictions cache.PredictionCache
	Jobs        jobs.Store
	Hub         *hub.Hub
	Sched       *jobs.Scheduler
	Shaula      upstream.Shaula
	Fang        upstream.Fang
	Clock       func() time.Time
	Cfg         config.Config
	Log         *slog.Logger
	// VersionTimeout bounds each upstream Version call separately when positive.
	VersionTimeout time.Duration

	mu            sync.RWMutex
	shaulaVersion string
	fangVersion   string
}

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func (s *Service) versionOf(ctx context.Context, get func(context.Context) (string, error)) (string, error) {
	if s.VersionTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.VersionTimeout)
		defer cancel()
	}
	return get(ctx)
}

// RefreshVersions asks Shaula and Fang for their library versions and caches them for Versions.
// When Shaula's version changes, cached features from other versions are superseded. A failing
// service keeps its last known version, is reported in the joined error and does not stop the
// other's refresh.
func (s *Service) RefreshVersions(ctx context.Context) error {
	var errs []error

	if version, err := s.versionOf(ctx, s.Shaula.Version); err != nil {
		errs = append(errs, fmt.Errorf("shaula: %w", err))
	} else {
		s.mu.Lock()
		changed := version != s.shaulaVersion
		s.shaulaVersion = version
		s.mu.Unlock()
		if changed {
			if n, err := s.Features.Supersede(ctx, version, s.Clock()); err != nil {
				errs = append(errs, fmt.Errorf("supersede features: %w", err))
			} else if n > 0 {
				s.log().Info("superseded cached features after a version change", "version", version, "entries", n)
			}
		}
	}

	if version, err := s.versionOf(ctx, s.Fang.Version); err != nil {
		errs = append(errs, fmt.Errorf("fang: %w", err))
	} else {
		s.mu.Lock()
		s.fangVersion = version
		s.mu.Unlock()
	}
	return errors.Join(errs...)
}

// Versions returns the last known library versions; empty means not reached yet.
func (s *Service) Versions() (shaula, fang string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shaulaVersion, s.fangVersion
}

func (s *Service) knownShaulaVersion() (string, bool) {
	shaula, _ := s.Versions()
	return shaula, shaula != ""
}

var missions = map[string]struct{ name, catalog string }{
	"kepler": {"Kepler", "KIC"},
	"k2":     {"K2", "EPIC"},
	"tess":   {"TESS", "TIC"},
}

func canonicalMission(raw string) (name, catalog string, err error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		key = "kepler"
	}
	m, ok := missions[key]
	if !ok {
		return "", "", errf(KindInvalid, "unsupported mission %q; expected Kepler, K2 or TESS", raw)
	}
	return m.name, m.catalog, nil
}

// InfoOf returns the public description of t.
func InfoOf(t store.Target) jobs.TargetInfo {
	return jobs.TargetInfo{
		Catalog: t.Catalog, CatalogID: t.CatalogID, DisplayName: t.DisplayName,
		Designation: t.Designation, RA: t.RA, Dec: t.Dec,
	}
}

// ResolveInput names a star by Query, or by both RA and Dec (ICRS degrees). Mission defaults to
// Kepler.
type ResolveInput struct {
	Query   string
	Mission string
	RA      *float64
	Dec     *float64
}

// Resolve finds the star a query or position names in the mission's catalogue. A query first tries
// the stored aliases; otherwise Shaula resolves it and the target is stored, with the query as a
// new alias.
func (s *Service) Resolve(ctx context.Context, in ResolveInput) (store.Target, error) {
	mission, catalog, err := canonicalMission(in.Mission)
	if err != nil {
		return store.Target{}, err
	}

	query := strings.TrimSpace(in.Query)
	if query != "" && (in.RA != nil || in.Dec != nil) {
		return store.Target{}, errf(KindInvalid, `send either "query" or "ra_deg" and "dec_deg", not both`)
	}
	if query == "" {
		return s.resolveByPosition(ctx, in, mission)
	}
	if utf8.RuneCountInString(query) > MaxQueryLength {
		return store.Target{}, errf(KindInvalid, "query is longer than %d characters", MaxQueryLength)
	}

	if target, ok, err := s.Targets.ByAlias(ctx, query, catalog); err != nil {
		return store.Target{}, errf(KindInternal, "alias lookup: %v", err)
	} else if ok {
		return target, nil
	}

	rctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()
	resolved, err := s.Shaula.Resolve(rctx, upstream.ResolveRequest{Query: query, Mission: mission})
	if err != nil {
		return store.Target{}, s.fromUpstream("pipeline", err)
	}
	target, err := s.Targets.Upsert(ctx, store.Target{
		Catalog: resolved.Catalog, CatalogID: resolved.CatalogID,
		RA: resolved.RA, Dec: resolved.Dec,
		DisplayName: resolved.DisplayName, Designation: resolved.Designation,
	}, s.Clock(), query)
	if err != nil {
		return store.Target{}, errf(KindInternal, "store target: %v", err)
	}
	return target, nil
}

func (s *Service) resolveByPosition(ctx context.Context, in ResolveInput, mission string) (store.Target, error) {
	if in.RA == nil || in.Dec == nil {
		return store.Target{}, errf(KindInvalid, `provide a name or designation in "query", or both "ra_deg" and "dec_deg"`)
	}
	ra, dec := *in.RA, *in.Dec
	if math.IsNaN(ra) || math.IsInf(ra, 0) || math.IsNaN(dec) || math.IsInf(dec, 0) ||
		ra < 0 || ra >= 360 || dec < -90 || dec > 90 {
		return store.Target{}, errf(KindInvalid, "ra_deg must be in [0, 360) and dec_deg in [-90, 90]")
	}

	rctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()
	resolved, err := s.Shaula.Resolve(rctx, upstream.ResolveRequest{RA: in.RA, Dec: in.Dec, Mission: mission})
	if err != nil {
		return store.Target{}, s.fromUpstream("pipeline", err)
	}
	target, err := s.Targets.Upsert(ctx, store.Target{
		Catalog: resolved.Catalog, CatalogID: resolved.CatalogID,
		RA: resolved.RA, Dec: resolved.Dec,
		DisplayName: resolved.DisplayName, Designation: resolved.Designation,
	}, s.Clock())
	if err != nil {
		return store.Target{}, errf(KindInternal, "store target: %v", err)
	}
	return target, nil
}

// fromUpstream maps an upstream error to an *Error for the operation what ("pipeline" or "model").
// Client-caused kinds keep the service's message; an unavailable or silent service gets a generic
// one, with details logged. FailedPrecondition from the model means no servable model (503); from
// the pipeline it means the star cannot be processed (422).
func (s *Service) fromUpstream(what string, err error) error {
	var ue *upstream.Error
	if errors.As(err, &ue) {
		switch ue.Kind {
		case upstream.KindNotFound:
			return errf(KindNotFound, "%s", ue.Message)
		case upstream.KindInvalid:
			return errf(KindInvalid, "%s", ue.Message)
		case upstream.KindPrecondition:
			if what == "model" {
				return errf(KindUnavailable, "%s", ue.Message)
			}
			return errf(KindUnprocessable, "%s", ue.Message)
		case upstream.KindUnavailable:
			s.log().Warn("upstream unavailable", "service", what, "error", ue.Message)
			return errf(KindUnavailable, "the %s service is unavailable; try again later", what)
		}
		return errf(KindInternal, "%s service: %s", what, ue.Message)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errf(KindUnavailable, "the %s service did not answer in time", what)
	}
	s.log().Warn("upstream unavailable", "service", what, "error", err)
	return errf(KindUnavailable, "the %s service is unavailable; try again later", what)
}
