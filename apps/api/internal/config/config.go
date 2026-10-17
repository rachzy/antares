package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr             string
	DBPath           string
	ShaulaAddr       string
	FangAddr         string
	MaxActive        int
	MaxQueue         int
	PromotionGrace   time.Duration
	JobTimeout       time.Duration
	StallTimeout     time.Duration
	TokenRetention   time.Duration
	TESSTTL          time.Duration
	RingSize         int
	ConnBuffer       int
	CoalesceInterval time.Duration
	AllowedOrigins   []string
}

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
