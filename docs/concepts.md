# Concepts

This page defines the core terms used throughout acm. Read this if you're new or if a term in the docs isn't clear.

## Pointer

A pointer is an entry in acm's index that points to something in your codebase — a file, a function, a doc, a test, a rule. Each pointer has:

- **Key** — unique identifier in the format `project:path#anchor` (anchor is optional). Example: `my-cool-app:src/auth/login.go#handleTimeout`
- **Kind** — one of: `code`, `rule`, `doc`, `test`, `command`
- **Label** — short human-readable name
- **Description** — one-line summary of what this thing does
- **Tags** — flat list of canonical tags for scoping (e.g., `backend`, `auth`, `test`)
- **Content hash** — used to detect staleness when files change

Pointers are lightweight. They don't contain the full file content — just enough for acm to decide what's relevant and for agents to decide what to fetch.

## Receipt

When you call `get_context`, acm returns a receipt. A receipt is a scoped snapshot of everything relevant to your task. It contains:

- **Rules** — hard constraints the agent must follow
- **Suggestions** — code/doc/test pointers relevant to the task (advisory)
- **Memories** — durable facts from past work
- **Plans** — active work plans for the project, with task counts and fetch keys for resumption
- **Meta** — receipt ID, resolved tags, budget accounting

The receipt ID is used as a handle for all subsequent operations (`fetch`, `work`, `verify`, `report_completion`, `propose_memory`). It ties everything back to the original retrieval.

## Rule

A rule is a pointer with `kind: rule` that represents a behavioral constraint for agents. Rules have two extra properties:

- **Rule ID** — stable identifier (e.g., `rule_get_context_first`). Deterministic — survives re-syncs.
- **Enforcement** — `hard` (must follow) or `soft` (should follow)

Hard rules are always included with their full content in the receipt. Soft rules are summary-only and can be fetched on demand.

You author rules in `.acm/acm-rules.yaml` and sync them into acm. acm delivers the right rules at the right time based on task tags and phase.

## Memory

A memory is a durable fact learned from completed work. Unlike model-specific memory (Claude's memory, ChatGPT's memory), acm memories are:

- **Model-agnostic** — any agent on any model can read them
- **Evidence-backed** — each memory links to the pointer(s) that prove it
- **Categorized** — `decision`, `gotcha`, `pattern`, or `preference`
- **Confidence-scored** — 1 to 5, used for ranking during retrieval

Memories are proposed via `propose_memory`, validated, and either promoted to durable storage or held in quarantine for review.

## Plan

A plan is a durable work container that tracks what an agent is doing. Plans survive context compaction — when an agent loses its conversation history, `get_context` returns active plans with task counts and fetch keys, so the agent can resume where it left off.

Each plan has:

- **Plan key** — `plan:<receipt_id>` format, auto-derived from `receipt_id` if not provided
- **Title** — human-readable name
- **Objective** — what the plan aims to achieve
- **Kind** — free-form label (e.g., `bugfix`, `feature`)
- **Status** — `pending`, `in_progress`, `complete`, or `blocked`
- **Stages** — optional planning stages that track spec maturity:
  - `spec_outline` — initial high-level spec drafted
  - `refined_spec` — spec reviewed and refined with details
  - `implementation_plan` — concrete implementation steps defined
  Each stage has its own status (`pending`, `in_progress`, `complete`, `blocked`).
- **Scope** — in-scope/out-of-scope/constraints lists

Plans are created and updated via the `work` command. Updates can use `merge` mode (default, upserts tasks by key) or `replace` mode (replaces all tasks).

## Task

A task is a unit of work within a plan. Tasks have:

- **Key** — unique identifier within the plan (e.g., `implement-auth-refactor`, `verify:tests`)
- **Summary** — what needs to be done
- **Status** — `pending`, `in_progress`, `complete`, or `blocked`
- **Dependencies** — keys of other tasks this one depends on
- **Acceptance criteria** — conditions for completion
- **Outcome** — what happened (filled on completion)
- **Evidence** — references supporting the outcome

Tasks can reference a `parent_task_key` for grouping, and can be fetched individually via `fetch --key task:plan:<receipt-id>#<task-key>`.

Two special task keys are used for definition-of-done verification:
- `verify:tests` — confirms tests were run
- `verify:diff-review` — optional manual review task for diff inspection

## Tag

Tags are flat labels used to scope pointers, rules, and memories. Examples: `backend`, `auth`, `test`, `frontend`.

acm normalizes tags through a canonical dictionary that maps aliases to a single form (e.g., `api` and `server` both map to `backend`). When you call `get_context`, your task text is decomposed into 3-6 canonical tags, and acm uses those to find relevant pointers.

The tag dictionary has two layers:
- **Embedded base** — ships with acm (`internal/service/postgres/canonical_tags.json`), covers common aliases
- **Repo-local overrides** — `.acm/acm-tags.yaml` (auto-discovered), adds project-specific tags and aliases on top of the base

Use `--tags-file` on any command that does tag normalization to override discovery with an explicit path.

Tags replace the concept of "profiles" from other systems. They're simpler: just strings, no hierarchy, no inheritance.

## Phase

Every `get_context` call includes a phase that controls how results are weighted:

- **plan** — rules weighted highest, then docs, then code
- **execute** — code weighted highest, then tests, then rules
- **review** — rules weighted highest, then tests, then code

The phase doesn't filter results — it changes their ranking so the most useful pointers surface first.

## Retrieval Caps

`get_context` applies built-in defaults for how many pointers, memories, and relation hops to include. You can override these defaults per call:

| Flag | Default | Purpose |
|---|---|---|
| `--max-non-rule-pointers` | 8 | Maximum non-rule pointers returned |
| `--max-rule-pointers` | unbounded | Maximum rule pointers returned |
| `--max-hops` | 1 | Relation-hop expansion depth |
| `--max-hop-expansion` | +5 | Maximum additional pointers from hop expansion |
| `--max-memories` | 6 | Maximum memories returned |
| `--min-pointer-count` | 2 | Minimum pointers before triggering fallback |
| `--word-budget-limit` | 0 (off) | Optional word budget for returned content |

Additional flags:
- `--allow-stale` — include stale pointers (default: false)
- `--fallback-mode` — `widen_once` (default) or `none` for controlling what happens when too few pointers match
- `--unbounded` — remove all built-in retrieval caps

These are advanced tuning knobs. The defaults work well for most projects.

## Scope Mode

Controls how strictly acm enforces that agents stay within the retrieved context:

- **warn** (default) — violations are logged but the request is accepted
- **strict** — violations reject the request; the agent must re-retrieve
- **auto_index** — new files are automatically indexed instead of flagged as violations

Used in `report_completion` to validate that changed files were within the receipt's scope.

## Canonical Ruleset

The human-authored YAML file where you define your rules. acm discovers it automatically at `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` in the project root. Use `--rules-file` on `sync`, `health-fix`, or `bootstrap` to override with an explicit path. If you maintain a repo-local tag dictionary separately, use `--tags-file` on `get-context`, `propose-memory`, `report-completion`, `sync`, `health-fix`, `eval`, `verify`, or `bootstrap` to supply it explicitly for canonical tag normalization. Format:

```yaml
version: acm.rules.v1
rules:
  - id: rule_name
    summary: One-line description
    content: Full rule text (optional, defaults to summary)
    enforcement: hard|soft (optional, defaults to hard)
    tags: [tag1, tag2] (optional)
```

acm reads this file during `sync` or `health-fix --fixer sync_ruleset` and upserts the rules as pointers. The canonical ruleset is the source of truth — acm stores and delivers, the file defines.

## Executable Verification Definitions

The human-authored YAML file where you define repo-local executable checks for `verify`. acm discovers it automatically at `.acm/acm-tests.yaml` (preferred) or `acm-tests.yaml` in the project root. Use `--tests-file` on `verify` to override with an explicit path.

`eval` and `verify` solve different problems:

- `eval` checks retrieval quality against an eval suite.
- `verify` selects repo-defined executable checks from receipt context, phase, changed files, and optional explicit test ids, then runs them sequentially.

Format:

```yaml
version: acm.tests.v1

defaults:
  cwd: .
  timeout_sec: 300

tests:
  - id: go-unit
    summary: Run Go unit tests for ACM packages
    command:
      argv: ["go", "test", "./cmd/...", "./internal/..."]
      env:
        GOFLAGS: "-count=1"
    select:
      phases: ["execute", "review"]
      tags_any: ["backend"]
      changed_paths_any: ["cmd/**", "internal/**"]
    expected:
      exit_code: 0

  - id: smoke
    summary: Run repo smoke checks on every verification pass
    command:
      argv: ["go", "test", "./cmd/...", "./internal/..."]
    select:
      always_run: true
```

v1 test definitions are argv-only. They support optional `command.env` entries for repo-defined environment variables and `select.always_run: true` for default smoke checks that should auto-select. `verify` reuses the existing `verify:tests` task key for definition-of-done updates when work context is present. `verify:diff-review` is optional workflow metadata, not a built-in acm completion gate.

## File-Based Flags

Most CLI commands that accept text or list values support inline flags and file-based alternatives. JSON list/object inputs also support `--*-json` variants for one-shot calls without temp files:

| Inline flag | JSON inline | File alternative | Format |
|---|---|---|---|
| `--task-text` | - | `--task-file` | Plain text |
| `--content` | - | `--content-file` | Plain text |
| `--outcome` | - | `--outcome-file` | Plain text |
| `--key` (repeatable) | `--keys-json` | `--keys-file` | JSON string array |
| `--file-changed` (repeatable) | `--files-changed-json` | `--files-changed-file` | JSON string array |
| `--evidence-key` (repeatable) | `--evidence-keys-json` | `--evidence-keys-file` | JSON string array |
| `--related-key` (repeatable) | `--related-keys-json` | `--related-keys-file` | JSON string array |
| `--memory-tag` (repeatable) | `--memory-tags-json` | `--memory-tags-file` | JSON string array |
| `--expect` (repeatable) | `--expected-versions-json` | `--expected-versions-file` | JSON object (`{"key": "version"}`) |
| - | `--plan-json` | `--plan-file` | JSON object (work plan metadata) |
| - | `--tasks-json` | `--tasks-file` | JSON array of work tasks |
| - | `--items-json` | `--items-file` | JSON array of alternate work items |
| - | `--eval-suite-inline-json` | `--eval-suite-inline-file` | JSON array of eval cases |

All file flags accept `-` for stdin.

`--tags-file` is reserved for the canonical tag dictionary override on commands that do runtime tag normalization.
