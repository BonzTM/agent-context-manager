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
- **Plans** — active work items and their status
- **Meta** — receipt ID, resolved tags, budget accounting

The receipt ID is used as a handle for all subsequent operations (`fetch`, `work`, `report_completion`, `propose_memory`). It ties everything back to the original retrieval.

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

## Work Item

A work item is a stateful task tracked by acm. Work items have:

- **Key** — unique identifier (e.g., `implement-auth-refactor`, `verify:tests`)
- **Summary** — what needs to be done
- **Status** — `pending`, `in_progress`, `complete`, or `blocked`
- **Outcome** — what happened (filled on completion)

Work items are grouped under a plan (identified by `plan_key`, auto-generated from `receipt_id` if not provided). They survive context compaction — when an agent loses its conversation history, it can call `get_context` and see active plans in the receipt, then `fetch` to get full details.

Two special work item keys are used for definition-of-done verification:
- `verify:tests` — confirms tests were run
- `verify:diff-review` — confirms the diff was reviewed for unintended changes

## Tag

Tags are flat labels used to scope pointers, rules, and memories. Examples: `backend`, `auth`, `test`, `frontend`.

acm maintains a canonical tag dictionary that normalizes aliases (e.g., `api` and `server` both map to `backend`). When you call `get_context`, your task text is decomposed into 3-6 canonical tags, and acm uses those to find relevant pointers.

Tags replace the concept of "profiles" from other systems. They're simpler: just strings, no hierarchy, no inheritance.

## Phase

Every `get_context` call includes a phase that controls how results are weighted:

- **plan** — rules weighted highest, then docs, then code
- **execute** — code weighted highest, then tests, then rules
- **review** — rules weighted highest, then tests, then code

The phase doesn't filter results — it changes their ranking so the most useful pointers surface first.

## Scope Mode

Controls how strictly acm enforces that agents stay within the retrieved context:

- **warn** (default) — violations are logged but the request is accepted
- **strict** — violations reject the request; the agent must re-retrieve
- **auto_index** — new files are automatically indexed instead of flagged as violations

Used in `report_completion` to validate that changed files were within the receipt's scope.

## Canonical Ruleset

The human-authored YAML file where you define your rules. acm discovers it automatically at `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` in the project root. Use `--rules-file` on `sync`, `health-fix`, or `bootstrap` to override with an explicit path. Format:

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
| `--tag` (repeatable) | `--tags-json` | `--tags-file` | JSON string array |
| `--expect` (repeatable) | `--expected-versions-json` | `--expected-versions-file` | JSON object (`{"key": "version"}`) |
| - | `--plan-json` | `--plan-file` | JSON object (work plan metadata) |
| - | `--tasks-json` | `--tasks-file` | JSON array of work tasks |
| - | `--items-json` | `--items-file` | JSON array of legacy work items |
| - | `--eval-suite-inline-json` | `--eval-suite-inline-file` | JSON array of regress cases |

All file flags accept `-` for stdin.
