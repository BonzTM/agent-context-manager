# Security and privacy

ACM stores retained conversation content locally. Capture input is untrusted and
may contain credentials, personal data, proprietary source, or hostile payloads.
The project policy is applied before any conversation or message row is created.

## Threat model

Owner-only (`0600`) modes protect the database, WAL/SHM sidecars, backups, and
offloaded files from ordinary access by other non-privileged local accounts.
They do not protect against:

- the same user account, malware running as that user, or a host administrator;
- raw-disk or snapshot access where the filesystem is not encrypted;
- secrets already captured by an older ACM version or before a policy change;
- copies exported outside ACM; or
- a lost, compromised, or unencrypted host backup.

Use operating-system full-disk encryption and encrypted backup storage when
those actors are in scope. Database encryption is deferred by
[ADR 0002](adr/0002-defer-database-encryption.md); ACM does not currently manage
encryption keys or promise encrypted database pages.

## Project policy

Place `.acm-policy.toml` at the project root. A missing file uses secure default
secret redaction with no exclusions. Omitting `redact` also leaves redaction on;
only the explicit `redact = false` setting disables it.

```toml
# Secure default: true. Set false only when the storage boundary is acceptable.
redact = true

exclude_sessions = ["scratch-*", "vendor-audit-*"]
exclude_tools = ["SecretRead", "PasswordManager*"]
exclude_paths = ["/private/*", "*/.env"]
exclude_content_classes = ["private-key", "personal-data"]

# Exact matched values/substrings that are known false positives.
allow_values = ["not-a-real-secret"]
```

Session, tool, and path rules use Go filepath globs. Path rules inspect bounded
structured hook fields whose names contain `path` or `file`, plus `cwd`.
Supported exclusion classes are `secrets`, `private-key`, `aws-key`,
`github-token`, `api-token`, `jwt`, `bearer-token`, `credential`, and
`personal-data` (email addresses and US SSN-shaped values).

Default redaction recognizes private-key blocks, AWS access-key IDs, GitHub and
`sk-` API tokens, JWTs, bearer tokens, and common credential assignments.
Markers such as `[REDACTED:api-token]` replace matches in both canonical content
and raw hook JSON. Allow values bypass a matching redaction and should therefore
contain only reviewed false positives.

Policy files are bounded to 1 MiB and 128 rules per category. Raw JSON path
inspection and per-message redaction also have finite work limits; exceeding a
limit rejects the capture before persistence.

## Data flow guarantee

Privacy filtering runs in `core.Service.Ingest` before `EnsureConversation` and
`AppendMessage`. Consequently excluded or original secret values do not enter:

- message rows or raw payloads;
- FTS indexes;
- summary sources or generated summaries;
- offloaded large-content files; or
- database backups created after filtering.

Excluded sessions create no conversation or message rows. Operational logs and
CLI results report counts and redaction classes, never the matched values.

Changing policy is prospective. Run an appropriate lifecycle prune after `#5`
if older retained data must be removed; until then, restore from a pre-capture
backup or delete the project state only through documented ACM operations.
