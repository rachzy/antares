package catalog_test

import (
	"context"
	"os"
	"testing"

	"antares/api/internal/catalog"
)

// The fake servers cannot parse ADQL, so this checks the real queries against SIMBAD and the NASA
// Exoplanet Archive. It runs only with ANTARES_LIVE_TESTS=1 because it needs the network.
func TestLiveQueriesAreAcceptedByTheServices(t *testing.T) {
	if os.Getenv("ANTARES_LIVE_TESTS") != "1" {
		t.Skip("set ANTARES_LIVE_TESTS=1 to query SIMBAD and the NASA Exoplanet Archive")
	}
	c := catalog.NewClient("", "")

	got, err := c.Locate(context.Background(), 298.6527, 43.9550) // Kepler-186
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got.Catalog != "KIC" || got.CatalogID != "8120608" || len(got.Star.Planets) == 0 {
		t.Errorf("located = %+v", got)
	}

	star, err := c.Lookup(context.Background(), catalog.Query{Catalog: "KIC", ID: "8120608"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(star.Sources) != 2 {
		t.Errorf("sources = %v, want both services to answer", star.Sources)
	}
}
