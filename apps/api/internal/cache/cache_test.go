package cache_test

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"antares/api/internal/cache"
	"antares/api/internal/store"
	"antares/api/internal/testdb"
)

func targetID(t *testing.T, targets store.Targets) int64 {
	t.Helper()
	got, err := targets.Upsert(context.Background(), store.Target{
		Catalog: "KIC", CatalogID: "8120608", RA: 298.65, Dec: 43.95,
		DisplayName: "Kepler-186", Designation: "KIC 8120608",
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return got.ID
}

func TestSanitizeMapsNonFiniteToNull(t *testing.T) {
	in := map[string]any{
		"nan":    math.NaN(),
		"inf":    math.Inf(1),
		"ok":     1.5,
		"text":   "x",
		"nested": map[string]any{"deep": math.Inf(-1)},
		"list":   []any{math.NaN(), 2.0},
	}
	got := cache.Sanitize(in).(map[string]any)
	if got["nan"] != nil || got["inf"] != nil || got["ok"] != 1.5 || got["text"] != "x" {
		t.Fatalf("top level: %v", got)
	}
	if got["nested"].(map[string]any)["deep"] != nil {
		t.Fatalf("nested: %v", got["nested"])
	}
	if list := got["list"].([]any); list[0] != nil || list[1] != 2.0 {
		t.Fatalf("list: %v", list)
	}
	if !math.IsNaN(in["nan"].(float64)) {
		t.Fatal("Sanitize must not mutate its input")
	}
}

func TestFeaturesHashIsStableAcrossKeyOrderAndMissingValues(t *testing.T) {
	a := []map[string]any{{"a": 1.0, "b": math.NaN(), "c": "x"}}
	b := []map[string]any{{"c": "x", "b": nil, "a": 1.0}}
	ha, err := cache.FeaturesHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, _ := cache.FeaturesHash(b)
	if ha != hb {
		t.Fatalf("NaN and null must hash alike: %s vs %s", ha, hb)
	}
	changed := []map[string]any{{"a": 1.0000001, "b": nil, "c": "x"}}
	if hc, _ := cache.FeaturesHash(changed); hc == ha {
		t.Fatal("a different value must hash differently")
	}
	reordered := []map[string]any{{"a": 1.0}, {"a": 2.0}}
	swapped := []map[string]any{{"a": 2.0}, {"a": 1.0}}
	h1, _ := cache.FeaturesHash(reordered)
	h2, _ := cache.FeaturesHash(swapped)
	if h1 == h2 {
		t.Fatal("row order is significant")
	}
}

func TestExtractParamsHashChangesWithEveryField(t *testing.T) {
	base := cache.ExtractParams{Mission: "Kepler", SigmaClip: 5, Author: "Kepler", Exptime: "long"}
	seen := map[string]string{base.Hash(): "base"}
	variants := map[string]cache.ExtractParams{
		"mission":       {Mission: "TESS", SigmaClip: 5, Author: "Kepler", Exptime: "long"},
		"sigma":         {Mission: "Kepler", SigmaClip: 3, Author: "Kepler", Exptime: "long"},
		"download_all":  {Mission: "Kepler", SigmaClip: 5, DownloadAll: true, Author: "Kepler", Exptime: "long"},
		"author":        {Mission: "Kepler", SigmaClip: 5, Author: "SPOC", Exptime: "long"},
		"exptime":       {Mission: "Kepler", SigmaClip: 5, Author: "Kepler", Exptime: "short"},
		"use_tls":       {Mission: "Kepler", SigmaClip: 5, Author: "Kepler", Exptime: "long", UseTLS: true},
		"mask_eclipses": {Mission: "Kepler", SigmaClip: 5, Author: "Kepler", Exptime: "long", MaskEclipses: true},
	}
	for name, p := range variants {
		h := p.Hash()
		if other, dup := seen[h]; dup {
			t.Errorf("%s collides with %s", name, other)
		}
		seen[h] = name
	}
	if base.Hash() != base.Hash() {
		t.Fatal("Hash must be deterministic")
	}
}

func TestFeatureCacheRoundTripAndMisses(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t)
	id := targetID(t, store.Targets{DB: db})
	features := cache.FeatureCache{DB: db}
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	rows := []map[string]any{{"period_days": 129.9, "vshape_metric": math.NaN()}}
	entry := cache.FeatureEntry{TargetID: id, ParamsHash: "p1", ShaulaVersion: "0.0.1a0", Rows: rows, CreatedAt: now}
	if err := features.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}

	got, ok, err := features.Get(ctx, id, "p1", "0.0.1a0", now)
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Rows[0]["period_days"] != 129.9 || got.Rows[0]["vshape_metric"] != nil {
		t.Fatalf("rows = %v", got.Rows)
	}
	want, _ := cache.FeaturesHash(rows)
	if got.FeaturesHash != want {
		t.Fatalf("stored hash %s, want %s", got.FeaturesHash, want)
	}

	for name, args := range map[string]struct {
		id              int64
		params, version string
	}{
		"other params":  {id, "p2", "0.0.1a0"},
		"other version": {id, "p1", "0.0.2"},
		"other target":  {id + 99, "p1", "0.0.1a0"},
	} {
		if _, ok, _ := features.Get(ctx, args.id, args.params, args.version, now); ok {
			t.Errorf("%s: unexpected hit", name)
		}
	}
}

func TestEmptyFeatureListIsACacheableAnswer(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t)
	id := targetID(t, store.Targets{DB: db})
	features := cache.FeatureCache{DB: db}
	now := time.Now()
	if err := features.Put(ctx, cache.FeatureEntry{TargetID: id, ParamsHash: "p", ShaulaVersion: "v", Rows: nil, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := features.Get(ctx, id, "p", "v", now)
	if err != nil || !ok || got.Rows == nil || len(got.Rows) != 0 {
		t.Fatalf("a star with no candidates must round-trip as an empty list: %+v ok=%v err=%v", got, ok, err)
	}
}

func TestSupersedeInvalidatesOtherVersionsAndPutRevives(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t)
	id := targetID(t, store.Targets{DB: db})
	features := cache.FeatureCache{DB: db}
	now := time.Now()
	put := func(version string) {
		t.Helper()
		err := features.Put(ctx, cache.FeatureEntry{TargetID: id, ParamsHash: "p", ShaulaVersion: version,
			Rows: []map[string]any{{"v": version}}, CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	put("old")
	put("new")

	n, err := features.Supersede(ctx, "new", now)
	if err != nil || n != 1 {
		t.Fatalf("Supersede = %d, %v; want 1", n, err)
	}
	if _, ok, _ := features.Get(ctx, id, "p", "old", now); ok {
		t.Error("a superseded entry must not be served")
	}
	if _, ok, _ := features.Get(ctx, id, "p", "new", now); !ok {
		t.Error("the current version must still be served")
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM feature_cache`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("superseded rows must be kept, found %d (err %v)", rows, err)
	}

	put("old")
	if _, ok, _ := features.Get(ctx, id, "p", "old", now); !ok {
		t.Error("recomputing under a rolled-back version must revive the entry")
	}
}

func TestExpiredEntriesAreNotServed(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t)
	id := targetID(t, store.Targets{DB: db})
	features := cache.FeatureCache{DB: db}
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	err := features.Put(ctx, cache.FeatureEntry{TargetID: id, ParamsHash: "p", ShaulaVersion: "v",
		Rows: []map[string]any{{"a": 1.0}}, CreatedAt: now, ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := features.Get(ctx, id, "p", "v", now.Add(30*time.Minute)); !ok {
		t.Error("an unexpired entry must be served")
	}
	if _, ok, _ := features.Get(ctx, id, "p", "v", now.Add(2*time.Hour)); ok {
		t.Error("an expired entry must not be served")
	}
}

func TestExpiresAtOnlyForTESS(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	ttl := 30 * 24 * time.Hour
	if got := cache.ExpiresAt("TESS", now, ttl); got == nil || !got.Equal(now.Add(ttl)) {
		t.Errorf("TESS = %v", got)
	}
	for _, mission := range []string{"Kepler", "K2"} {
		if got := cache.ExpiresAt(mission, now, ttl); got != nil {
			t.Errorf("%s must not expire, got %v", mission, got)
		}
	}
}

func TestPredictionCacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t)
	id := targetID(t, store.Targets{DB: db})
	predictions := cache.PredictionCache{DB: db}
	rows := []map[string]any{{"prob_stack": 0.71, "star_id": "KIC-8120608", "skipped": math.NaN()}}
	entry := cache.PredictionEntry{FeaturesHash: "fh", ModelRunID: "run-1", SchemaVersion: 1, Rows: rows,
		TargetID: &id, CreatedAt: time.Now()}
	if err := predictions.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}
	got, ok, err := predictions.Get(ctx, "fh", "run-1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.SchemaVersion != 1 || got.Rows[0]["prob_stack"] != 0.71 || got.Rows[0]["skipped"] != nil {
		t.Fatalf("got %+v", got)
	}
	if got.TargetID == nil || *got.TargetID != id {
		t.Fatalf("target = %v", got.TargetID)
	}
	if _, ok, _ := predictions.Get(ctx, "fh", "run-2"); ok {
		t.Error("a different run must miss: predictions are keyed on the run id")
	}
	if _, ok, _ := predictions.Get(ctx, "other", "run-1"); ok {
		t.Error("different features must miss")
	}
	if err := predictions.Put(ctx, entry); err != nil {
		t.Fatalf("putting the same entry twice must not fail: %v", err)
	}
	if !reflect.DeepEqual(got.ModelRunID, "run-1") {
		t.Fatal("run id lost")
	}
}
