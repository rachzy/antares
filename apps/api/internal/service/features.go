package service

import (
	"context"
	"time"

	"antares/api/internal/jobs"
	"antares/api/internal/store"
)

// FeaturesView is the latest features extracted for a star, whatever parameters produced them.
type FeaturesView struct {
	Target        jobs.TargetInfo  `json:"target"`
	ShaulaVersion string           `json:"shaula_version"`
	CreatedAt     time.Time        `json:"created_at"`
	Features      []map[string]any `json:"features"`
}

// LatestFeatures returns the most recent features extracted for the star a name or position identifies. It
// never starts an extraction; a star with none is KindNotFound.
func (s *Service) LatestFeatures(ctx context.Context, in ResolveInput) (FeaturesView, error) {
	target, err := s.Resolve(ctx, in)
	if err != nil {
		return FeaturesView{}, err
	}
	return s.latestFor(ctx, target)
}

// LatestFeaturesByToken returns the most recent features extracted for the star of a subscription's
// job, which may be newer than that job's own result.
func (s *Service) LatestFeaturesByToken(ctx context.Context, token string) (FeaturesView, error) {
	job, err := s.jobByToken(ctx, token)
	if err != nil {
		return FeaturesView{}, err
	}
	target, ok, err := s.Targets.ByID(ctx, job.TargetID)
	if err != nil {
		return FeaturesView{}, errf(KindInternal, "load target: %v", err)
	}
	if !ok {
		return FeaturesView{}, errf(KindNotFound, "unknown or expired subscription")
	}
	return s.latestFor(ctx, target)
}

func (s *Service) latestFor(ctx context.Context, target store.Target) (FeaturesView, error) {
	entry, ok, err := s.Features.Latest(ctx, target.ID)
	if err != nil {
		return FeaturesView{}, errf(KindInternal, "latest features: %v", err)
	}
	if !ok {
		return FeaturesView{}, errf(KindNotFound, "no features have been extracted for %s yet; start one with /pipeline/extract", target.DisplayName)
	}
	return FeaturesView{
		Target: InfoOf(target), ShaulaVersion: entry.ShaulaVersion,
		CreatedAt: entry.CreatedAt, Features: entry.Rows,
	}, nil
}
