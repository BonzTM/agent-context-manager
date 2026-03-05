Report completion through the context broker.

Arguments format:
`<receipt_id> <comma-separated-files> <outcome summary>`

Input: $ARGUMENTS

Steps:
1. Parse arguments into:
   - `receipt_id`
   - `files_changed[]`
   - `outcome`
2. Build valid `acm.v1` `report_completion` JSON.
3. Validate:
   - `go run ./cmd/acm validate --in <request.json>`
4. Execute:
   - `go run ./cmd/acm run --in <request.json>`
5. If plan tracking context is available (for example from prior `fetch` results), build a `work` request with:
   - the active `project_id`
   - `receipt_id`
   - optional `plan_key` (only if you need to override inference)
   - zero or more updated work tasks (`status` + `outcome` when sending updates)
   - when sending updates, include verification tasks keyed `verify:tests` and `verify:diff-review`
6. Validate and execute the `work` request:
   - `go run ./cmd/acm validate --in <work-request.json>`
   - `go run ./cmd/acm run --in <work-request.json>`
7. Return broker response(s) exactly.

Constraints:
- Never omit any changed file.
- `scope_mode` defaults to advisory `warn`; set `strict` or `auto_index` only when explicitly required.
- When work tasks are present, treat `verify:tests` and `verify:diff-review` as required quality gates: `strict` is enforced, `warn` is surfaced as warnings.
