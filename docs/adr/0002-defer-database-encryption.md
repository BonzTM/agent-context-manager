# 2. Defer application-level database encryption pending key management

- Status: Accepted
- Date: 2026-07-09

## Context

ACM persists sensitive local conversation history and currently uses
`modernc.org/sqlite` to preserve CGo-free, static builds across Linux, macOS,
and Windows. The driver's documentation describes it as a CGo-free SQLite port
but does not document an encryption-at-rest interface:
<https://pkg.go.dev/modernc.org/sqlite>.

The maintained SQLCipher project provides encrypted SQLite pages, tamper
detection, memory sanitization, and key derivation. Its official build
instructions require codec compile definitions and linking a cryptographic
provider such as OpenSSL:
<https://github.com/sqlcipher/sqlcipher#compiling>. Adopting it would replace
the current pure-Go build contract and require platform packaging changes.

SQLite's official SEE implementation is another page-encryption option, but
source and binaries require a commercial license:
<https://www.sqlite.org/see/doc/trunk/www/index.wiki>.

Encryption also requires product decisions that a driver swap cannot answer:
key source and storage, unattended hook access, rotation, backup/export keys,
plaintext-to-encrypted migration, rollback, lost-key recovery, and how WAL,
temporary files, offloads, and crash artifacts share the boundary.

## Decision

Do not add application-level database encryption in this release. Keep the
current CGo-free driver and static release matrix. Enforce owner-only modes,
redact secrets before persistence by default, support capture exclusions, and
document the residual local attacker model.

Encryption may be reconsidered only through a replacement ADR that includes:

1. a maintained driver and license review;
2. cross-platform static/build consequences;
3. an explicit key-management and unattended-hook contract;
4. tested plaintext migration, rotation, backup, restore, and rollback;
5. lost-key behavior with no misleading recovery promise; and
6. coverage for WAL, temporary files, offloads, and backups.

## Consequences

- ACM database pages remain plaintext and rely on host/disk encryption against
  raw storage access.
- Default redaction reduces newly persisted secret exposure but is not a
  substitute for encryption and does not rewrite historical data.
- Static CGo-free artifacts and the existing portability contract remain intact.
- Users who require application-managed encrypted pages must treat this as an
  unmet requirement until a key-management design is accepted and implemented.
