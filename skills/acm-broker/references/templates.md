# acm-broker Templates

## Recommended Loop

These examples assume installed `acm` and `acm-mcp` binaries are available on `PATH`.

1. Run `get_context`.
2. Follow the returned rules block (or rule pointers) as hard requirements.
3. Treat code pointers as advisory suggestions.
4. Run `fetch` for indexed artifacts by explicit keys or by `receipt_id` shorthand, which derives the plan fetch key.
5. Execute the task.
6. Run `work` with `receipt_id` (no `plan_key` required) to publish updates. Use `tasks` and include `verify:tests` for executable verification tracking.
7. Run `verify` before `report_completion` when code changes.
8. Run `report_completion`.
9. Run `propose_memory` when the result should persist.
10. When resuming or auditing prior work, use `history_search` or the CLI `work list`, `work search`, `history search --entity all`, or `history search --entity memory`, then `fetch` the returned `fetch_keys`.

## CLI `get_context` request

```json
{
  "version": "acm.v1",
  "command": "get_context",
  "request_id": "req-get-context-001",
  "payload": {
    "project_id": "customer-portal",
    "task_text": "fix the profile settings save race so stale responses cannot overwrite newer edits",
    "phase": "execute"
  }
}
```

Run:

```bash
acm validate --in assets/requests/get_context.execute.json
acm run --in assets/requests/get_context.execute.json
```

## MCP `get_context` input

```json
{
  "project_id": "customer-portal",
  "task_text": "fix the profile settings save race so stale responses cannot overwrite newer edits",
  "phase": "execute"
}
```

Run:

```bash
acm-mcp invoke --tool get_context --in assets/requests/mcp_get_context.execute.json
```

## CLI `fetch` request

```json
{
  "version": "acm.v1",
  "command": "fetch",
  "request_id": "req-fetch-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-get-context-receipt",
    "keys": [
      "customer-portal:web/src/features/profile/ProfileForm.tsx",
      "customer-portal:web/src/features/profile/useSaveProfile.ts",
      "customer-portal:web/src/features/profile/useSaveProfile.test.tsx"
    ]
  }
}
```

Run:

```bash
acm validate --in assets/requests/fetch.json
acm run --in assets/requests/fetch.json
```

## MCP `fetch` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-get-context-receipt",
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
    "receipt_id": "replace-from-get-context-receipt",
    "mode": "merge",
    "plan": {
      "title": "Profile settings save race",
      "objective": "Prevent stale save responses from overwriting newer settings and verify the fix",
      "kind": "bugfix",
      "status": "in_progress",
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

Run:

```bash
acm validate --in assets/requests/work.json
acm run --in assets/requests/work.json
```

## MCP `work` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-get-context-receipt",
  "mode": "merge",
  "plan": {
    "title": "Profile settings save race",
    "objective": "Prevent stale save responses from overwriting newer settings and verify the fix",
    "kind": "bugfix",
    "status": "in_progress"
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

When work tasks are present, `report_completion.scope_mode` controls gate behavior: `strict` enforces `verify:tests`, `warn` surfaces warnings.

## CLI `verify` request

```json
{
  "version": "acm.v1",
  "command": "verify",
  "request_id": "req-verify-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-get-context-receipt",
    "phase": "review",
    "files_changed": [
      "web/src/features/profile/ProfileForm.tsx",
      "web/src/features/profile/useSaveProfile.ts",
      "web/src/features/profile/useSaveProfile.test.tsx"
    ]
  }
}
```

Run:

```bash
acm validate --in assets/requests/verify.json
acm run --in assets/requests/verify.json
```

## MCP `verify` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-get-context-receipt",
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

## CLI `report_completion` request

```json
{
  "version": "acm.v1",
  "command": "report_completion",
  "request_id": "req-report-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-get-context-receipt",
    "files_changed": [
      "web/src/features/profile/ProfileForm.tsx",
      "web/src/features/profile/useSaveProfile.ts",
      "web/src/features/profile/useSaveProfile.test.tsx"
    ],
    "outcome": "Ignored stale profile save responses, preserved optimistic UI feedback, and added regression coverage."
  }
}
```

Run:

```bash
acm validate --in assets/requests/report_completion.json
acm run --in assets/requests/report_completion.json
```

## MCP `report_completion` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-get-context-receipt",
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
acm-mcp invoke --tool report_completion --in assets/requests/mcp_report_completion.json
```

## CLI `propose_memory` request

```json
{
  "version": "acm.v1",
  "command": "propose_memory",
  "request_id": "req-memory-001",
  "payload": {
    "project_id": "customer-portal",
    "receipt_id": "replace-from-get-context-receipt",
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

Run:

```bash
acm validate --in assets/requests/propose_memory.json
acm run --in assets/requests/propose_memory.json
```

## MCP `propose_memory` input

```json
{
  "project_id": "customer-portal",
  "receipt_id": "replace-from-get-context-receipt",
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
acm-mcp invoke --tool propose_memory --in assets/requests/mcp_propose_memory.json
```

## CLI `history search` request

```json
{
  "version": "acm.v1",
  "command": "history_search",
  "request_id": "req-history-001",
  "payload": {
    "project_id": "customer-portal",
    "entity": "all",
    "query": "profile save",
    "limit": 20
  }
}
```

Run:

```bash
acm validate --in assets/requests/history_search.json
acm run --in assets/requests/history_search.json
```

Convenience CLI equivalents:

```bash
acm work list --project customer-portal --scope current
acm work search --project customer-portal --query "profile save"
acm work search --project customer-portal --query "profile save" --scope completed --kind bugfix
acm history search --project customer-portal --entity all --query "profile save" --limit 20
acm history search --project customer-portal --entity memory --query "profile save"
```

## MCP `history_search` input

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
acm-mcp invoke --tool history_search --in assets/requests/mcp_history_search.json
```
