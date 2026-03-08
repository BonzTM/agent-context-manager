# Bootstrap Templates

`acm bootstrap` accepts repeatable `--apply-template <id>` flags so you can start minimal, then opt into heavier scaffolding later without replacing edited repo files.

Example:

```bash
acm bootstrap \
  --apply-template starter-contract \
  --apply-template verify-generic \
  --apply-template claude-command-pack
```

For repos that want stricter feature planning, add `--apply-template detailed-planning-enforcement`. It can be applied directly or after `starter-contract` + `verify-generic`, and it only upgrades those scaffolds while they are still pristine.

Current built-ins:

- `starter-contract`
  - seeds `AGENTS.md` and `CLAUDE.md`
  - upgrades blank ACM rules scaffolds to a richer starter ruleset
- `detailed-planning-enforcement`
  - seeds `docs/feature-plans.md` and `scripts/acm-feature-plan-validate.py`
  - upgrades pristine `starter-contract` docs/rules and pristine blank or `verify-generic` test scaffolds to the richer feature-plan workflow
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
- `claude-hooks`
  - merges additive Claude hook settings into `.claude/settings.json`
  - seeds `.claude/hooks/acm-receipt-guard.sh`, `.claude/hooks/acm-receipt-mark.sh`, `.claude/hooks/acm-session-context.sh`, `.claude/hooks/acm-edit-state.sh`, and `.claude/hooks/acm-stop-guard.sh`
  - injects the ACM loop at session start/compaction, keeps edits blocked until a task-bearing `/acm-get` or equivalent `get_context` request succeeds, nudges `/acm-work` once edits span files, and blocks stop until edited work is reported
  - the older `claude-receipt-guard` id still resolves to this template as a compatibility alias
- `git-hooks-precommit`
  - seeds `.githooks/pre-commit`
  - forwards staged additions, modifications, renames, type changes, and deletions into `acm verify --phase review`

Safety contract:

- templates create missing files
- templates may replace ACM-owned blank scaffolds only while they are still pristine
- templates may merge known additive JSON fragments such as `.claude/settings.json`
- templates never delete files
- templates never overwrite user-edited files; conflicts are skipped
