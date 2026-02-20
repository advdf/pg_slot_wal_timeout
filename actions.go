package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InvalidateSlot(ctx context.Context, pool *pgxpool.Pool, slot StaleSlot) error {
	logger := slog.With("slot", slot.Name, "wal_age", slot.WALAge.Truncate(time.Second))

	// If the slot has an active connection, terminate it first.
	if slot.ActivePID != 0 {
		logger.Info("terminating backend for active slot", "pid", slot.ActivePID)
		_, err := pool.Exec(ctx, "SELECT pg_terminate_backend($1)", slot.ActivePID)
		if err != nil {
			return fmt.Errorf("terminate backend pid %d: %w", slot.ActivePID, err)
		}

		// Wait for the slot to become inactive (max 5s).
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var activePID int32
			err := pool.QueryRow(ctx,
				"SELECT COALESCE(active_pid, 0) FROM pg_replication_slots WHERE slot_name = $1",
				slot.Name,
			).Scan(&activePID)
			if err != nil {
				// Slot may already be gone.
				return nil
			}
			if activePID == 0 {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.Info("dropping replication slot")
	_, err := pool.Exec(ctx, "SELECT pg_drop_replication_slot($1)", slot.Name)
	if err != nil {
		return fmt.Errorf("drop slot %q: %w", slot.Name, err)
	}

	logger.Info("slot dropped successfully")
	return nil
}
