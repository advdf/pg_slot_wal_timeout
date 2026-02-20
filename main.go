package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := ParseConfig()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("starting pg_slot_wal_timeout",
		"dsn", cfg.DSN,
		"max_wal_keep_time", cfg.MaxWALKeepTime,
		"check_interval", cfg.CheckInterval,
		"dry_run", cfg.DryRun,
		"slot_names", cfg.SlotNames,
		"slot_exclude", cfg.SlotExclude,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		slog.Error("failed to create connection pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Verify connectivity.
	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to PostgreSQL")

	// Run first check immediately, then on interval.
	runCheck(ctx, pool, cfg)

	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			return
		case <-ticker.C:
			runCheck(ctx, pool, cfg)
		}
	}
}

func runCheck(ctx context.Context, pool *pgxpool.Pool, cfg *Config) {
	staleSlots, err := CheckSlots(ctx, pool, cfg.MaxWALKeepTime, cfg.SlotNames, cfg.SlotExclude)
	if err != nil {
		slog.Error("failed to check slots", "error", err)
		return
	}

	if len(staleSlots) == 0 {
		slog.Info("no stale slots found")
		return
	}

	for _, slot := range staleSlots {
		logger := slog.With("slot", slot.Name, "wal_age", slot.WALAge.Truncate(time.Second), "restart_lsn", slot.RestartLSN)

		if cfg.DryRun {
			logger.Warn("[DRY-RUN] would drop stale slot")
			continue
		}

		logger.Warn("dropping stale slot")
		if err := InvalidateSlot(ctx, pool, slot); err != nil {
			logger.Error("failed to invalidate slot", "error", err)
		}
	}
}
