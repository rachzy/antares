package service_test

import (
	"context"
	"errors"
	"testing"

	"antares/api/internal/catalog"
	"antares/api/internal/service"
)

type fakeCatalog struct {
	got  catalog.Query
	star catalog.Star
	err  error
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
