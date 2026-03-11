# acm-broker Templates

## Recommended Loop

These examples assume installed `acm` and `acm-mcp` binaries are available on `PATH`.
Example payloads show explicit `project_id` values for clarity. In live usage you may omit `project_id` when `ACM_PROJECT_ID` is set or acm can infer the project from the effective repo root.
For Codex-first repo setup, the installed skill also ships `codex/README.md` and `codex/AGENTS.example.md`, and `acm init --apply-template codex-pack` seeds repo-local copies under `.codex/acm-broker/`.

1. Run `acm context`.
2. Follow the returned rules block (or rule pointers) as hard requirements.
3. Note any `initial_scope_paths` already known at task start and treat them as the initial governed scope, not as a substitute for native repo search.
4. Run `fetch` only for indexed artifacts you actually need to hydrate, either by explicit keys or by `receipt_id` shorthand, which derives the plan fetch key.
5. Execute the task.
6. Run `work` with `receipt_id` (no `plan_key` required) to publish updates. Use `tasks` and include `verify:tests` for executable verification tracking; add other task keys when `.acm/acm-workflows.yaml` requires them. When governed file scope expands after `context`, append those repo-relative files to `plan.discovered_paths`.
7. Run `review` when you only need to record a single review-gate outcome instead of assembling a broader `work` payload.
8. Run `verify` before `done` when code changes.
9. Run `done` with changed files for file-backed work when you know them, or omit / leave `files_changed` empty to let ACM derive the real task delta from the receipt baseline.
10. Run `memory` when the result should persist.
11. When resuming or auditing prior work, use direct CLI `acm history`, setting `--entity work` when you need work-specific `--scope` or `--kind` filters, then `acm fetch` the returned `fetch_keys`.

Maintenance note:
- When rules, tags, tests, workflows, onboarding, or tool-surface behavior change, run `acm sync --mode working_tree --insert-new-candidates` and then `acm health --include-details` before `done`.
- `acm health --fix <name>` applies the selected fixer by default.
- Add `--dry-run` to preview health fixes without changing state.
- Use `acm health --fix all` to run the default fixer set explicitly.

## CLI `context` request

```json
{
  "version": "acm.v1",
  "command": "context",
  "request_id": "req-context-001",
  "payload": {
    "project_id": "customer-portal",
    "task_text": "fix the profile settings save race so stale responses cannot overwrite newer edits",
    "phase": "execute"
  }
}
```

Run directly:

```bash
acm context --project customer-portal --task-text "fix the profile settings save race so stale responses cannot overwrite newer edits" --phase execute
```

Optional structured JSON automation:

```bash
acm validate --in assets/requests/context.execute.json
acm run --in assets/requests/context.execute.json
```

## MCP `context` input

```json
{
  "project_id": "customer-portal",
  "task_text": "fix the profile settings save race so stale responses cannot overwrite newer edits",
  "phase": "execute"
}
```

Run:

```bash
acm-mcp invoke --tool context --in assets/requests/mcp_context.execute.json
```

## CLI `fetch` request

```json
{
  "version": "acm.v1",
  "command": "fetch",
  "request_id": "req-fetch-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "keys": [
      "customer-portal:web/src/features/profile/ProfileForm.tsx",
      "customer-portal:web/src/features/profile/useSaveProfile.ts",
      "customer-portal:web/src/features/profile/useSaveProfile.test.tsx"
    ]
  }
}
```

Run directly:

```bash
acm fetch --project customer-portal --receipt-id replace-from-context-receipt --key customer-portal:web/src/features/profile/ProfileForm.tsx --key customer-portal:web/src/features/profile/useSaveProfile.ts --key customer-portal:web/src/features/profile/useSaveProfile.test.tsx
```

Optional structured JSON automation:

```bash
acm validate --in assets/requests/fetch.json
acm run --in assets/requests/fetch.json
```

## MCP `fetch` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-context-receipt",
  "keys": [
    "customer-portal:web/src/features/profile/ProfileForm.tsx",
    "customer-portal:web/src/features/profile/useSaveProfile.ts",
    "customer-portal:web/src/features/profile/useSaveProfile.test.tsx"
  ]
}
```

Run:

```bash
acm-mcp invoke --tool fetch --in assets/requests/mcp_fetch.json
```

## CLI `work` request

```json
{
  "version": "acm.v1",
  "command": "work",
  "request_id": "req-work-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "mode": "merge",
    "plan": {
      "title": "Profile settings save race",
      "objective": "Prevent stale save responses from overwriting newer settings and verify the fix",
      "kind": "bugfix",
      "status": "in_progress",
      "discovered_paths": [
        "web/src/features/profile/useSaveProfile.test.tsx"
      ],
      "constraints": [
        "Preserve the existing save API contract",
        "Keep optimistic success feedback for the newest request"
      ],
      "references": [
        "customer-portal:web/src/features/profile/ProfileForm.tsx",
        "customer-portal:web/src/features/profile/useSaveProfile.ts"
      ]
    },
    "tasks": [
      {
        "key": "confirm-race-window",
        "summary": "Confirm how concurrent save requests can resolve out of order",
        "status": "complete",
        "references": [
          "customer-portal:web/src/features/profile/useSaveProfile.ts"
        ],
        "outcome": "Older requests can still commit state after a newer submission resolves."
      },
      {
        "key": "guard-stale-saves",
        "summary": "Ignore stale save responses and keep the newest request authoritative",
        "status": "in_progress",
        "depends_on": ["confirm-race-window"],
        "acceptance_criteria": [
          "Submitting profile changes twice quickly leaves the latest values in state",
          "Loading and success UI still resolve correctly for the newest request"
        ],
        "references": [
          "customer-portal:web/src/features/profile/ProfileForm.tsx",
          "customer-portal:web/src/features/profile/useSaveProfile.ts"
        ]
      },
      {
        "key": "verify:tests",
        "summary": "Run targeted frontend verification for profile save behavior",
        "status": "pending",
        "acceptance_criteria": [
          "Profile save regression tests pass",
          "Relevant smoke or build checks pass"
        ]
      }
    ]
  }
}
```

Run directly:

```bash
acm work --project customer-portal --receipt-id replace-from-context-receipt --mode merge --plan-json '{"title":"Profile settings save race","objective":"Prevent stale save responses from overwriting newer settings and verify the fix","kind":"bugfix","status":"in_progress","discovered_paths":["web/src/features/profile/useSaveProfile.test.tsx"]}' --tasks-json '[{"key":"guard-stale-saves","summary":"Ignore stale save responses and keep the newest request authoritative","status":"in_progress"},{"key":"verify:tests","summary":"Run targeted frontend verification for profile save behavior","status":"pending"}]'
```

The example above treats the regression test file as later-discovered governed scope through `plan.discovered_paths`, which is what `review` and `done` validate when work expands beyond the initial receipt.

Optional structured JSON automation:

```bash
acm validate --in assets/requests/work.json
acm run --in assets/requests/work.json
```

If the repo defines a richer feature-plan contract, use the same `work` surface with explicit stages and grouping tasks instead of inventing a separate planning system:

```json
{
  "version": "acm.v1",
  "command": "work",
  "request_id": "req-work-feature-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "mode": "merge",
    "plan": {
      "title": "Offline downloads",
      "kind": "feature",
      "objective": "Ship bounded offline downloads across backend and frontend surfaces.",
      "status": "in_progress",
      "stages": {
        "spec_outline": "complete",
        "refined_spec": "complete",
        "implementation_plan": "in_progress"
      }
    },
    "tasks": [
      {
        "key": "stage:spec-outline",
        "summary": "Spec outline",
        "status": "complete"
      },
      {
        "key": "spec:capabilities",
        "summary": "Define required capabilities",
        "status": "complete",
        "parent_task_key": "stage:spec-outline",
        "acceptance_criteria": [
          "Capabilities cover UX, backend, and operator impact"
        ]
      },
      {
        "key": "stage:implementation-plan",
        "summary": "Implementation plan",
        "status": "in_progress"
      },
      {
        "key": "impl:backend-contract",
        "summary": "Implement the backend contract",
        "status": "in_progress",
        "parent_task_key": "stage:implementation-plan",
        "acceptance_criteria": [
          "Backend behavior and regression coverage are explicit"
        ]
      },
      {
        "key": "verify:tests",
        "summary": "Run verification for feature work",
        "status": "pending"
      }
    ]
  }
}
```

Repos can enforce that schema through a repo-local `verify` script that inspects the active plan via `ACM_PLAN_KEY` / `ACM_RECEIPT_ID` and verify selection metadata via `ACM_VERIFY_PHASE`, `ACM_VERIFY_TAGS_JSON`, and `ACM_VERIFY_FILES_CHANGED_JSON`.

## MCP `work` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-context-receipt",
  "mode": "merge",
  "plan": {
    "title": "Profile settings save race",
    "objective": "Prevent stale save responses from overwriting newer settings and verify the fix",
    "kind": "bugfix",
    "status": "in_progress",
    "discovered_paths": [
      "web/src/features/profile/useSaveProfile.test.tsx"
    ]
  },
  "tasks": [
    {
      "key": "guard-stale-saves",
      "summary": "Ignore stale save responses and keep the newest request authoritative",
      "status": "in_progress"
    },
    {
      "key": "verify:tests",
      "summary": "Run targeted frontend verification for profile save behavior",
      "status": "pending"
    }
  ]
}
```

Run:

```bash
acm-mcp invoke --tool work --in assets/requests/mcp_work.json
```

## CLI `review` request

```json
{
  "version": "acm.v1",
  "command": "review",
  "request_id": "req-review-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "run": true
  }
}
```

Run directly:

```bash
acm review --project customer-portal --receipt-id replace-from-context-receipt --run
```

Optional structured JSON automation:

```bash
acm validate --in assets/requests/review.json
acm run --in assets/requests/review.json
```

## MCP `review` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-context-receipt",
  "run": true
}
```

Run:

```bash
acm-mcp invoke --tool review --in assets/requests/mcp_review.json
```

`review` is intentionally thin. It lowers to one `work.tasks[]` merge update. Omitted `key`, `summary`, and `status` default to `review:cross-llm`, `Cross-LLM review`, and `complete`. Prefer `run=true` when the repo workflow defines a runnable review gate because manual complete notes do not satisfy runnable gates. Use `status=blocked` plus `blocked_reason` when the review gate is waiting or failed, and reserve manual `status`, `outcome`, `blocked_reason`, and `evidence` fields for non-run mode. Put repo-local reviewer choices such as script arguments, model ids, or reasoning levels in the workflow `run.argv` block.

When work tasks are present, `done.scope_mode` controls gate behavior: `strict` enforces configured completion tasks from `.acm/acm-workflows.yaml`, and `warn` surfaces warnings. ACM uses the receipt baseline delta as the authoritative file set when it is available. If you also supply `files_changed`, ACM cross-checks that list and surfaces mismatches as violations. When changed files are present and no workflow gates are configured, ACM falls back to `verify:tests`; a detected empty delta behaves like a no-file closure but still honors explicit workflow gates.

## CLI `verify` request

```json
{
  "version": "acm.v1",
  "command": "verify",
  "request_id": "req-verify-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "phase": "review",
    "files_changed": [
      "web/src/features/profile/ProfileForm.tsx",
      "web/src/features/profile/useSaveProfile.ts",
      "web/src/features/profile/useSaveProfile.test.tsx"
    ]
  }
}
```

Run directly:

```bash
acm verify --project customer-portal --receipt-id replace-from-context-receipt --phase review --file-changed web/src/features/profile/ProfileForm.tsx --file-changed web/src/features/profile/useSaveProfile.ts --file-changed web/src/features/profile/useSaveProfile.test.tsx
```

Optional structured JSON automation:

```bash
acm validate --in assets/requests/verify.json
acm run --in assets/requests/verify.json
```

## MCP `verify` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-context-receipt",
  "phase": "review",
  "files_changed": [
    "web/src/features/profile/ProfileForm.tsx",
    "web/src/features/profile/useSaveProfile.ts",
    "web/src/features/profile/useSaveProfile.test.tsx"
  ]
}
```

Run:

```bash
acm-mcp invoke --tool verify --in assets/requests/mcp_verify.json
```

## CLI `done` request

```json
{
  "version": "acm.v1",
  "command": "done",
  "request_id": "req-done-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "files_changed": [
      "web/src/features/profile/ProfileForm.tsx",
      "web/src/features/profile/useSaveProfile.ts",
      "web/src/features/profile/useSaveProfile.test.tsx"
    ],
    "outcome": "Ignored stale profile save responses, preserved optimistic UI feedback, and added regression coverage."
  }
}
```

Run directly:

```bash
acm done --project customer-portal --receipt-id replace-from-context-receipt --file-changed web/src/features/profile/ProfileForm.tsx --file-changed web/src/features/profile/useSaveProfile.ts --file-changed web/src/features/profile/useSaveProfile.test.tsx --outcome "Ignored stale profile save responses, preserved optimistic UI feedback, and added regression coverage."
```

Optional structured JSON automation:

```bash
acm validate --in assets/requests/done.json
acm run --in assets/requests/done.json
```

## MCP `done` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-context-receipt",
  "files_changed": [
    "web/src/features/profile/ProfileForm.tsx",
    "web/src/features/profile/useSaveProfile.ts",
    "web/src/features/profile/useSaveProfile.test.tsx"
  ],
  "outcome": "Ignored stale profile save responses, preserved optimistic UI feedback, and added regression coverage."
}
```

Run:

```bash
acm-mcp invoke --tool done --in assets/requests/mcp_done.json
```

When the detected task delta is empty, the closeout is effectively no-file:

```json
{
  "version": "acm.v1",
  "command": "done",
  "request_id": "req-done-no-files-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "outcome": "Drafted the rollout plan, recorded follow-up review work, and closed the no-file planning task."
  }
}
```

## CLI `memory` request

```json
{
  "version": "acm.v1",
  "command": "memory",
  "request_id": "req-memory-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-context-receipt",
    "memory": {
      "category": "gotcha",
      "subject": "profile save hook must ignore stale request completions",
      "content": "Concurrent profile saves can resolve out of order. The save hook must treat the newest request as authoritative or stale responses will overwrite fresh form state.",
      "related_pointer_keys": [
        "customer-portal:web/src/features/profile/useSaveProfile.ts",
        "customer-portal:web/src/features/profile/ProfileForm.tsx"
      ],
      "tags": [
        "frontend",
        "api",
        "test"
      ],
      "confidence": 4,
      "evidence_pointer_keys": [
        "customer-portal:web/src/features/profile/useSaveProfile.ts",
        "customer-portal:web/src/features/profile/useSaveProfile.test.tsx"
      ]
    },
    "auto_promote": true
  }
}
```

Run directly:

```bash
acm memory --project customer-portal --receipt-id replace-from-context-receipt --category gotcha --subject "profile save hook must ignore stale request completions" --content "Concurrent profile saves can resolve out of order. The save hook must treat the newest request as authoritative or stale responses will overwrite fresh form state." --related-path web/src/features/profile/useSaveProfile.ts --related-path web/src/features/profile/ProfileForm.tsx --memory-tag frontend --memory-tag api --memory-tag test --confidence 4 --evidence-path web/src/features/profile/useSaveProfile.ts --evidence-path web/src/features/profile/useSaveProfile.test.tsx --auto-promote
```

For CLI calls, prefer `--evidence-path` / `--related-path` when you only know governed repo-relative files. JSON/MCP payloads still carry the explicit pointer-key arrays because the wire contract uses exact keys rather than path shorthands.

Optional structured JSON automation:

```bash
acm validate --in assets/requests/memory.json
acm run --in assets/requests/memory.json
```

## MCP `memory` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-context-receipt",
  "memory": {
    "category": "gotcha",
    "subject": "profile save hook must ignore stale request completions",
    "content": "Concurrent profile saves can resolve out of order. The save hook must treat the newest request as authoritative or stale responses will overwrite fresh form state.",
    "related_pointer_keys": [
      "customer-portal:web/src/features/profile/useSaveProfile.ts",
      "customer-portal:web/src/features/profile/ProfileForm.tsx"
    ],
    "tags": [
      "frontend",
      "api",
      "test"
    ],
    "confidence": 4,
    "evidence_pointer_keys": [
      "customer-portal:web/src/features/profile/useSaveProfile.ts",
      "customer-portal:web/src/features/profile/useSaveProfile.test.tsx"
    ]
  },
  "auto_promote": true
}
```

Run:

```bash
acm-mcp invoke --tool memory --in assets/requests/mcp_memory.json
```

## CLI `acm history` request

```json
{
  "version": "acm.v1",
  "command": "history",
  "request_id": "req-history-001",
  "payload": {
    "project_id": "customer-portal",
    "entity": "all",
    "query": "profile save",
    "limit": 20
  }
}
```

Run directly:

```bash
acm history --project customer-portal --entity all --query "profile save" --limit 20
```

Optional structured JSON automation:

```bash
acm validate --in assets/requests/history.json
acm run --in assets/requests/history.json
```

Convenience CLI equivalents:

```bash
acm history --project customer-portal --entity work --scope current
acm history --project customer-portal --entity work --query "profile save"
acm history --project customer-portal --entity work --query "profile save" --scope completed --kind bugfix
acm history --project customer-portal --entity all --query "profile save" --limit 20
acm history --project customer-portal --entity memory --query "profile save"
```

## MCP `history` input

```json
{
  "project_id": "customer-portal",
  "entity": "memory",
  "query": "profile save",
  "limit": 10
}
```

Run:

```bash
acm-mcp invoke --tool history --in assets/requests/mcp_history.json
```
