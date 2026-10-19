package service

import (
	"context"
	"errors"
	"time"

	"antares/api/internal/catalog"
	"antares/api/internal/jobs"
)

const infoTimeout = 25 * time.Second

// InfoView is a resolved star with what the public catalogues say about it.
type InfoView struct {
	Target jobs.TargetInfo `json:"target"`
	Star   catalog.Star    `json:"star"`
}

// StarInfo resolves a name or position to a star and looks it up in SIMBAD and the NASA Exoplanet
// Archive. A star they do not know comes back with no details; both services being down is
// KindUnavailable.
func (s *Service) StarInfo(ctx context.Context, in ResolveInput) (InfoView, error) {
	if s.Catalog == nil {
		return InfoView{}, errf(KindUnavailable, "star details are not configured")
	}
	target, err := s.Resolve(ctx, in)
	if err != nil {
		return InfoView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, infoTimeout)
	defer cancel()
	star, err := s.Catalog.Lookup(ctx, catalog.Query{Catalog: target.Catalog, ID: target.CatalogID})
	switch {
	case errors.Is(err, catalog.ErrBadQuery):
		// A catalogue the services cannot be asked about, such as a name Shaula resolved oddly.
		star = catalog.Star{Aliases: []string{}, Planets: []catalog.Planet{}, Sources: []string{}}
	case err != nil:
		s.log().Warn("star details lookup failed", "catalog", target.Catalog, "id", target.CatalogID, "error", err)
		return InfoView{}, errf(KindUnavailable, "star details are temporarily unavailable")
	}
	return InfoView{Target: InfoOf(target), Star: star}, nil
}
