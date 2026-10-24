package service

import (
	"context"
	"errors"
	"strings"
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

// StarInfo resolves a name through Shaula and looks the star up in SIMBAD and the NASA Exoplanet
// Archive. A position skips Shaula: SIMBAD finds the nearest star, so any star in the sky has
// details, mission target or not. A star they do not know comes back with no details; both
// services being down is KindUnavailable.
func (s *Service) StarInfo(ctx context.Context, in ResolveInput) (InfoView, error) {
	if s.Catalog == nil {
		return InfoView{}, errf(KindUnavailable, "star details are not configured")
	}
	if strings.TrimSpace(in.Query) == "" && in.RA != nil && in.Dec != nil {
		return s.starInfoAt(ctx, *in.RA, *in.Dec)
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

func (s *Service) starInfoAt(ctx context.Context, ra, dec float64) (InfoView, error) {
	if err := validPosition(ra, dec); err != nil {
		return InfoView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, infoTimeout)
	defer cancel()
	found, err := s.Catalog.Locate(ctx, ra, dec)
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		return InfoView{}, errf(KindNotFound, "no star found at that position")
	case err != nil:
		s.log().Warn("star position lookup failed", "error", err)
		return InfoView{}, errf(KindUnavailable, "star details are temporarily unavailable")
	}
	return InfoView{
		Target: jobs.TargetInfo{
			Catalog: found.Catalog, CatalogID: found.CatalogID, DisplayName: found.DisplayName,
			Designation: found.Designation, RA: found.RA, Dec: found.Dec,
		},
		Star: found.Star,
	}, nil
}
