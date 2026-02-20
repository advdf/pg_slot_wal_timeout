package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	DSN            string
	MaxWALKeepTime time.Duration
	CheckInterval  time.Duration
	DryRun         bool
	SlotFilter     string
}

func ParseConfig() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.DSN, "dsn", envOrDefault("PG_DSN", "postgres://localhost:5432/postgres"), "PostgreSQL connection string")
	flag.DurationVar(&cfg.MaxWALKeepTime, "max-wal-keep-time", envDurationOrDefault("PG_MAX_WAL_KEEP_TIME", time.Hour), "Maximum WAL retention duration")
	flag.DurationVar(&cfg.CheckInterval, "check-interval", envDurationOrDefault("PG_CHECK_INTERVAL", time.Minute), "Interval between checks")
	flag.BoolVar(&cfg.DryRun, "dry-run", envBoolOrDefault("PG_DRY_RUN", false), "Log actions without executing them")
	flag.StringVar(&cfg.SlotFilter, "slot-filter", envOrDefault("PG_SLOT_FILTER", "*"), "Glob pattern to filter slot names")

	flag.Parse()

	if cfg.MaxWALKeepTime <= 0 {
		return nil, fmt.Errorf("--max-wal-keep-time must be positive")
	}
	if cfg.CheckInterval <= 0 {
		return nil, fmt.Errorf("--check-interval must be positive")
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envDurationOrDefault(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return def
}

func envBoolOrDefault(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		return v == "1" || v == "true" || v == "yes"
	}
	return def
}
