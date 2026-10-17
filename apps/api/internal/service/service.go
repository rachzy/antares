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

	"github.com/rachzy/antares/apps/api/internal/cache"
	"github.com/rachzy/antares/apps/api/internal/config"
	"github.com/rachzy/antares/apps/api/internal/hub"
	"github.com/rachzy/antares/apps/api/internal/jobs"
	"github.com/rachzy/antares/apps/api/internal/store"
	"github.com/rachzy/antares/apps/api/internal/upstream"
)

type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindUnprocessable
	KindUnavailable
	KindBusy
)

type Error struct {
	Kind    Kind
	Message string
}

func (e *Error) Error() string { return e.Message }

func errf(kind Kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

const (
	MaxQueryLength  = 200
	MaxPredictRows  = 100
	maxStarIDLength = 200

	resolveTimeout  = 60 * time.Second
	predictTimeout  = 30 * time.Second
	listRunsTimeout = 10 * time.Second
)

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

func (s *Service) RefreshVersions(ctx context.Context) error {
	var errs []error

	if version, err := s.Shaula.Version(ctx); err != nil {
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

	if version, err := s.Fang.Version(ctx); err != nil {
		errs = append(errs, fmt.Errorf("fang: %w", err))
	} else {
		s.mu.Lock()
		s.fangVersion = version
		s.mu.Unlock()
	}
	return errors.Join(errs...)
}

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

func InfoOf(t store.Target) jobs.TargetInfo {
	return jobs.TargetInfo{
		Catalog: t.Catalog, CatalogID: t.CatalogID, DisplayName: t.DisplayName,
		Designation: t.Designation, RA: t.RA, Dec: t.Dec,
	}
}

type ResolveInput struct {
	Query   string
	Mission string
	RA      *float64
	Dec     *float64
}

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

func (s *Service) fromUpstream(what string, err error) error {
	var ue *upstream.Error
	if errors.As(err, &ue) {
		switch ue.Kind {
		case upstream.KindNotFound:
			return errf(KindNotFound, "%s", ue.Message)
		case upstream.KindInvalid:
			return errf(KindInvalid, "%s", ue.Message)
		case upstream.KindPrecondition:
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
