package service_test

import (
	"context"
	"errors"
	"testing"

	"antares/api/internal/catalog"
	"antares/api/internal/service"
)

type fakeCatalog struct {
	got     catalog.Query
	star    catalog.Star
	err     error
	located catalog.Located
	at      [2]float64
}

func (f *fakeCatalog) Locate(_ context.Context, ra, dec float64) (catalog.Located, error) {
	f.at = [2]float64{ra, dec}
	return f.located, f.err
}

func (f *fakeCatalog) Lookup(_ context.Context, q catalog.Query) (catalog.Star, error) {
	f.got = q
	return f.star, f.err
}

func TestStarInfoLooksUpTheResolvedCatalogueEntry(t *testing.T) {
	fc := &fakeCatalog{star: catalog.Star{SpectralType: "G0V"}}
	e := newEnv(t, nil)
	e.svc.Catalog = fc

	view, err := e.svc.StarInfo(context.Background(), service.ResolveInput{Query: "Kepler-186"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Star.SpectralType != "G0V" || view.Target.Catalog != fc.got.Catalog || view.Target.CatalogID != fc.got.ID || fc.got.ID == "" {
		t.Fatalf("view = %+v, looked up %+v", view, fc.got)
	}
}

func TestStarInfoMapsLookupFailures(t *testing.T) {
	e := newEnv(t, nil)
	e.svc.Catalog = &fakeCatalog{err: errors.New("simbad: connection refused 10.0.0.5")}
	_, err := e.svc.StarInfo(context.Background(), service.ResolveInput{Query: "Kepler-186"})
	if kindOf(t, err) != service.KindUnavailable || err.Error() != "star details are temporarily unavailable" {
		t.Fatalf("err = %v", err)
	}

	e.svc.Catalog = &fakeCatalog{err: catalog.ErrBadQuery}
	view, err := e.svc.StarInfo(context.Background(), service.ResolveInput{Query: "Kepler-186"})
	if err != nil || view.Star.Planets == nil || view.Target.DisplayName == "" {
		t.Fatalf("a catalogue entry the services cannot be asked about should give empty details: %+v, %v", view, err)
	}

	e.svc.Catalog = nil
	if _, err := e.svc.StarInfo(context.Background(), service.ResolveInput{Query: "Kepler-186"}); kindOf(t, err) != service.KindUnavailable {
		t.Fatalf("err = %v", err)
	}
}

func TestStarInfoByPositionNeverAsksShaula(t *testing.T) {
	fc := &fakeCatalog{located: catalog.Located{
		Catalog: "KIC", CatalogID: "8120608", Designation: "KIC 8120608", DisplayName: "Kepler-186",
		RA: 298.65, Dec: 43.95, Star: catalog.Star{SpectralType: "M1V"},
	}}
	e := newEnv(t, nil)
	e.svc.Catalog = fc
	e.svc.Shaula = nil

	ra, dec := 298.65, 43.95
	view, err := e.svc.StarInfo(context.Background(), service.ResolveInput{RA: &ra, Dec: &dec})
	if err != nil {
		t.Fatal(err)
	}
	if fc.at != [2]float64{ra, dec} || view.Star.SpectralType != "M1V" {
		t.Fatalf("located %v, view = %+v", fc.at, view)
	}
	if view.Target.Catalog != "KIC" || view.Target.CatalogID != "8120608" || view.Target.DisplayName != "Kepler-186" || view.Target.RA != 298.65 {
		t.Fatalf("target = %+v", view.Target)
	}
}

func TestStarInfoByPositionMapsErrors(t *testing.T) {
	e := newEnv(t, nil)
	ra, dec := 10.0, 10.0
	in := service.ResolveInput{RA: &ra, Dec: &dec}

	e.svc.Catalog = &fakeCatalog{err: catalog.ErrNotFound}
	if _, err := e.svc.StarInfo(context.Background(), in); kindOf(t, err) != service.KindNotFound {
		t.Fatalf("err = %v", err)
	}

	e.svc.Catalog = &fakeCatalog{err: errors.New("simbad: connection refused 10.0.0.5")}
	if _, err := e.svc.StarInfo(context.Background(), in); kindOf(t, err) != service.KindUnavailable || err.Error() != "star details are temporarily unavailable" {
		t.Fatalf("err = %v", err)
	}

	bad := 400.0
	if _, err := e.svc.StarInfo(context.Background(), service.ResolveInput{RA: &bad, Dec: &dec}); kindOf(t, err) != service.KindInvalid {
		t.Fatalf("err = %v", err)
	}
}
