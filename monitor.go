package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StaleSlot struct {
	Name       string
	ActivePID  int32
	RestartLSN string
	WALAge     time.Duration
}

const checkSlotsQuery = `
SELECT s.slot_name,
       COALESCE(s.active_pid, 0),
       s.restart_lsn::text,
       EXTRACT(EPOCH FROM (now() - w.modification)) AS wal_age_seconds
FROM pg_replication_slots s
JOIN pg_ls_waldir() w ON w.name = pg_walfile_name(s.restart_lsn)
WHERE s.restart_lsn IS NOT NULL
`

func CheckSlots(ctx context.Context, pool *pgxpool.Pool, maxAge time.Duration, slotFilter string) ([]StaleSlot, error) {
	rows, err := pool.Query(ctx, checkSlotsQuery)
	if err != nil {
		return nil, fmt.Errorf("query replication slots: %w", err)
	}
	defer rows.Close()

	var stale []StaleSlot
	for rows.Next() {
		var s StaleSlot
		var walAgeSeconds float64
		if err := rows.Scan(&s.Name, &s.ActivePID, &s.RestartLSN, &walAgeSeconds); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		s.WALAge = time.Duration(walAgeSeconds * float64(time.Second))

		if !matchFilter(s.Name, slotFilter) {
			continue
		}

		if s.WALAge > maxAge {
			stale = append(stale, s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return stale, nil
}

func matchFilter(name, pattern string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}
	return matched
}

