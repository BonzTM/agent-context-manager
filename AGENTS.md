# AGENTS.md - agent-context-manager Maintainer Contract

IMPORTANT: This file is the repository-local operating contract for work in `agent-context-manager`.
Read it before doing substantive work in this repo. After any compaction, reset, or handoff, re-read this file and restart the ACM task loop before resuming.

## Source Of Truth

- This file is the top-level maintainer contract for this repository.
- Canonical ACM rules live only in `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` at the repo root.
- Canonical ACM tags live in `.acm/acm-tags.yaml`.
- Canonical ACM verification definitions live in `.acm/acm-tests.yaml` (preferred) or `acm-tests.yaml` at the repo root.
- Canonical ACM workflow gate definitions live in `.acm/acm-workflows.yaml` (preferred) or `acm-workflows.yaml` at the repo root.
- `docs/examples/AGENTS.md` and `docs/examples/CLAUDE.md` are generic starter templates, not the contract for maintaining this repo.
- Tool-specific companions such as `CLAUDE.md` and skill-pack prompts must stay consistent with this file. If they disagree, this file wins.

## Required Startup Order

Complete this sequence before making code changes or giving repo-specific implementation guidance:

1. Read `AGENTS.md`.
2. If the active tool is Claude, read `CLAUDE.md`.
3. Run `acm context --project agent-context-manager --task-text "<current task>" --phase <plan|execute|review>`.
4. Read the returned hard rules and fetch only the keys needed for the current step.
5. If the task is multi-step, multi-file, or handoff-prone, create or update `work` immediately.
6. If you change code, config, contracts, onboarding, or other executable behavior, run `verify` before `done`.
7. If `.acm/acm-workflows.yaml` requires a review task such as `review:cross-llm`, satisfy it with `acm review --run ...` when the task defines a `run` block; otherwise use manual `review` fields or `work`.
8. End the task with `done`, including every changed file or letting ACM auto-detect the task delta when possible, plus a concise outcome.
9. Capture durable decisions or recurring pitfalls with `memory`.

If `acm` is not on `PATH`, fix the environment or invoke the installed binary directly. Do not substitute `go run ./cmd/acm` for normal repository workflow unless you are explicitly testing source-build behavior.

## Required ACM Task Loop

Use these commands as the default flow for this repo:

1. `acm context --project agent-context-manager ...`
2. `acm work --project agent-context-manager ...` when the work is not trivially one-step or when discovered file scope must be declared
3. `acm verify --project agent-context-manager ...` for executable changes
4. `acm review --run --project agent-context-manager ...` when a workflow gate requires it
5. `acm done --project agent-context-manager ...`
6. `acm memory --project agent-context-manager ...` when the result should survive future sessions

Use `acm fetch` only when a receipt or plan returned a key you actually need to hydrate. Do not skip `work` for multi-step changes or when governed file scope expands beyond the initial receipt. Do not skip `verify` for executable changes. Do not treat `done` as optional.

## Repository-Specific Rules

Keep these constraints front-of-mind. They are derived from `.acm/acm-rules.yaml` and are expected on every relevant change:

- Contract and schema lockstep:
  Any payload, result, validation, or command semantics change must update `internal/contracts/v1`, `spec/v1`, and the relevant tests in the same change.
- CLI and MCP parity:
  Command-surface changes must be reflected across `cmd/acm`, `cmd/acm-mcp`, MCP tool definitions, help output, and tests.
- Storage parity:
  Postgres and SQLite behavior must stay aligned unless a backend-specific divergence is intentional and documented. Update both adapters, migrations, and parity tests together.
- Onboarding invariants:
  `init` must leave a clean repo in a usable first-run state. Do not let ACM-managed runtime files or SQLite sidecars leak into indexing or health findings.
- Docs, examples, and skills sync:
  User-facing command, onboarding, install, or workflow changes must update `README.md`, `docs/getting-started.md`, `docs/examples`, and `skills/acm-broker/**` in the same change.
- Verification before completion:
  Code, config, contract, onboarding, and behavior changes require `verify` evidence before `done`.
- Workflow gates:
  `done` may enforce additional required work item keys from `.acm/acm-workflows.yaml`; keep workflow definitions aligned with actual task keys and docs, and prefer `acm review --run` when a required review gate defines a `run` block.
- Durable decisions:
  If a decision or pitfall would save future agent time, record it with `memory`.

## When To Use work

Create or update `work` when any of the following are true:

- the task takes more than one material step
- more than one file or subsystem is involved
- the task includes explicit planning, verification, or handoff
- you need durable state that should survive compaction or session reset

For code or behavior changes, include `verify:tests`. Add other tasks only when they help execution or resumption.
For single review-gate updates, `review` is the thinner convenience wrapper around `work`; use `review --run` for runnable workflow gates and reserve manual `status` / `outcome` / `evidence` fields for non-run mode.

## Feature Plans

Use the richer ACM feature plan contract for net-new feature work and large capability expansions in this repo.

- Create a root ACM plan with `kind=feature` before implementation.
- Root feature plans must include `objective`, `in_scope`, `out_of_scope`, `constraints`, `references`, and stage statuses for `spec_outline`, `refined_spec`, and `implementation_plan`.
- Root feature plans must include top-level `stage:spec-outline`, `stage:refined-spec`, and `stage:implementation-plan` tasks. Put concrete child tasks beneath them with `parent_task_key`.
- Atomic tasks in this contract are leaf tasks: tasks with no children. Leaf tasks must carry explicit `acceptance_criteria`.
- If a feature splits into multiple execution streams, create child plans with `kind=feature_stream` and `parent_plan_key=<root plan key>`.
- Feature and feature-stream plans must carry `verify:tests`.
- `acm verify` selects `acm-feature-plan-validate` for feature-relevant work and runs `scripts/acm-feature-plan-validate.py` with the active receipt or plan context.
- The validator enforces the schema for `kind=feature` and `kind=feature_stream` plans and exits cleanly for other plan kinds or receipt contexts that do not materialize a concrete plan.
- See `docs/feature-plans.md` for examples and command shapes.

## Verification Expectations

- `.acm/acm-tests.yaml` is the repo-local verification contract.
- Current baseline:
  - `smoke` always runs
  - `cli-build` runs in `execute` and `review` when `cmd/**`, `internal/**`, `go.mod`, or `go.sum` changes
  - `full-go-suite` runs in `review` when `cmd/**`, `internal/**`, `spec/**`, `skills/**`, `scripts/**`, `go.mod`, or `go.sum` changes
- If your change is not adequately covered, update `.acm/acm-tests.yaml` before claiming completion.
- For broad product changes, expect `go test ./...` to run through `verify` or as explicit supplemental validation.
- For changes that match the repo-local review workflow gate, run `acm review --run --project agent-context-manager --receipt-id <receipt-id>` before `done`.

## Governance And Maintenance Changes

When changing ACM governance, onboarding, or tool-surface behavior:

1. Update the relevant repo-local ACM files and product code.
2. Run `acm sync --project agent-context-manager --mode working_tree --insert-new-candidates`.
3. Run `acm health --project agent-context-manager --include-details`.
4. Update docs and skill-pack assets in the same change when behavior or workflow changed.

When rules, tags, or tests changed but you only need to refresh ACM-managed state, `acm health --project agent-context-manager --apply` is also acceptable if it covers the needed fixers. Re-run `acm health` afterward.

When changing rules, tags, or tests specifically, keep these files coherent:

- `.acm/acm-rules.yaml`
- `.acm/acm-tags.yaml`
- `.acm/acm-tests.yaml`
- `.acm/acm-workflows.yaml`
- `README.md`
- `docs/getting-started.md`
- `docs/examples/AGENTS.md`
- `docs/examples/CLAUDE.md`
- `skills/acm-broker/**`

## Repository Notes

- `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `.acm/acm-tests.yaml`, and `.acm/acm-workflows.yaml` are tracked product inputs.
- `.acm/context.db`, `.acm/context.db-shm`, and `.acm/context.db-wal` are local runtime state and should stay ignored.
- Prefer small, reviewable changes over broad cleanup.
- Do not invent compatibility promises, migration behavior, or product requirements that the repo does not define.
- If verification fails, fix the issue or report the failure clearly. Do not claim success as if checks passed.
