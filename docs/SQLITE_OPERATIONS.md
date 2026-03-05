# SQLite Operations

## Deployment Defaults

- Always set `CTX_SQLITE_PATH` explicitly in non-local environments.
- Store the DB on persistent storage (not `/tmp`).
- Recommended location pattern: `/var/lib/agents-context/context.db`.
- Recommended ownership/permissions:
  - directory: `0700`
  - db file: `0600`

Example:

```bash
install -d -m 0700 /var/lib/agents-context
touch /var/lib/agents-context/context.db
chmod 0600 /var/lib/agents-context/context.db
export CTX_SQLITE_PATH=/var/lib/agents-context/context.db
```

## Backup

Use SQLite online backup mode (safe with concurrent readers/writers):

```bash
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
sqlite3 "$CTX_SQLITE_PATH" ".timeout 5000" ".backup '/var/backups/agents-context/context-$STAMP.sqlite'"
```

## Restore

Stop writers, then restore from a selected backup:

```bash
cp /var/backups/agents-context/context-<stamp>.sqlite "$CTX_SQLITE_PATH"
chmod 0600 "$CTX_SQLITE_PATH"
```

## Retention and Rotation

- Keep hourly backups for 24h.
- Keep daily backups for 14d.
- Keep weekly backups for 8w.
- Run retention cleanup via cron/systemd timer.

Example cleanup (keep last 14 days):

```bash
find /var/backups/agents-context -type f -name 'context-*.sqlite' -mtime +14 -delete
```

## Escalation Threshold

If multiple concurrent writers or high write volume become normal, use Postgres (`CTX_PG_DSN`) as the primary backend.
