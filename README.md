# pg_slot_wal_timeout

A Go daemon that monitors PostgreSQL replication slots and **drops** those whose retained WAL files exceed a configurable age.

PostgreSQL provides `max_slot_wal_keep_size` to limit WAL retention by size, but has no equivalent based on **time**. This daemon fills that gap.

## How it works

On each check interval the daemon:

1. Queries `pg_replication_slots` joined with `pg_ls_waldir()` to determine the age of the oldest WAL file retained by each slot.
2. Compares that age against the configured threshold (`--max-wal-keep-time`).
3. For stale slots:
   - If the slot has an active connection, terminates the backend (`pg_terminate_backend`) and waits up to 5 s for it to become inactive.
   - Drops the slot (`pg_drop_replication_slot`).

> **Requires** the `pg_monitor` role or superuser privileges.

## Installation

### From source

```bash
git clone <repo-url> && cd pg_slot_wal_timeout
go build -o pg_slot_wal_timeout .
```

### Pre-built binary

Copy the `pg_slot_wal_timeout` binary to a directory in your `$PATH`.

## Usage

```bash
# Basic usage — drop slots retaining WAL older than 1 hour, check every minute
pg_slot_wal_timeout --dsn "postgres://user:pass@localhost:5432/postgres"

# Custom thresholds
pg_slot_wal_timeout \
  --dsn "postgres://localhost:5432/postgres" \
  --max-wal-keep-time 30m \
  --check-interval 15s

# Dry-run — log what would be dropped without actually dropping
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --dry-run

# Target a specific slot
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --slot-name my_replica_slot

# Target multiple specific slots
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --slot-name "slot_a,slot_b,slot_c"

# Use a glob pattern to match slot names
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --slot-name "replica_*"

# Mix exact names and patterns
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --slot-name "critical_slot,staging_*"

# Monitor all slots except specific ones
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --slot-exclude "do_not_touch"

# Monitor all slots except those matching a pattern
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --slot-exclude "prod_*"

# Combine: target staging slots but protect a specific one
pg_slot_wal_timeout --dsn "postgres://localhost:5432/postgres" --slot-name "staging_*" --slot-exclude "staging_critical"
```

## Configuration

Every flag can also be set via an environment variable. Flags take precedence.

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--dsn` | `PG_DSN` | `postgres://localhost:5432/postgres` | PostgreSQL connection string |
| `--max-wal-keep-time` | `PG_MAX_WAL_KEEP_TIME` | `1h` | Maximum WAL retention age (Go duration: `30m`, `1h`, `24h`, ...) |
| `--check-interval` | `PG_CHECK_INTERVAL` | `1m` | Interval between checks |
| `--dry-run` | `PG_DRY_RUN` | `false` | Log stale slots without dropping them |
| `--slot-name` | `PG_SLOT_NAME` | `*` (all) | Slot names to monitor (comma-separated, glob patterns allowed) |
| `--slot-exclude` | `PG_SLOT_EXCLUDE` | *(none)* | Slot names to exclude (comma-separated, glob patterns allowed) |

## Running as a systemd service

```ini
# /etc/systemd/system/pg-slot-wal-timeout.service
[Unit]
Description=pg_slot_wal_timeout — WAL retention watchdog
After=postgresql.service

[Service]
Type=simple
ExecStart=/usr/local/bin/pg_slot_wal_timeout \
  --dsn "postgres://monitor:secret@localhost:5432/postgres" \
  --max-wal-keep-time 1h \
  --check-interval 1m
Restart=on-failure
User=postgres

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now pg-slot-wal-timeout
```

## Required PostgreSQL privileges

The connecting role needs access to `pg_replication_slots`, `pg_ls_waldir()`, `pg_walfile_name()`, `pg_terminate_backend()`, and `pg_drop_replication_slot()`.

The simplest setup:

```sql
CREATE ROLE slot_monitor LOGIN PASSWORD 'secret';
GRANT pg_monitor TO slot_monitor;
-- pg_monitor grants read access; superuser is needed for pg_drop_replication_slot
-- on PG < 16. On PG 16+ you can use pg_checkpoint role or grant explicitly.
ALTER ROLE slot_monitor SUPERUSER;  -- or grant specific privileges if PG 16+
```

## Log output

The daemon uses Go's structured logging (`slog`). Example output:

```
2025/02/20 15:00:00 INFO starting pg_slot_wal_timeout dsn=postgres://localhost:5432/postgres max_wal_keep_time=1h0m0s check_interval=1m0s dry_run=false slot_names=[*]
2025/02/20 15:00:00 INFO connected to PostgreSQL
2025/02/20 15:00:00 WARN dropping stale slot slot=replica_old wal_age=2h15m33s restart_lsn=0/5000000
2025/02/20 15:00:00 INFO dropping replication slot slot=replica_old
2025/02/20 15:00:00 INFO slot dropped successfully slot=replica_old
```

In dry-run mode:

```
2025/02/20 15:00:00 WARN [DRY-RUN] would drop stale slot slot=replica_old wal_age=2h15m33s restart_lsn=0/5000000
```

## License

MIT
