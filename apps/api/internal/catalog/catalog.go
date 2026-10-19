// Package catalog looks up descriptive data about a star in two public services: SIMBAD (names,
// spectral type, distance, magnitude) and the NASA Exoplanet Archive (stellar parameters and known
// planets). Both are queried over TAP/ADQL; a failing service only leaves its fields empty.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// Query identifies a star by its mission catalogue entry, e.g. Catalog "KIC" and ID "11442793".
type Query struct {
	Catalog string
	ID      string
}

// Planet is a known planet of the star.
type Planet struct {
	Name         string   `json:"name"`
	PeriodDays   *float64 `json:"period_days,omitempty"`
	RadiusEarth  *float64 `json:"radius_earth,omitempty"`
	Confirmation string   `json:"confirmation"`
}

// Star is what the catalogues say about a star. Every measurement is a pointer so an unknown
// value is omitted instead of reported as zero.
type Star struct {
	Aliases       []string `json:"aliases"`
	SpectralType  string   `json:"spectral_type,omitempty"`
	ObjectType    string   `json:"object_type,omitempty"`
	DistanceLY    *float64 `json:"distance_ly,omitempty"`
	TeffK         *float64 `json:"teff_k,omitempty"`
	RadiusSolar   *float64 `json:"radius_solar,omitempty"`
	MassSolar     *float64 `json:"mass_solar,omitempty"`
	Magnitude     *float64 `json:"magnitude,omitempty"`
	MagnitudeBand string   `json:"magnitude_band,omitempty"`
	Planets       []Planet `json:"planets"`
	// Sources lists the services that answered.
	Sources []string `json:"sources"`
}

// Source looks a star up. A star the services do not know yields an empty Star, not an error; an
// error means no service could be reached.
type Source interface {
	Lookup(ctx context.Context, q Query) (Star, error)
}

var (
	digits   = regexp.MustCompile(`^[0-9]{1,18}$`)
	hostName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 .+_-]{0,60}$`)
)

// ErrBadQuery is returned for a catalogue or ID that cannot be a mission catalogue entry. It also
// keeps the ADQL built from a Query free of injected text.
var ErrBadQuery = errors.New("catalog: invalid catalogue entry")

func (q Query) validate() error {
	switch q.Catalog {
	case "KIC", "EPIC", "TIC":
	default:
		return fmt.Errorf("%w: unknown catalogue %q", ErrBadQuery, q.Catalog)
	}
	if !digits.MatchString(q.ID) {
		return fmt.Errorf("%w: %q", ErrBadQuery, q.ID)
	}
	return nil
}

func (q Query) designation() string { return q.Catalog + " " + q.ID }
