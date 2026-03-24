# ACM Agent Workflow — agent-context-manager

This extends [AGENTS.md](../AGENTS.md) with the ACM-managed workflow for agents that have `acm` available. All invariants and routing from AGENTS.md still apply.

See [acm-work-loop.md](acm-work-loop.md) for the full command reference.

## Source Of Truth

- Canonical rules: `.acm/acm-rules.yaml`
- Canonical tags: `.acm/acm-tags.yaml`
- Canonical verification: `.acm/acm-tests.yaml`
- Canonical workflow gates: `.acm/acm-workflows.yaml`
- `docs/examples/` contains generic starter templates, not this repo's contract.

## Fast Path

1. Read `AGENTS.md` and any tool-specific companion (e.g. `CLAUDE.md`).
2. If you need orientation before reading code, run `acm status --project agent-context-manager --task-text "<task>" --phase <plan|execute|review>`.
3. For non-trivial work (multi-step, multi-file, or governed), run `acm context --project agent-context-manager --task-text "<task>" --phase <plan|execute|review>`. Trivial single-file fixes can skip the ACM ceremony.
4. Follow the returned hard rules. Use `acm fetch` only for keys you need.
5. Run `acm work` for multi-step, multi-file, handoff-prone, or governed-scope-expanding work.
6. Run `acm verify` for code, config, contract, onboarding, or behavior changes.
7. Run `acm review --run` when `.acm/acm-workflows.yaml` requires a runnable review gate.
8. Run `acm done` for non-trivial work.

If `acm` is not on `PATH`, bootstrap the toolchain before continuing:

```bash
# 1. Ensure Go is available (requires sudo for /usr/local install)
if ! command -v go &>/dev/null; then
  curl -fsSL https://go.dev/dl/go1.26.0.linux-amd64.tar.gz -o /tmp/go.tar.gz \
    && sudo tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz
  export PATH="/usr/local/go/bin:$PATH"
fi

# 2. Build and install acm binaries from this repo
if ! command -v acm &>/dev/null; then
  go build -o /tmp/acm ./cmd/acm \
    && go build -o /tmp/acm-mcp ./cmd/acm-mcp \
    && sudo mv /tmp/acm /tmp/acm-mcp /usr/local/bin/
fi
```

Do not substitute `go run ./cmd/acm` unless you are explicitly testing source-build behavior.

When changing rules, tags, tests, workflows, onboarding, or tool-surface behavior, run `acm sync --project agent-context-manager --mode working_tree --insert-new-candidates` and `acm health --project agent-context-manager --include-details` before `done`.

## ACM-Specific Working Norms

- Do not silently widen governed scope. Re-run `context` or declare later-discovered files through `work.plan.discovered_paths`.
- `acm health` and `acm status` warnings about stale plans, plan-status drift, or plans left open only for administrative closeout are bookkeeping regressions. Clean them up before `done` when they are part of your task.
- Governed multi-step work in this repo uses the staged plan contract in `docs/feature-plans.md`, not thin ad hoc task lists.
- Governed root plans must always carry `spec_outline`, `refined_spec`, and `implementation_plan`; do not start implementation with only a vague root objective.
- The root plan owner acts as the orchestrator for multi-file or multi-step work: keep the whole-plan spec, scope, verification, review, and closeout there; keep leaf tasks narrow enough for low-context execution.
- When the runtime supports sub-agents, prefer delegating bounded leaf tasks so the orchestrator keeps the full-plan context. When it does not, execute the same leaf tasks sequentially and return to the root plan between them.
- Keep leaf tasks so tight that an assignee can succeed from the listed `references`, `acceptance_criteria`, and `depends_on` edges without inventing missing scope.
- If governed scope expands, declare new files through `acm work` before `acm review` or `acm done`.

## Skill Aliases

Tools with the ACM skill pack installed expose these shorthand commands:

| ACM CLI | Skill alias |
|---|---|
| `acm context` | `/acm-context [phase] <task>` |
| `acm work` | `/acm-work` |
| `acm verify` | `/acm-verify` |
| `acm review --run` | `/acm-review <id> {"run":true}` |
| `acm done` | `/acm-done` |

Direct CLI (`acm sync`, `acm health`, `acm history`, `acm status`) has no skill aliases — call those directly.
