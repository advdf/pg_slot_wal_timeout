package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DSN            string
	MaxWALKeepTime time.Duration
	CheckInterval  time.Duration
	DryRun         bool
	SlotNames      []string // exact names or glob patterns, ["*"] means all
}

func ParseConfig() (*Config, error) {
	cfg := &Config{}

	var slotNameRaw string

	flag.StringVar(&cfg.DSN, "dsn", envOrDefault("PG_DSN", "postgres://localhost:5432/postgres"), "PostgreSQL connection string")
	flag.DurationVar(&cfg.MaxWALKeepTime, "max-wal-keep-time", envDurationOrDefault("PG_MAX_WAL_KEEP_TIME", time.Hour), "Maximum WAL retention duration")
	flag.DurationVar(&cfg.CheckInterval, "check-interval", envDurationOrDefault("PG_CHECK_INTERVAL", time.Minute), "Interval between checks")
	flag.BoolVar(&cfg.DryRun, "dry-run", envBoolOrDefault("PG_DRY_RUN", false), "Log actions without executing them")
	flag.StringVar(&slotNameRaw, "slot-name", envOrDefault("PG_SLOT_NAME", "*"), "Slot names to monitor (comma-separated, glob patterns allowed, \"*\" = all)")

	flag.Parse()

	for _, s := range strings.Split(slotNameRaw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			cfg.SlotNames = append(cfg.SlotNames, s)
		}
	}

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
