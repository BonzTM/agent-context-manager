# acm-broker Templates

## Recommended Loop

1. Run `get_context`.
2. Follow the returned rules block (or rule pointers) as hard requirements.
3. Treat code pointers as advisory suggestions.
4. Run `fetch` for indexed artifacts by explicit keys or by `receipt_id` shorthand.
5. Execute the task.
6. Run `work` with `receipt_id` (no `plan_key` required) to publish updates. Use zero `items` for status-only retrieval; when posting updates, include `verify:tests` and `verify:diff-review`.
7. Run `report_completion`.
8. Run `propose_memory` when the result should persist.

## CLI `get_context` request

```json
{
  "version": "ctx.v1",
  "command": "get_context",
  "request_id": "req-get-context-001",
  "payload": {
    "project_id": "my-cool-app",
    "task_text": "fix preference save bug",
    "phase": "execute"
  }
}
```

Run:

```bash
go run ./cmd/acm validate --in assets/requests/get_context.execute.json
go run ./cmd/acm run --in assets/requests/get_context.execute.json
```

## MCP `get_context` input

```json
{
  "project_id": "my-cool-app",
  "task_text": "fix preference save bug",
  "phase": "execute"
}
```

Run:

```bash
go run ./cmd/acm-mcp invoke --tool get_context --in assets/requests/mcp_get_context.execute.json
```

## CLI `fetch` request

```json
{
  "version": "ctx.v1",
  "command": "fetch",
  "request_id": "req-fetch-001",
  "payload": {
    "project_id": "my-cool-app",
    "receipt_id": "replace-from-get-context-receipt"
  }
}
```

Run:

```bash
go run ./cmd/acm validate --in assets/requests/fetch.json
go run ./cmd/acm run --in assets/requests/fetch.json
```

## MCP `fetch` input

```json
{
  "project_id": "my-cool-app",
  "receipt_id": "replace-from-get-context-receipt"
}
```

Run:

```bash
go run ./cmd/acm-mcp invoke --tool fetch --in assets/requests/mcp_fetch.json
```

## CLI `work` request

For status-only retrieval, send zero `items`.

```json
{
  "version": "ctx.v1",
  "command": "work",
  "request_id": "req-work-001",
  "payload": {
    "project_id": "my-cool-app",
    "receipt_id": "replace-from-get-context-receipt",
    "items": []
  }
}
```

For update submissions, include `verify:tests` and `verify:diff-review` work items.

```json
{
  "version": "ctx.v1",
  "command": "work",
  "request_id": "req-work-002",
  "payload": {
    "project_id": "my-cool-app",
    "receipt_id": "replace-from-get-context-receipt",
    "items": [
      {
        "key": "verify:tests",
        "summary": "Run targeted tests for changed behavior",
        "status": "complete",
        "outcome": "PASS - targeted suite green"
      },
      {
        "key": "verify:diff-review",
        "summary": "Review diff for unintended file changes",
        "status": "complete",
        "outcome": "PASS - no unrelated edits found"
      }
    ]
  }
}
```

Run:

```bash
go run ./cmd/acm validate --in assets/requests/work.json
go run ./cmd/acm run --in assets/requests/work.json
```

## MCP `work` input

```json
{
  "project_id": "my-cool-app",
  "receipt_id": "replace-from-get-context-receipt",
  "items": []
}
```

For update submissions, use the same verification keys.

```json
{
  "project_id": "my-cool-app",
  "receipt_id": "replace-from-get-context-receipt",
  "items": [
    {
      "key": "verify:tests",
      "summary": "Run targeted tests for changed behavior",
      "status": "complete",
      "outcome": "PASS - targeted suite green"
    },
    {
      "key": "verify:diff-review",
      "summary": "Review diff for unintended file changes",
      "status": "complete",
      "outcome": "PASS - no unrelated edits found"
    }
  ]
}
```

Run:

```bash
go run ./cmd/acm-mcp invoke --tool work --in assets/requests/mcp_work.json
```

When work items are present, `report_completion.scope_mode` controls gate behavior: `strict` enforces verification checks, `warn` surfaces warnings.

## CLI `report_completion` request

```json
{
  "version": "ctx.v1",
  "command": "report_completion",
  "request_id": "req-report-001",
  "payload": {
    "project_id": "my-cool-app",
    "receipt_id": "replace-from-get-context-receipt",
    "files_changed": [
      "backend/src/services/preferences.ts"
    ],
    "outcome": "Fixed persistence write path and added regression coverage"
  }
}
```

## CLI `propose_memory` request

```json
{
  "version": "ctx.v1",
  "command": "propose_memory",
  "request_id": "req-memory-001",
  "payload": {
    "project_id": "my-cool-app",
    "receipt_id": "replace-from-get-context-receipt",
    "memory": {
      "category": "gotcha",
      "subject": "preference save requires cache invalidation",
      "content": "Preference persistence succeeds only when cache invalidation runs after DB commit.",
      "related_pointer_keys": [
        "my-cool-app:backend/src/services/preferences.ts"
      ],
      "tags": [
        "backend",
        "persistence",
        "cache"
      ],
      "confidence": 4,
      "evidence_pointer_keys": [
        "my-cool-app:backend/src/services/preferences.ts"
      ]
    },
    "auto_promote": true
  }
}
```
