# Proposal: Executable Verification v1 for acm

> Accepted v1 contract proposal. This document defines the intended public surface and storage model for executable verification.

- Status: Accepted
- Date: 2026-03-05
- Owners: Human operator + agent maintainers
- Scope: Project-local executable verification definitions, deterministic selection, execution, and durable verification run summaries

## Summary

acm should expose two separate verification surfaces:

- `acm eval`: retrieval-quality evaluation against an eval suite
- `acm verify`: executable verification against repo-defined test commands

`eval` answers whether acm retrieved the right context. `verify` answers whether the code and workflow changes satisfy project-defined executable checks.

This proposal keeps those concerns separate, reuses `work` for definition-of-done state, and adds durable verification run storage without inventing a second planning system.

## Decisions

1. Public command naming is `eval` and `verify`. Do not ship `regress` as a public command name.
2. Executable verification definitions live in `.acm/acm-tests.yaml` (preferred) or `acm-tests.yaml` in the repo root.
3. v1 test commands are `argv` only. Raw shell strings are out of scope.
4. `verify` selection is deterministic and sequential.
5. `verify` updates `verify:tests` by default when `receipt_id` or `plan_key` is provided.
6. `verify` persists durable batch/result summaries with concise excerpts, not full logs.
7. `get_context` does not return verification suggestions in v1.

## Goals

- Keep retrieval evaluation and executable verification separate.
- Let projects define reusable executable checks in the repo.
- Select verification definitions deterministically from receipt context, phase, changed files, and explicit test ids.
- Persist machine-usable verification run summaries.
- Reuse the existing `work`/`verify:tests` model instead of introducing parallel task tracking.

## Non-Goals

- Replacing native runners such as `go test`, `pytest`, or `npm test`
- Full shell-script support in v1
- Rich output assertion DSL in v1
- Returning verification suggestions in `get_context`
- Storing full stdout/stderr logs inside acm

## Repo Contract

### File Discovery

acm discovers verification definitions from exactly one source:

1. `.acm/acm-tests.yaml` if present
2. otherwise `acm-tests.yaml` in the repo root
3. or an explicit `--tests-file` / `tests_file` override

Unlike rules, test definitions are not merged across multiple files in v1.

### File Format

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

  - id: retrieval-eval
    summary: Run retrieval evaluation for auth-sensitive flows
    command:
      argv: ["acm", "eval", "--project", "myproject", "--eval-suite-path", ".acm/eval/auth.json", "--minimum-recall", "0.8"]
      timeout_sec: 120
    select:
      phases: ["review"]
      tags_any: ["auth", "retrieval"]
    expected:
      exit_code: 0

  - id: smoke
    summary: Run repo smoke checks on every verification pass
    command:
      argv: ["go", "test", "./cmd/...", "./internal/..."]
    select:
      always_run: true
    expected:
      exit_code: 0
```

### Field Semantics

- `version`: required, must be `acm.tests.v1`
- `defaults.cwd`: optional repo-relative working directory, default `.`
- `defaults.timeout_sec`: optional default timeout, default `300`
- `tests[].id`: required stable identifier, unique within the file
- `tests[].summary`: required human-readable label
- `tests[].command.argv`: required non-empty argv vector
- `tests[].command.cwd`: optional repo-relative working directory override
- `tests[].command.timeout_sec`: optional timeout override
- `tests[].command.env`: optional repo-defined environment variables applied only to that test command
- `tests[].select.phases`: optional allowed phases
- `tests[].select.tags_any`: optional canonical tag intersection selector
- `tests[].select.changed_paths_any`: optional repo-relative glob selectors
- `tests[].select.pointer_keys_any`: optional receipt pointer key intersection selector
- `tests[].select.always_run`: optional boolean for smoke checks that should auto-select on every verify call
- `tests[].expected.exit_code`: optional expected exit code, default `0`

There is no `record_as.work_item_key` in v1. `verify` updates `verify:tests` at the batch level when work context is present.

## Command Contract

### `acm eval`

`eval` is the public name for retrieval evaluation. It replaces `regress` as the user-facing command.

### `acm verify`

CLI shape:

```bash
acm verify [--project <id>] \
  [--receipt-id <id>] \
  [--plan-key <key>] \
  [--phase <plan|execute|review>] \
  [--test-id <id>]... \
  [--file-changed <path>]... \
  [--files-changed-file <path>|--files-changed-json <json>] \
  [--tests-file <path>] \
  [--tags-file <path>] \
  [--dry-run]
```

MCP exposes a `verify` tool with the same payload semantics.

### Verify Input Semantics

- `project_id`: optional when runtime defaults are configured; explicit values override `ACM_PROJECT_ID` and repo-root inference
- `receipt_id`: optional receipt context source
- `plan_key`: optional plan context / work update target
- `phase`: optional explicit phase override; otherwise use receipt phase when available
- `test_ids`: optional explicit test selection; preserves caller order
- `files_changed`: optional changed files used by path selectors
- `tests_file`: optional explicit tests definition file override
- `tags_file`: optional canonical tag dictionary override used to normalize `select.tags_any`
- `dry_run`: optional; select but do not execute or persist runs

If `test_ids` is omitted, callers must provide enough selection context to avoid accidental broad execution. In v1 that means at least one of:

- `receipt_id`
- `plan_key`
- `phase`
- `files_changed`

Otherwise `verify` returns `INVALID_INPUT`.

## Selection Rules

Selection is deterministic.

If `test_ids` is provided:

- select exactly those tests in caller order
- fail validation on unknown ids

If `test_ids` is omitted:

1. Resolve receipt context when `receipt_id` is available.
2. Derive effective phase from explicit `phase`, else receipt phase.
3. Match candidate tests against available selectors.

Selector groups:

- `phases`: matches when effective phase is present and listed
- `tags_any`: matches when any selector tag intersects resolved receipt tags
- `changed_paths_any`: matches when any changed file matches any configured glob
- `pointer_keys_any`: matches when any configured pointer key is inside receipt scope
- `always_run`: auto-selects the test on every verify run; it must not be combined with other selector groups

If a test defines multiple selector groups, all present groups must match.

Results are sorted by:

1. explicit `test_id` order when provided
2. otherwise lexical `id`

v1 executes selected tests sequentially.

## Output Contract

`verify` returns:

```json
{
  "status": "failed",
  "batch_run_id": "verify-20260305-abc123",
  "selected_test_ids": ["go-unit", "retrieval-eval"],
  "selected": [
    {
      "test_id": "go-unit",
      "summary": "Run Go unit tests for ACM packages",
      "selection_reasons": ["phase=review", "changed_paths_any matched internal/service/postgres/service.go"]
    }
  ],
  "passed": false,
  "results": [
    {
      "test_id": "go-unit",
      "status": "passed",
      "definition_hash": "sha256:...",
      "exit_code": 0,
      "duration_ms": 18234
    },
    {
      "test_id": "retrieval-eval",
      "status": "failed",
      "definition_hash": "sha256:...",
      "exit_code": 1,
      "duration_ms": 814,
      "stderr_excerpt": "aggregate recall below threshold"
    }
  ]
}
```

Top-level `status` values:

- `dry_run`
- `no_tests_selected`
- `passed`
- `failed`

Per-test `status` values:

- `passed`
- `failed`
- `timed_out`
- `errored`
- `skipped`

`dry_run` returns selected tests with no `batch_run_id` and no executed results.
`no_tests_selected` returns an empty selection and does not execute or persist anything.

`selected[].selection_reasons` is required so operators and agents can audit why a test was selected.

## Work Integration

When `receipt_id` or `plan_key` is present and `verify` actually executes tests:

- update `verify:tests` through the existing `work` pathway
- set `status=complete` when all selected tests pass
- set `status=blocked` when any selected test fails, times out, or errors
- write a concise batch outcome summary
- attach evidence entries referencing the batch run id and per-test ids

`verify` does not create a parallel work model.

## Storage Contract

Persist durable verification data in dedicated tables.

### Batch Record

Minimal batch fields:

- `batch_run_id`
- `project_id`
- `receipt_id`
- `plan_key`
- `phase`
- `tests_source_path`
- `status`
- `passed`
- `selected_test_ids`
- `created_at`

### Per-Test Result Record

Minimal result fields:

- `batch_run_id`
- `project_id`
- `test_id`
- `definition_hash`
- `summary`
- `command_argv`
- `command_cwd`
- `timeout_sec`
- `expected_exit_code`
- `selection_reasons`
- `status`
- `exit_code`
- `duration_ms`
- `stdout_excerpt`
- `stderr_excerpt`
- `started_at`
- `finished_at`

This is enough for auditability, work evidence, and future fetch/reporting surfaces.

## V1 Constraints

- `argv` only, no raw shell strings
- sequential execution only
- `expected.exit_code` only
- concise stdout/stderr excerpts only
- no `get_context` integration

## Implementation Notes

- Treat `eval` as the public replacement for `regress`
- Keep selection deterministic and bounded
- Normalize `select.tags_any` through the same canonical tag system used elsewhere
- Use existing `work` persistence instead of building per-test task mapping

## Recommendation

Ship this as a narrow v1:

- `eval` for retrieval evaluation
- `verify` for repo-defined executable verification
- durable batch/result summaries
- automatic `verify:tests` updates through `work`

That closes the real product gap without conflating retrieval quality, executable verification, and planning.
