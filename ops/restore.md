# Backup and restore

This document covers nightly PostgreSQL backups, restore steps, and a basic verification checklist.

## 1) Nightly backup with pg_dump

Run a nightly backup from cron on the host.

Example script:

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR=/data/xreader/backups
mkdir -p "$BACKUP_DIR"

TIMESTAMP="$(date +%F-%H%M%S)"
BACKUP_FILE="$BACKUP_DIR/xreader-$TIMESTAMP.sql.gz"

docker compose -f /data/xreader/deploy/docker-compose.yml exec -T postgres \
  pg_dump -U xreader -d xreader \
  | gzip > "$BACKUP_FILE"
```

Suggested cron entry:

```cron
0 2 * * * /usr/local/bin/xreader-backup.sh
```

Keep a retention policy so the backup directory does not grow without bound.

## 2) Backup verification

Verify that the backup job is still working every day:

1. Confirm the backup file exists and is non-empty.
2. Check the gzip file can be read:

   ```bash
   gzip -t /data/xreader/backups/xreader-YYYY-MM-DD-HHMMSS.sql.gz
   ```

3. Inspect the latest backup timestamp.
4. Periodically restore into a scratch database to confirm the dump is usable.

## 3) Restore procedure

When you need to restore the database:

1. Stop the xreader service so nothing writes during restore:

   ```bash
   docker compose stop xreader
   ```

2. Drop and recreate the database:

   ```bash
   docker compose exec postgres psql -U xreader -c "DROP DATABASE xreader;"
   docker compose exec postgres psql -U xreader -d postgres -c "CREATE DATABASE xreader OWNER xreader;"
   ```

3. Restore the backup into PostgreSQL:

   ```bash
   gunzip -c /data/xreader/backups/xreader-YYYY-MM-DD-HHMMSS.sql.gz \
     | docker compose exec -T postgres psql -U xreader -d xreader
   ```

4. Start xreader again (migrations run automatically on startup):

   ```bash
   docker compose start xreader
   ```

## 4) Post-restore checks

After the restore completes, verify:

- `curl -fsS http://localhost:3000/health` returns `{"status":"ok"}`
- The xreader logs show successful database connection
- Articles, sources, and admin allowlist rows are present
- The web app loads and sign-in works

## 5) Disaster recovery notes

- Store backups on a volume separate from the live Postgres data directory.
- Test the restore path on a staging host before you need it in production.
- Keep at least one off-host copy if the homelab storage itself is the failure domain.
