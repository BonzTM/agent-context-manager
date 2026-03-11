# Shared Verify/Review Plumbing Refactor Plan

Purpose: define a maintainer-only staged plan for extracting neutral shared plumbing from `verify` and `review`.
Audience: the orchestrator agent and any delegated leaf-task agents executing this refactor.
Update when: the refactor scope, extraction seams, or file ownership changes.
Not for: product onboarding or user-facing command documentation.

Read [AGENTS.md](../../AGENTS.md), [docs/feature-plans.md](../feature-plans.md), and [docs/maintainer-map.md](../maintainer-map.md) first.

## Spec Outline

### Problem

`verify` and `review` are intentionally different commands, but they currently depend on overlapping runtime substrate:

- command execution helpers
- environment/context serialization for repo-local scripts
- changed-file detection and effective-scope helpers

The overlap is spread across:

- `internal/service/backend/verify.go`
- `internal/service/backend/review.go`
- `internal/service/backend/scope_runtime.go`
- `internal/service/backend/service.go`

That shape makes safe changes harder because one runtime-policy adjustment can require edits in multiple command files.

### Goal

Extract policy-neutral shared plumbing so `verify`, `review`, and future runner-backed commands can reuse:

1. common command execution helpers
2. common env/context serialization helpers
3. common changed-file normalization and scope-resolution helpers

### Hard Invariants

- Do not collapse `verify` and `review` into one product command.
- Do not change CLI or MCP payload shapes.
- Do not change the meaning of `.acm/acm-tests.yaml` or `.acm/acm-workflows.yaml`.
- Keep repo-local policy in repo-local scripts or config, not in new product helpers.
- Preserve current `done` behavior and current review fingerprint semantics.
- Preserve the current `ACM_VERIFY_*` and `ACM_REVIEW_*` environment variable names.

### Out Of Scope

- changing review workflow policy
- changing verify selection policy
- changing staged-plan rules
- changing starter templates
- adding new user-facing commands

## Refined Spec

### Extraction Area 1: Command execution helpers

Current shared substrate already exists, but it is stranded in `internal/service/backend/verify.go`:

- `verifyCommandRun`
- `runtimeCommandEnv`
- `runConfiguredCommand`
- `resolveConfiguredCommandArgv`
- `mergeCommandEnv`
- `verifyEnvPairs`

`review.go` already depends on that substrate indirectly through `runWorkflowReviewCommand`.

Refactor target:

- move the neutral runner substrate into a dedicated backend runtime file
- keep verify-specific classification and batching in `verify.go`
- keep review-specific attempt persistence and fingerprint logic in `review.go`

Suggested destination:

- `internal/service/backend/command_runtime.go`

### Extraction Area 2: Env/context serialization helpers

Current command-owned serializers:

- `verifyCommandEnvironment` in `internal/service/backend/verify.go`
- `verifyJSONListEnv` in `internal/service/backend/verify.go`
- `reviewCommandEnvironment` in `internal/service/backend/review.go`

Refactor target:

- one shared helper layer owns receipt/plan normalization and JSON-list encoding
- command files keep only the command-specific environment keys

Suggested destination:

- `internal/service/backend/command_context_env.go`

Candidate shared helpers:

- `normalizeReceiptPlanEnvIDs(...)`
- `jsonListEnv(...)`
- `commandBaseEnvironment(...)`
- `mergeCommandEnvironment(...)`

### Extraction Area 3: Changed-file normalization and scope resolution

Current helpers are split across:

- `normalizeCompletionPath` / `normalizeCompletionPaths` in `internal/service/backend/service.go`
- `detectReceiptChangedPaths`, `effectiveScopePaths`, `completionEffectiveScopePaths`, and related helpers in `internal/service/backend/scope_runtime.go`

Refactor target:

- keep path normalization central
- isolate receipt-baseline diffing behind one obvious helper surface
- keep review effective scope and completion effective scope separate and explicitly named

Suggested destination:

- keep `internal/service/backend/scope_runtime.go` as the entrypoint, but split into narrower files if it improves ownership:
  - `scope_delta.go`
  - `scope_effective_paths.go`

### Non-Negotiable Call-Site Behavior

- `verify` still updates `verify:tests` only after selected checks run.
- `review --run` still persists review attempts and honors fingerprint dedupe and `max_attempts`.
- `done` still validates scope and completion gates against the detected delta and completion effective scope.
- Review fingerprints must keep using the current effective-scope semantics.

### Main Risks

1. Shared-runner drift: moving helpers could change timeout, cwd, or env-precedence behavior.
2. Fingerprint drift: changing scope helper ordering or normalization could invalidate review dedupe behavior.
3. Over-unification: `review` and `done` intentionally do not use the same effective-scope helper.
4. Weak coverage: compile success may hide subtle env-serialization or path-normalization regressions.

### Safety Rules

- extract one helper family at a time
- migrate one call site at a time
- add focused regression tests before broad cleanup
- do not mix semantic behavior changes into the same leaf task as helper extraction

## Atomic Execution Tasks

Each task below is written for low-context execution. Hand one task to one agent. Do not merge tasks because they look related.

### Stage: spec-outline

#### Task `spec:inventory-runtime-seams`

- Goal: confirm the exact seams before code moves.
- References:
  - `internal/service/backend/verify.go`
  - `internal/service/backend/review.go`
  - `internal/service/backend/scope_runtime.go`
- Exact output:
  - list each shared helper or type that will move
  - list each helper that must stay command-specific
- Do not:
  - move product code
  - rename helpers
- Completion proof:
  - another agent can identify the destination file for each seam without re-reading the full codebase

### Stage: refined-spec

#### Task `refine:shared-runner-contract`

- Goal: freeze the shared runner API before extraction.
- References:
  - `internal/service/backend/verify.go`
  - `internal/service/backend/review.go`
  - `internal/service/backend/service_test.go`
- Exact decisions to record:
  - final shared result-type name
  - final shared helper names
  - env precedence order
  - whether shared helpers stay package-private
- Do not:
  - extract code
  - mix in unrelated cleanup
- Completion proof:
  - an implementation agent can create the shared runtime file without making new naming decisions

#### Task `refine:scope-helper-boundaries`

- Goal: freeze which scope helpers are shared and which remain command-specific.
- References:
  - `internal/service/backend/scope_runtime.go`
  - `internal/service/backend/completion.go`
  - `internal/service/backend/review.go`
- Exact decisions to record:
  - which helper owns receipt-baseline diffing
  - which helper owns review effective scope assembly
  - which helper owns completion effective scope assembly
  - which helpers must keep returning normalized sorted repo-relative paths
- Do not:
  - change behavior
  - rename command semantics
- Completion proof:
  - no implementation agent needs to guess whether `done` should call the same helper as `review`

### Stage: implementation-plan

#### Task `impl:add-shared-command-runtime-file`

- Goal: create the shared runner file without changing behavior.
- References:
  - `internal/service/backend/verify.go`
  - `internal/service/backend/review.go`
  - `internal/service/backend/service_test.go`
- Files allowed to edit:
  - `internal/service/backend/command_runtime.go`
  - `internal/service/backend/verify.go`
  - `internal/service/backend/review.go`
- Exact work:
  - move the neutral runner substrate out of `verify.go`
  - keep verify and review behavior identical
- Do not:
  - rename environment variables
  - change timeout defaults
  - change absolute-path resolution rules
- Completion proof:
  - the new file owns the generic command runner substrate
  - `verify.go` and `review.go` no longer hide the shared runner implementation

#### Task `impl:add-shared-command-env-file`

- Goal: extract neutral environment serialization helpers.
- References:
  - `internal/service/backend/verify.go`
  - `internal/service/backend/review.go`
  - `scripts/acm_verify_scripts_test.go`
- Files allowed to edit:
  - `internal/service/backend/command_context_env.go`
  - `internal/service/backend/verify.go`
  - `internal/service/backend/review.go`
- Exact work:
  - centralize receipt/plan normalization for env emission
  - centralize JSON list encoding for path and tag arrays
  - keep command-specific env assembly readable at the call site
- Do not:
  - add new environment variables
  - remove current environment variables
- Completion proof:
  - command files call shared env helpers instead of open-coding JSON serialization

#### Task `impl:extract-scope-delta-helper`

- Goal: isolate receipt-baseline delta calculation into one obvious helper surface.
- References:
  - `internal/service/backend/scope_runtime.go`
  - `internal/service/backend/verify.go`
  - `internal/service/backend/completion.go`
- Files allowed to edit:
  - `internal/service/backend/scope_runtime.go`
  - one new scope helper file if needed
  - related tests only
- Exact work:
  - keep receipt-baseline capture and diff logic together
  - expose one obvious helper for detected changed files
  - keep ordering normalized and deterministic
- Do not:
  - change git-versus-walk fallback behavior
  - change managed-path filtering
- Completion proof:
  - both `verify` and `done` use the same narrow delta helper instead of open-coding receipt diff concerns

#### Task `impl:extract-effective-scope-helpers`

- Goal: make review scope and completion scope helper boundaries explicit.
- References:
  - `internal/service/backend/scope_runtime.go`
  - `internal/service/backend/review.go`
  - `internal/service/backend/completion.go`
- Files allowed to edit:
  - `internal/service/backend/scope_runtime.go`
  - one new scope helper file if needed
  - related tests only
- Exact work:
  - keep separate helper names for review effective scope and completion effective scope
  - ensure both helpers rely on the same path-normalization layer
- Do not:
  - erase the intentional `plan.in_scope` behavior for `done`
- Completion proof:
  - a new reader can tell from helper names why `review` and `done` do not use identical scope assembly

#### Task `impl:migrate-verify-call-sites`

- Goal: switch `verify.go` fully onto the shared helpers.
- References:
  - `internal/service/backend/verify.go`
  - `internal/service/backend/command_runtime.go`
  - `internal/service/backend/command_context_env.go`
- Files allowed to edit:
  - `internal/service/backend/verify.go`
  - related tests only
- Exact work:
  - replace inline runner and env plumbing with shared helpers
  - keep verify selection, result classification, and verify-work updates local
- Do not:
  - rewrite verify selection behavior
  - change evidence or batch-save semantics
- Completion proof:
  - `verify.go` reads as verify-specific orchestration, not runtime plumbing

#### Task `impl:migrate-review-call-sites`

- Goal: switch `review.go` fully onto the shared helpers.
- References:
  - `internal/service/backend/review.go`
  - `internal/service/backend/command_runtime.go`
  - `internal/service/backend/command_context_env.go`
- Files allowed to edit:
  - `internal/service/backend/review.go`
  - related tests only
- Exact work:
  - replace inline runner and env plumbing with shared helpers
  - keep attempt persistence, fingerprint rules, and review-task updates local
- Do not:
  - rewrite review fingerprint meaning
  - change rerun or dedupe behavior
- Completion proof:
  - `review.go` reads as workflow-gate orchestration, not runtime infrastructure

#### Task `impl:add-focused-regression-tests`

- Goal: add direct coverage for the extracted helper families.
- References:
  - `internal/service/backend/service_test.go`
  - `scripts/acm_verify_scripts_test.go`
  - `scripts/acm_cross_review_test.go`
- Files allowed to edit:
  - existing test files
  - one new backend helper test file if that is cleaner
- Exact work:
  - add direct tests for env serialization helpers
  - add direct tests for normalized changed-path output
  - add direct tests for runner env precedence if current coverage is indirect only
- Do not:
  - mix in unrelated test cleanup
- Completion proof:
  - shared-helper regressions fail in focused tests before they surface through `verify` or `review`

#### Task `impl:maintainer-routing-pass`

- Goal: update maintainer routing only if file ownership changed materially.
- References:
  - `docs/maintainer-map.md`
  - `docs/maintainer-reference.md`
  - `AGENTS.md`
- Files allowed to edit:
  - maintainer docs only
- Exact work:
  - add the new shared backend files to maintainer routing if they were created
  - keep `AGENTS.md` fast-path sized
- Do not:
  - rewrite product-facing docs unless product behavior changed
- Completion proof:
  - future maintainers can find the new helper files without rediscovering the refactor

#### Task `verify:tests`

- Goal: run repo-defined verification for the refactor.
- References:
  - `.acm/acm-tests.yaml`
  - `scripts/acm-go-test-targets.py`
  - `scripts/acm-tdd-guard.py`
- Required commands:
  - targeted Go package tests for touched backend packages
  - `go test ./... -count=1`
  - `acm verify` with the active receipt or plan context
- Completion proof:
  - verification passes and updates `verify:tests`

#### Task `review:cross-llm`

- Goal: run the repo’s runnable review gate after implementation stabilizes.
- References:
  - `.acm/acm-workflows.yaml`
  - `scripts/acm-cross-review.sh`
  - `internal/service/backend/review.go`
- Required command:
  - `acm review --run`
- Completion proof:
  - the review gate passes for the current fingerprint

## Suggested ACM Root Plan Payload

Use this only when the refactor is actually being executed. Do not pre-open the work item if the refactor is still hypothetical.

```json
{
  "plan": {
    "title": "Extract shared verify/review command runtime plumbing",
    "kind": "maintenance",
    "objective": "Separate policy-neutral runtime helpers from verify/review-specific behavior so future changes land in one place without blurring command semantics.",
    "status": "pending",
    "in_scope": [
      "shared command execution helpers",
      "shared env/context serialization helpers",
      "shared changed-file and effective-scope helpers"
    ],
    "out_of_scope": [
      "command API changes",
      "workflow policy changes",
      "template changes"
    ],
    "constraints": [
      "Keep verify and review semantically distinct",
      "Do not rename current ACM_VERIFY_* or ACM_REVIEW_* env variables",
      "Keep done and review fingerprint behavior stable"
    ],
    "references": [
      "internal/service/backend/verify.go",
      "internal/service/backend/review.go",
      "internal/service/backend/scope_runtime.go"
    ],
    "stages": {
      "spec_outline": "pending",
      "refined_spec": "pending",
      "implementation_plan": "pending"
    }
  },
  "tasks": [
    {
      "key": "stage:spec-outline",
      "summary": "Spec outline",
      "status": "pending"
    },
    {
      "key": "spec:inventory-runtime-seams",
      "summary": "Inventory the shared verify/review runtime seams",
      "status": "pending",
      "parent_task_key": "stage:spec-outline",
      "references": [
        "internal/service/backend/verify.go",
        "internal/service/backend/review.go",
        "internal/service/backend/scope_runtime.go"
      ],
      "acceptance_criteria": [
        "Shared versus command-specific helpers are explicitly listed",
        "The destination file for each shared seam is specified"
      ]
    },
    {
      "key": "stage:refined-spec",
      "summary": "Refined spec",
      "status": "pending"
    },
    {
      "key": "refine:shared-runner-contract",
      "summary": "Freeze the shared runner API before extraction",
      "status": "pending",
      "parent_task_key": "stage:refined-spec",
      "references": [
        "internal/service/backend/verify.go",
        "internal/service/backend/review.go",
        "internal/service/backend/service_test.go"
      ],
      "acceptance_criteria": [
        "The shared runner helper names and ownership are fixed",
        "Env precedence and symbol visibility decisions are fixed"
      ]
    },
    {
      "key": "refine:scope-helper-boundaries",
      "summary": "Freeze the review versus completion scope helper boundaries",
      "status": "pending",
      "parent_task_key": "stage:refined-spec",
      "references": [
        "internal/service/backend/scope_runtime.go",
        "internal/service/backend/completion.go",
        "internal/service/backend/review.go"
      ],
      "acceptance_criteria": [
        "Receipt delta, review scope, and completion scope helpers are assigned separate ownership",
        "No implementation agent needs to infer which helper each command should call"
      ]
    },
    {
      "key": "stage:implementation-plan",
      "summary": "Implementation plan",
      "status": "pending"
    },
    {
      "key": "impl:add-shared-command-runtime-file",
      "summary": "Extract the generic command runner substrate",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "refine:shared-runner-contract"
      ],
      "references": [
        "internal/service/backend/verify.go",
        "internal/service/backend/review.go",
        "internal/service/backend/service_test.go"
      ],
      "acceptance_criteria": [
        "A shared backend file owns the neutral command runner substrate",
        "Verify and review behavior remain unchanged"
      ]
    },
    {
      "key": "impl:add-shared-command-env-file",
      "summary": "Extract neutral command env/context serialization helpers",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "refine:shared-runner-contract"
      ],
      "references": [
        "internal/service/backend/verify.go",
        "internal/service/backend/review.go",
        "scripts/acm_verify_scripts_test.go"
      ],
      "acceptance_criteria": [
        "Shared helpers own receipt/plan normalization and JSON list encoding",
        "Current ACM_VERIFY_* and ACM_REVIEW_* variables remain unchanged"
      ]
    },
    {
      "key": "impl:extract-scope-delta-helper",
      "summary": "Narrow receipt-baseline changed-file detection into one shared helper surface",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "refine:scope-helper-boundaries"
      ],
      "references": [
        "internal/service/backend/scope_runtime.go",
        "internal/service/backend/verify.go",
        "internal/service/backend/completion.go"
      ],
      "acceptance_criteria": [
        "Receipt-baseline delta detection has one obvious shared owner",
        "Changed-path ordering stays normalized and deterministic"
      ]
    },
    {
      "key": "impl:extract-effective-scope-helpers",
      "summary": "Make review and completion scope helper boundaries explicit",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "refine:scope-helper-boundaries"
      ],
      "references": [
        "internal/service/backend/scope_runtime.go",
        "internal/service/backend/review.go",
        "internal/service/backend/completion.go"
      ],
      "acceptance_criteria": [
        "Review and completion scope helpers are explicit and separately named",
        "Shared path normalization remains central"
      ]
    },
    {
      "key": "impl:migrate-verify-call-sites",
      "summary": "Switch verify onto the shared helpers",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "impl:add-shared-command-runtime-file",
        "impl:add-shared-command-env-file",
        "impl:extract-scope-delta-helper"
      ],
      "references": [
        "internal/service/backend/verify.go",
        "internal/service/backend/command_runtime.go",
        "internal/service/backend/command_context_env.go"
      ],
      "acceptance_criteria": [
        "verify.go reads as verify-specific orchestration instead of runtime plumbing",
        "Verify behavior and evidence updates remain unchanged"
      ]
    },
    {
      "key": "impl:migrate-review-call-sites",
      "summary": "Switch review onto the shared helpers",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "impl:add-shared-command-runtime-file",
        "impl:add-shared-command-env-file",
        "impl:extract-effective-scope-helpers"
      ],
      "references": [
        "internal/service/backend/review.go",
        "internal/service/backend/command_runtime.go",
        "internal/service/backend/command_context_env.go"
      ],
      "acceptance_criteria": [
        "review.go reads as workflow-gate orchestration instead of runtime plumbing",
        "Review attempt persistence and fingerprint behavior remain unchanged"
      ]
    },
    {
      "key": "impl:add-focused-regression-tests",
      "summary": "Add direct tests for the extracted shared helpers",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "impl:migrate-verify-call-sites",
        "impl:migrate-review-call-sites"
      ],
      "references": [
        "internal/service/backend/service_test.go",
        "scripts/acm_verify_scripts_test.go",
        "scripts/acm_cross_review_test.go"
      ],
      "acceptance_criteria": [
        "Shared helper regressions fail in focused tests",
        "The test suite covers env serialization, path normalization, and runner behavior"
      ]
    },
    {
      "key": "impl:maintainer-routing-pass",
      "summary": "Update maintainer routing for any new shared backend files",
      "status": "pending",
      "parent_task_key": "stage:implementation-plan",
      "depends_on": [
        "impl:add-focused-regression-tests"
      ],
      "references": [
        "docs/maintainer-map.md",
        "docs/maintainer-reference.md",
        "AGENTS.md"
      ],
      "acceptance_criteria": [
        "Maintainer docs route future runtime changes to the new helper files",
        "AGENTS.md stays fast-path sized"
      ]
    },
    {
      "key": "verify:tests",
      "summary": "Run verification for the refactor",
      "status": "pending",
      "acceptance_criteria": [
        "Repo-defined verification passes for the active refactor receipt or plan",
        "go test ./... -count=1 passes"
      ]
    },
    {
      "key": "review:cross-llm",
      "summary": "Run the runnable cross-LLM review gate for the current fingerprint",
      "status": "pending",
      "acceptance_criteria": [
        "acm review --run passes for the active fingerprint"
      ]
    }
  ]
}
```
