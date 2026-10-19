package config

import (
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" || cfg.DBPath != "antares.db" {
		t.Fatalf("addr/db = %q/%q", cfg.Addr, cfg.DBPath)
	}
	if cfg.ShaulaAddr != "127.0.0.1:50051" || cfg.FangAddr != "127.0.0.1:50052" {
		t.Fatalf("upstream addrs = %q/%q", cfg.ShaulaAddr, cfg.FangAddr)
	}
	if cfg.MaxActive != 2 || cfg.MaxQueue != 50 {
		t.Fatalf("slots = %d/%d", cfg.MaxActive, cfg.MaxQueue)
	}
	want := map[string][2]time.Duration{
		"grace":     {cfg.PromotionGrace, 30 * time.Second},
		"timeout":   {cfg.JobTimeout, 30 * time.Minute},
		"stall":     {cfg.StallTimeout, 5 * time.Minute},
		"retention": {cfg.TokenRetention, 7 * 24 * time.Hour},
		"tess":      {cfg.TESSTTL, 30 * 24 * time.Hour},
		"coalesce":  {cfg.CoalesceInterval, 100 * time.Millisecond},
		"catalog":   {cfg.CatalogTTL, 24 * time.Hour},
	}
	for name, pair := range want {
		if pair[0] != pair[1] {
			t.Errorf("%s = %s, want %s", name, pair[0], pair[1])
		}
	}
	if cfg.RingSize != 500 || cfg.ConnBuffer != 64 {
		t.Fatalf("ring/conn = %d/%d", cfg.RingSize, cfg.ConnBuffer)
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Fatalf("origins = %v, want none", cfg.AllowedOrigins)
	}
}

func TestOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"ANTARES_ADDR":              ":9090",
		"ANTARES_MAX_ACTIVE":        "4",
		"ANTARES_PROMOTION_GRACE":   "5s",
		"ANTARES_ALLOWED_ORIGINS":   "http://localhost:4200, http://example.test ,,",
		"ANTARES_COALESCE_INTERVAL": "250ms",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9090" || cfg.MaxActive != 4 || cfg.PromotionGrace != 5*time.Second {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.CoalesceInterval != 250*time.Millisecond {
		t.Fatalf("coalesce = %s", cfg.CoalesceInterval)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[1] != "http://example.test" {
		t.Fatalf("origins = %q", cfg.AllowedOrigins)
	}
}

func TestInvalidValuesAreAllReported(t *testing.T) {
	_, err := Load(env(map[string]string{
		"ANTARES_MAX_ACTIVE":  "two",
		"ANTARES_JOB_TIMEOUT": "soon",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"ANTARES_MAX_ACTIVE", "ANTARES_JOB_TIMEOUT"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not name %s", err, key)
		}
	}
}

func TestLimitsMustBePositive(t *testing.T) {
	for key, value := range map[string]string{
		"ANTARES_MAX_ACTIVE":  "0",
		"ANTARES_RING_SIZE":   "0",
		"ANTARES_CONN_BUFFER": "-1",
		"ANTARES_JOB_TIMEOUT": "0s",
	} {
		if _, err := Load(env(map[string]string{key: value})); err == nil {
			t.Errorf("%s=%s was accepted", key, value)
		}
	}
}

func TestZeroQueueIsAllowed(t *testing.T) {
	if _, err := Load(env(map[string]string{"ANTARES_MAX_QUEUE": "0"})); err != nil {
		t.Fatalf("a zero-length queue (reject when busy) must be valid: %v", err)
	}
}
