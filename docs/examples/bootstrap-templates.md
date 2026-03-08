# Bootstrap Templates

`acm bootstrap` accepts repeatable `--apply-template <id>` flags so you can start minimal, then opt into heavier scaffolding later without replacing edited repo files.

Example:

```bash
acm bootstrap \
  --apply-template starter-contract \
  --apply-template verify-generic \
  --apply-template claude-command-pack
```

Current built-ins:

- `starter-contract`
  - seeds `AGENTS.md` and `CLAUDE.md`
  - upgrades blank ACM rules scaffolds to a richer starter ruleset
- `verify-generic`
  - upgrades blank ACM test scaffolds to a language-agnostic verify profile
  - uses `acm status` plus git checks so it works out of the box
- `verify-go`
  - upgrades blank ACM test scaffolds to a Go-oriented verify profile
- `verify-ts`
  - upgrades blank ACM test scaffolds to a TypeScript-oriented verify profile
- `verify-python`
  - upgrades blank ACM test scaffolds to a Python-oriented verify profile
- `verify-rust`
  - upgrades blank ACM test scaffolds to a Rust-oriented verify profile
- `claude-command-pack`
  - seeds `.claude/commands/*` and `.claude/acm-broker/*`
- `claude-receipt-guard`
  - merges additive Claude hook settings into `.claude/settings.json`
  - seeds `.claude/hooks/acm-receipt-guard.sh` and `.claude/hooks/acm-receipt-mark.sh`
  - keeps edits blocked until a task-bearing `/acm-get` or equivalent `get_context` request succeeds in the session
- `git-hooks-precommit`
  - seeds `.githooks/pre-commit`
  - forwards staged additions, modifications, renames, type changes, and deletions into `acm verify --phase review`

Safety contract:

- templates create missing files
- templates may replace ACM-owned blank scaffolds only while they are still pristine
- templates may merge known additive JSON fragments such as `.claude/settings.json`
- templates never delete files
- templates never overwrite user-edited files; conflicts are skipped
