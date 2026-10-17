package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/rachzy/antares/apps/api/internal/store"
	"github.com/rachzy/antares/apps/api/internal/testdb"
)

var kepler186 = store.Target{
	Catalog: "KIC", CatalogID: "8120608",
	RA: 298.65272, Dec: 43.95502,
	DisplayName: "Kepler-186", Designation: "KIC 8120608",
}

func TestUpsertIsIdempotentPerCatalogID(t *testing.T) {
	ctx := context.Background()
	targets := store.Targets{DB: testdb.Open(t)}
	now := time.Now()

	first, err := targets.Upsert(ctx, kepler186, now, "Kepler-186f")
	if err != nil || first.ID == 0 {
		t.Fatalf("first upsert: %+v, %v", first, err)
	}
	drifted := kepler186
	drifted.RA += 0.0001
	drifted.DisplayName = "KIC 8120608"
	second, err := targets.Upsert(ctx, drifted, now, "kepler 186")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("id changed: %d then %d", first.ID, second.ID)
	}
	if second.RA != first.RA || second.DisplayName != "Kepler-186" {
		t.Fatalf("first-stored coordinates and name must be kept, got %+v", second)
	}
}

func TestByAliasNormalisesAndIsPerCatalog(t *testing.T) {
	ctx := context.Background()
	targets := store.Targets{DB: testdb.Open(t)}
	if _, err := targets.Upsert(ctx, kepler186, time.Now(), "Kepler-186f"); err != nil {
		t.Fatal(err)
	}

	for _, alias := range []string{"Kepler-186f", "kepler 186f", "KEPLER_186F", "Kepler-186", "KIC 8120608", "kic8120608"} {
		got, ok, err := targets.ByAlias(ctx, alias, "KIC")
		if err != nil || !ok || got.CatalogID != "8120608" {
			t.Errorf("alias %q: %+v ok=%v err=%v", alias, got, ok, err)
		}
	}
	if _, ok, _ := targets.ByAlias(ctx, "Kepler-186", "TIC"); ok {
		t.Error("an alias must not leak across catalogs")
	}
	if _, ok, _ := targets.ByAlias(ctx, "Kepler-999", "KIC"); ok {
		t.Error("unknown alias was found")
	}
}

func TestAliasClashLatestResolutionWins(t *testing.T) {
	ctx := context.Background()
	targets := store.Targets{DB: testdb.Open(t)}
	a := store.Target{Catalog: "KIC", CatalogID: "1", RA: 10, Dec: 10, DisplayName: "A", Designation: "KIC 1"}
	b := store.Target{Catalog: "KIC", CatalogID: "2", RA: 20, Dec: 20, DisplayName: "B", Designation: "KIC 2"}
	if _, err := targets.Upsert(ctx, a, time.Now(), "shared-name"); err != nil {
		t.Fatal(err)
	}
	if _, err := targets.Upsert(ctx, b, time.Now(), "shared-name"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := targets.ByAlias(ctx, "shared name", "KIC")
	if err != nil || !ok || got.CatalogID != "2" {
		t.Fatalf("alias should follow the latest resolution: %+v ok=%v err=%v", got, ok, err)
	}
}

func TestNormalizeAlias(t *testing.T) {
	for in, want := range map[string]string{
		"  Kepler-186 f ": "kepler186f",
		"HAT-P-7":         "hatp7",
		"TOI_178":         "toi178",
		"":                "",
	} {
		if got := store.NormalizeAlias(in); got != want {
			t.Errorf("NormalizeAlias(%q) = %q, want %q", in, got, want)
		}
	}
}
