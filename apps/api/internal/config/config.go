// Package config loads the API's settings from ANTARES_* environment variables.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds the API's runtime settings. Use Load: several fields must be positive, so the zero
// value is not usable.
type Config struct {
	// Addr is the HTTP listen address.
	Addr string
	// DBPath is the SQLite database file.
	DBPath string
	// ShaulaAddr and FangAddr are the gRPC addresses of the private services.
	ShaulaAddr string
	FangAddr   string
	// MaxActive is how many jobs may run at once.
	MaxActive int
	// MaxQueue is how many jobs may wait for a slot; with 0, a submission is rejected whenever
	// every slot is busy.
	MaxQueue int
	// PromotionGrace is how long after its last poll or disconnect a queued job still counts as
	// watched. Unwatched jobs are dropped instead of started.
	PromotionGrace time.Duration
	// JobTimeout bounds a job's whole run.
	JobTimeout time.Duration
	// StallTimeout is how long a running job may go without a progress event before it is
	// cancelled.
	StallTimeout time.Duration
	// TokenRetention is how long a subscription token stays valid after it is issued.
	TokenRetention time.Duration
	// TESSTTL is how long cached TESS features are served; Kepler and K2 features never expire.
	TESSTTL time.Duration
	// RingSize is how many recent events each job keeps in memory for replay.
	RingSize int
	// ConnBuffer is each subscriber's event buffer; a subscriber that falls this far behind is
	// disconnected.
	ConnBuffer int
	// CoalesceInterval is how long the WebSocket stream gathers events into one frame.
	CoalesceInterval time.Duration
	// AllowedOrigins are the browser origins allowed for CORS and WebSocket upgrades; empty allows
	// no cross-origin access.
	AllowedOrigins []string
}

// Load reads each setting from its ANTARES_ variable through getenv, using the default when the
// variable is unset. Every malformed or out-of-range value is reported in one joined error, in
// which case the returned Config must not be used.
func Load(getenv func(string) string) (Config, error) {
	var errs []error

	text := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}
	number := func(key string, fallback int) int {
		v := getenv(key)
		if v == "" {
			return fallback
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			return fallback
		}
		return n
	}
	duration := func(key string, fallback time.Duration) time.Duration {
		v := getenv(key)
		if v == "" {
			return fallback
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			return fallback
		}
		return d
	}

	cfg := Config{
		Addr:             text("ANTARES_ADDR", ":8080"),
		DBPath:           text("ANTARES_DB_PATH", "antares.db"),
		ShaulaAddr:       text("ANTARES_SHAULA_ADDR", "127.0.0.1:50051"),
		FangAddr:         text("ANTARES_FANG_ADDR", "127.0.0.1:50052"),
		MaxActive:        number("ANTARES_MAX_ACTIVE", 2),
		MaxQueue:         number("ANTARES_MAX_QUEUE", 50),
		PromotionGrace:   duration("ANTARES_PROMOTION_GRACE", 30*time.Second),
		JobTimeout:       duration("ANTARES_JOB_TIMEOUT", 30*time.Minute),
		StallTimeout:     duration("ANTARES_STALL_TIMEOUT", 5*time.Minute),
		TokenRetention:   duration("ANTARES_TOKEN_RETENTION", 7*24*time.Hour),
		TESSTTL:          duration("ANTARES_TESS_TTL", 30*24*time.Hour),
		RingSize:         number("ANTARES_RING_SIZE", 500),
		ConnBuffer:       number("ANTARES_CONN_BUFFER", 64),
		CoalesceInterval: duration("ANTARES_COALESCE_INTERVAL", 100*time.Millisecond),
	}

	for _, origin := range strings.Split(getenv("ANTARES_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
		}
	}

	positive := func(key string, ok bool) {
		if !ok {
			errs = append(errs, fmt.Errorf("%s must be positive", key))
		}
	}
	positive("ANTARES_MAX_ACTIVE", cfg.MaxActive >= 1)
	positive("ANTARES_RING_SIZE", cfg.RingSize >= 1)
	positive("ANTARES_CONN_BUFFER", cfg.ConnBuffer >= 1)
	positive("ANTARES_MAX_QUEUE", cfg.MaxQueue >= 0)
	positive("ANTARES_PROMOTION_GRACE", cfg.PromotionGrace > 0)
	positive("ANTARES_JOB_TIMEOUT", cfg.JobTimeout > 0)
	positive("ANTARES_STALL_TIMEOUT", cfg.StallTimeout > 0)
	positive("ANTARES_TOKEN_RETENTION", cfg.TokenRetention > 0)
	positive("ANTARES_TESS_TTL", cfg.TESSTTL > 0)
	positive("ANTARES_COALESCE_INTERVAL", cfg.CoalesceInterval > 0)

	return cfg, errors.Join(errs...)
}
