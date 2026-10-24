package catalog_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"antares/api/internal/catalog"
)

// simbadServer answers the three SIMBAD queries the client sends, keyed by the table they read.
func simbadServer(t *testing.T, ident, aliases, flux string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.PostForm.Get("QUERY")
		switch {
		case strings.Contains(q, "JOIN ident"):
			_, _ = w.Write([]byte(ident))
		case strings.Contains(q, "FROM ident"):
			_, _ = w.Write([]byte(aliases))
		case strings.Contains(q, "FROM flux"):
			_, _ = w.Write([]byte(flux))
		default:
			http.Error(w, "unexpected query "+q, http.StatusBadRequest)
		}
	}))
}

func nasaServer(t *testing.T, byTable map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.PostForm.Get("query")
		for table, body := range byTable {
			if strings.Contains(q, "FROM "+table+" ") {
				_, _ = w.Write([]byte(body))
				return
			}
		}
		http.Error(w, "unexpected query "+q, http.StatusBadRequest)
	}))
}

const kepler90Ident = `{"data": [[1234, "G0V", "*", 1.2]]}`
const kepler90Aliases = `{"data": [["KIC 11442793"], ["* Kepler-90"], ["KOI-351"], ["HD 175692"], ["Gaia DR3 2134"], ["2MASS J1"]]}`
const kepler90Flux = `{"data": [["V", 14.1], ["G", 13.9]]}`
const kepler90KOI = `[
 {"kepoi_name":"K00351.02","kepler_name":"Kepler-90 c","koi_disposition":"CONFIRMED","koi_period":8.72,"koi_prad":1.2,"koi_steff":6080.0,"koi_srad":1.2,"koi_kepmag":13.8},
 {"kepoi_name":"K00351.01","kepler_name":"Kepler-90 h","koi_disposition":"CONFIRMED","koi_period":331.6,"koi_prad":11.3,"koi_steff":6080.0,"koi_srad":1.2,"koi_kepmag":13.8},
 {"kepoi_name":"K00351.99","kepler_name":null,"koi_disposition":"FALSE POSITIVE","koi_period":3.0,"koi_prad":1.0,"koi_steff":6080.0,"koi_srad":1.2,"koi_kepmag":13.8},
 {"kepoi_name":"K00351.07","kepler_name":"Kepler-90 b","koi_disposition":"CONFIRMED","koi_period":7.01,"koi_prad":1.3,"koi_steff":6080.0,"koi_srad":1.2,"koi_kepmag":13.8}
]`

func TestLookupKepler90(t *testing.T) {
	simbad := simbadServer(t, kepler90Ident, kepler90Aliases, kepler90Flux)
	defer simbad.Close()
	nasa := nasaServer(t, map[string]string{
		"cumulative": kepler90KOI,
		"pscomppars": `[{"st_mass": 1.1, "sy_dist": 255.0}]`,
	})
	defer nasa.Close()

	star, err := catalog.NewClient(simbad.URL, nasa.URL).Lookup(context.Background(), catalog.Query{Catalog: "KIC", ID: "11442793"})
	if err != nil {
		t.Fatal(err)
	}
	if star.SpectralType != "G0V" || star.ObjectType != "*" {
		t.Errorf("spectral/object type = %q/%q", star.SpectralType, star.ObjectType)
	}
	if got := strings.Join(star.Aliases, ","); got != "Kepler-90,KOI-351,HD 175692" {
		t.Errorf("aliases = %s", got)
	}
	if star.DistanceLY == nil || *star.DistanceLY < 2700 || *star.DistanceLY > 2800 { // 1000/1.2 pc
		t.Errorf("distance = %v", star.DistanceLY)
	}
	if star.Magnitude == nil || *star.Magnitude != 13.9 || star.MagnitudeBand != "G" {
		t.Errorf("magnitude = %v %q", star.Magnitude, star.MagnitudeBand)
	}
	if star.TeffK == nil || *star.TeffK != 6080 || star.RadiusSolar == nil || *star.RadiusSolar != 1.2 || star.MassSolar == nil || *star.MassSolar != 1.1 {
		t.Errorf("stellar parameters = %v %v %v", star.TeffK, star.RadiusSolar, star.MassSolar)
	}
	var names []string
	for _, p := range star.Planets {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "Kepler-90 b,Kepler-90 c,Kepler-90 h" {
		t.Errorf("planets = %s (false positives must be dropped, the rest sorted by period)", got)
	}
	if got := strings.Join(star.Sources, ","); got != "SIMBAD,NASA Exoplanet Archive" {
		t.Errorf("sources = %s", got)
	}
}

func TestLookupTICUsesTheConfirmedPlanetTable(t *testing.T) {
	simbad := simbadServer(t, `{"data": []}`, `{"data": []}`, `{"data": []}`)
	defer simbad.Close()
	nasa := nasaServer(t, map[string]string{
		"pscomppars": `[{"pl_name":"TOI-700 d","pl_orbper":37.4,"pl_rade":1.07,"st_teff":3480,"st_rad":0.42,"st_mass":0.42,"sy_dist":31.1}]`,
	})
	defer nasa.Close()

	star, err := catalog.NewClient(simbad.URL, nasa.URL).Lookup(context.Background(), catalog.Query{Catalog: "TIC", ID: "150428135"})
	if err != nil {
		t.Fatal(err)
	}
	if len(star.Planets) != 1 || star.Planets[0].Name != "TOI-700 d" || star.Planets[0].Confirmation != "confirmed" {
		t.Errorf("planets = %+v", star.Planets)
	}
	if star.DistanceLY == nil || *star.DistanceLY < 100 || *star.DistanceLY > 103 {
		t.Errorf("distance = %v", star.DistanceLY)
	}
	if got := strings.Join(star.Sources, ","); got != "NASA Exoplanet Archive" {
		t.Errorf("sources = %s", got)
	}
}

func TestLookupSurvivesOneServiceFailing(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer down.Close()
	nasa := nasaServer(t, map[string]string{"cumulative": kepler90KOI, "pscomppars": `[]`})
	defer nasa.Close()

	star, err := catalog.NewClient(down.URL, nasa.URL).Lookup(context.Background(), catalog.Query{Catalog: "KIC", ID: "11442793"})
	if err != nil {
		t.Fatal(err)
	}
	if len(star.Planets) != 3 || star.SpectralType != "" {
		t.Errorf("star = %+v", star)
	}
}

func TestLookupFailsOnlyWhenBothServicesFail(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer down.Close()
	if _, err := catalog.NewClient(down.URL, down.URL).Lookup(context.Background(), catalog.Query{Catalog: "KIC", ID: "1"}); err == nil {
		t.Fatal("want an error")
	}
}

func TestLookupOfAnUnknownStarIsEmptyNotAnError(t *testing.T) {
	simbad := simbadServer(t, `{"data": []}`, `{"data": []}`, `{"data": []}`)
	defer simbad.Close()
	nasa := nasaServer(t, map[string]string{"cumulative": `[]`})
	defer nasa.Close()
	star, err := catalog.NewClient(simbad.URL, nasa.URL).Lookup(context.Background(), catalog.Query{Catalog: "KIC", ID: "1"})
	if err != nil || len(star.Sources) != 0 || star.Aliases == nil || star.Planets == nil {
		t.Fatalf("star = %+v, err = %v", star, err)
	}
}

func TestLookupRejectsAnythingThatIsNotACatalogueEntry(t *testing.T) {
	c := catalog.NewClient("http://127.0.0.1:1", "http://127.0.0.1:1")
	for _, q := range []catalog.Query{
		{Catalog: "KIC", ID: "1' OR '1'='1"}, {Catalog: "KIC", ID: ""}, {Catalog: "Kepler", ID: "1"}, {Catalog: "KIC", ID: "-5"},
	} {
		if _, err := c.Lookup(context.Background(), q); !errors.Is(err, catalog.ErrBadQuery) {
			t.Errorf("%+v: err = %v", q, err)
		}
	}
}

type counting struct{ calls atomic.Int32 }

func (c *counting) Lookup(context.Context, catalog.Query) (catalog.Star, error) {
	if c.calls.Add(1) == 1 {
		return catalog.Star{}, errors.New("down")
	}
	return catalog.Star{SpectralType: "G0V"}, nil
}

func TestCachedRemembersSuccessesUntilTheyExpire(t *testing.T) {
	src := &counting{}
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	c := &catalog.Cached{Source: src, TTL: time.Hour, Clock: func() time.Time { return now }}
	q := catalog.Query{Catalog: "KIC", ID: "1"}

	if _, err := c.Lookup(context.Background(), q); err == nil {
		t.Fatal("first call should fail and not be cached")
	}
	for range 3 {
		if star, err := c.Lookup(context.Background(), q); err != nil || star.SpectralType != "G0V" {
			t.Fatalf("star = %+v, err = %v", star, err)
		}
	}
	if got := src.calls.Load(); got != 2 {
		t.Fatalf("source calls = %d, want 2", got)
	}
	now = now.Add(2 * time.Hour)
	_, _ = c.Lookup(context.Background(), q)
	if got := src.calls.Load(); got != 3 {
		t.Fatalf("source calls after expiry = %d, want 3", got)
	}
}

// locateServer answers the cone search and the follow-up queries of a position lookup.
func locateServer(t *testing.T, cone, aliases string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.PostForm.Get("QUERY")
		switch {
		case strings.Contains(q, "CONTAINS"):
			// SIMBAD rejects an expression after ORDER BY, so the distance must be a selected alias.
			if strings.Contains(q, "ORDER BY DISTANCE") || !strings.Contains(q, "AS sep") || !strings.Contains(q, "ORDER BY sep") {
				http.Error(w, "bad ADQL "+q, http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(cone))
		case strings.Contains(q, "FROM ident"):
			_, _ = w.Write([]byte(aliases))
		case strings.Contains(q, "FROM flux"):
			_, _ = w.Write([]byte(kepler90Flux))
		default:
			http.Error(w, "unexpected query "+q, http.StatusBadRequest)
		}
	}))
}

func TestLocateFindsTheNearestStarAndItsMissionEntry(t *testing.T) {
	simbad := locateServer(t, `{"data": [[1234, "* Kepler-90", 284.43, 49.30, "G0V", "*", 1.2]]}`, `{"data": [["KIC 11442793"], ["KOI-351"], ["* Kepler-90"]]}`)
	defer simbad.Close()
	nasa := nasaServer(t, map[string]string{"cumulative": kepler90KOI, "pscomppars": `[]`})
	defer nasa.Close()

	got, err := catalog.NewClient(simbad.URL, nasa.URL).Locate(context.Background(), 284.43, 49.30)
	if err != nil {
		t.Fatal(err)
	}
	if got.Catalog != "KIC" || got.CatalogID != "11442793" || got.Designation != "KIC 11442793" || got.DisplayName != "Kepler-90" {
		t.Errorf("located = %+v", got)
	}
	if got.RA != 284.43 || got.Dec != 49.30 || got.Star.SpectralType != "G0V" || len(got.Star.Planets) != 3 {
		t.Errorf("located = %+v", got)
	}
}

func TestLocateWorksForAStarWithoutAMissionEntry(t *testing.T) {
	simbad := locateServer(t, `{"data": [[7, "* 16 Cyg B", 295.46, 50.51, "G3V", "*", 47.4]]}`, `{"data": [["* 16 Cyg B"], ["HD 186427"]]}`)
	defer simbad.Close()
	nasa := nasaServer(t, nil)
	defer nasa.Close()

	got, err := catalog.NewClient(simbad.URL, nasa.URL).Locate(context.Background(), 295.46, 50.51)
	if err != nil {
		t.Fatal(err)
	}
	if got.Catalog != "" || got.CatalogID != "" || got.Designation != "HD 186427" || got.DisplayName != "HD 186427" {
		t.Errorf("located = %+v", got)
	}
	if got.Star.SpectralType != "G3V" || len(got.Star.Planets) != 0 {
		t.Errorf("star = %+v", got.Star)
	}
}

func TestLocateReportsNothingNearThePosition(t *testing.T) {
	simbad := locateServer(t, `{"data": []}`, `{"data": []}`)
	defer simbad.Close()
	if _, err := catalog.NewClient(simbad.URL, simbad.URL).Locate(context.Background(), 10, 10); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestLocateFailsWhenSimbadIsDown(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer down.Close()
	_, err := catalog.NewClient(down.URL, down.URL).Locate(context.Background(), 10, 10)
	if err == nil || errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func (c *counting) Locate(context.Context, float64, float64) (catalog.Located, error) {
	return catalog.Located{}, catalog.ErrNotFound
}

func TestLocateNamesAStarByItsCommonNameAndKeepsTheDesignation(t *testing.T) {
	simbad := locateServer(t, `{"data": [[9, "* alf Sco", 247.35, -26.43, "M1Ib", "s*r", 5.9]]}`,
		`{"data": [["* alf Sco"], ["HIP 80763"], ["HD 148478"], ["NAME Antares"]]}`)
	defer simbad.Close()
	nasa := nasaServer(t, nil)
	defer nasa.Close()

	got, err := catalog.NewClient(simbad.URL, nasa.URL).Locate(context.Background(), 247.35, -26.43)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Antares" || got.Designation != "HIP 80763" {
		t.Errorf("display %q, designation %q", got.DisplayName, got.Designation)
	}
}
