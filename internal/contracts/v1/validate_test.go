package v1

import (
	"fmt"
	"strings"
	"testing"
)

func TestDecodeAndValidateCommand_GetContextSuccess(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"fix preference save bug",
			"phase":"execute"
		}
	}`
	env, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	if env.Command != CommandGetContext {
		t.Fatalf("expected command %q got %q", CommandGetContext, env.Command)
	}
	p, ok := payload.(GetContextPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.ProjectID != "my-cool-app" {
		t.Fatalf("unexpected project_id: %s", p.ProjectID)
	}
	if p.TagsFile != "" {
		t.Fatalf("expected empty tags_file by default, got %q", p.TagsFile)
	}
}

func TestDecodeAndValidateCommand_GetContextAcceptsTagsFile(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"fix preference save bug",
			"phase":"execute",
			"tags_file":".acm/acm-tags.yaml"
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(GetContextPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", p.TagsFile)
	}
}

func TestDecodeAndValidateCommand_InvalidVersion(t *testing.T) {
	json := `{
		"version":"ctx.v0",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"x",
			"phase":"execute"
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_VERSION" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_RejectsUnknownField(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"x",
			"phase":"execute",
			"oops":true
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_ReportCompletionRejectsInvalidScopeMode(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"report_completion",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"receipt-1234",
			"files_changed":["src/main.go"],
			"outcome":"done",
			"scope_mode":"invalid_mode"
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_CoveragePayloadValidation(t *testing.T) {
	validJSON := `{
		"version":"acm.v1",
		"command":"coverage",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"project_root":"."
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(validJSON))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(CoveragePayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.ProjectID != "my-cool-app" || p.ProjectRoot != "." {
		t.Fatalf("unexpected payload: %+v", p)
	}

	invalidJSON := `{
		"version":"acm.v1",
		"command":"coverage",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"project_root":" "
		}
	}`
	_, _, errp = DecodeAndValidateCommand([]byte(invalidJSON))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_FetchSuccess(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"keys":["docs/runtime.md","src/main.go"],
			"expected_versions":{
				"docs/runtime.md":"v2"
			}
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(FetchPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.ProjectID != "my-cool-app" || len(p.Keys) != 2 {
		t.Fatalf("unexpected payload: %+v", p)
	}
	if p.ExpectedVersions["docs/runtime.md"] != "v2" {
		t.Fatalf("unexpected expected_versions: %+v", p.ExpectedVersions)
	}
}

func TestDecodeAndValidateCommand_FetchRejectsUnknownField(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"keys":["docs/runtime.md"],
			"extra":true
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_FetchRejectsBlankKey(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"keys":["   "]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_FetchRejectsLongKey(t *testing.T) {
	longKey := strings.Repeat("k", 513)
	json := fmt.Sprintf(`{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"keys":[%q]
		}
	}`, longKey)
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkSuccess(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:receipt-1234",
			"plan_title":"Release readiness",
			"receipt_id":"receipt-1234",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"in_progress"},
				{"key":"docs/runtime.md","summary":"finalize docs","status":"complete","outcome":"done"}
			]
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(WorkPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.ProjectID != "my-cool-app" || p.PlanKey != "plan:receipt-1234" || len(p.Items) != 2 {
		t.Fatalf("unexpected payload: %+v", p)
	}
	if p.Items[1].Status != WorkItemStatusComplete {
		t.Fatalf("unexpected item status: %+v", p.Items[1])
	}
}

func TestDecodeAndValidateCommand_WorkAcceptsHierarchyAndExternalRefs(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:receipt-1234",
			"plan":{
				"title":"Checkout cleanup",
				"kind":"story",
				"parent_plan_key":"plan:receipt-9999",
				"external_refs":["jira:WEB-123"]
			},
			"tasks":[
				{
					"key":"task.checkout.1",
					"summary":"Split cart service",
					"status":"in_progress",
					"parent_task_key":"task.checkout.epic",
					"external_refs":["linear:ENG-77"]
				}
			]
		}
	}`

	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}

	p, ok := payload.(WorkPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.Plan == nil || p.Plan.Kind != "story" || p.Plan.ParentPlanKey != "plan:receipt-9999" {
		t.Fatalf("unexpected plan payload: %+v", p.Plan)
	}
	if len(p.Tasks) != 1 || p.Tasks[0].ParentTaskKey != "task.checkout.epic" {
		t.Fatalf("unexpected tasks payload: %+v", p.Tasks)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsInvalidPlanKind(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:receipt-1234",
			"plan":{
				"kind":"Story"
			}
		}
	}`

	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsWhitespaceWrappedParentPlanKey(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:receipt-1234",
			"plan":{
				"parent_plan_key":" plan:receipt-9999 "
			}
		}
	}`

	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsInvalidStatus(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:receipt-1234",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"completed"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsEmptyPlanKey(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"   ",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsEmptyItemKey(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:receipt-1234",
			"items":[
				{"key":"   ","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsInvalidPlanKeyFormat(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan.release.v1",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsMixedCasePlanKeyPrefix(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"PLAN:receipt-1234",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsWhitespacePaddedPlanKey(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":" plan:receipt-1234 ",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsShortPlanReceiptID(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:short",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsPlanKeyReceiptMismatch(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"plan_key":"plan:receipt-1234",
			"receipt_id":"receipt-9999",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_FetchReceiptIDOnlySuccess(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"receipt-1234"
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(FetchPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.ReceiptID != "receipt-1234" || len(p.Keys) != 0 {
		t.Fatalf("unexpected payload: %+v", p)
	}
}

func TestDecodeAndValidateCommand_FetchRejectsMissingKeysAndReceiptID(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app"
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_FetchRejectsExpectedVersionsWithoutKeys(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"receipt-1234",
			"expected_versions":{
				"docs/runtime.md":"v2"
			}
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_FetchRejectsInvalidReceiptID(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"bad"
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_WorkReceiptOnlySuccess(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"receipt-1234"
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(WorkPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.PlanKey != "" || p.ReceiptID != "receipt-1234" || len(p.Items) != 0 {
		t.Fatalf("unexpected payload: %+v", p)
	}
}

func TestDecodeAndValidateCommand_WorkReceiptOnlyAllowsEmptyItems(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"receipt-1234",
			"items":[]
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(WorkPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if len(p.Items) != 0 {
		t.Fatalf("unexpected payload: %+v", p)
	}
}

func TestDecodeAndValidateCommand_WorkRejectsMissingPlanKeyAndReceiptID(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"work",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"items":[
				{"key":"src/main.go","summary":"wire dispatch","status":"pending"}
			]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_SyncPayloadValidation(t *testing.T) {
	validJSON := `{
		"version":"acm.v1",
		"command":"sync",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"mode":"working_tree",
			"rules_file":"custom-rules.yaml",
			"tags_file":"custom-tags.json"
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(validJSON))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(SyncPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.RulesFile != "custom-rules.yaml" {
		t.Fatalf("unexpected rules_file: %q", p.RulesFile)
	}
	if p.TagsFile != "custom-tags.json" {
		t.Fatalf("unexpected tags_file: %q", p.TagsFile)
	}
}

func TestDecodeAndValidateCommand_HealthFixPayloadValidation(t *testing.T) {
	validJSON := `{
		"version":"acm.v1",
		"command":"health_fix",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"apply":true,
			"project_root":".",
			"rules_file":"custom-rules.yaml",
			"tags_file":"custom-tags.json",
			"fixers":["sync_working_tree","sync_ruleset"]
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(validJSON))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(HealthFixPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.ProjectID != "my-cool-app" {
		t.Fatalf("unexpected project_id: %q", p.ProjectID)
	}
	if p.Apply == nil || !*p.Apply {
		t.Fatalf("expected apply=true, got %+v", p.Apply)
	}
	if p.RulesFile != "custom-rules.yaml" {
		t.Fatalf("unexpected rules_file: %q", p.RulesFile)
	}
	if p.TagsFile != "custom-tags.json" {
		t.Fatalf("unexpected tags_file: %q", p.TagsFile)
	}
	if len(p.Fixers) != 2 {
		t.Fatalf("unexpected fixer count: %d", len(p.Fixers))
	}

	invalidJSON := `{
		"version":"acm.v1",
		"command":"health_fix",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"fixers":["bad_fixer"]
		}
	}`
	_, _, errp = DecodeAndValidateCommand([]byte(invalidJSON))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_BootstrapPayloadPersistCandidates(t *testing.T) {
	validJSON := `{
		"version":"acm.v1",
		"command":"bootstrap",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"project_root":".",
			"tags_file":"custom-tags.json",
			"persist_candidates":true
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(validJSON))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(BootstrapPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.PersistCandidates == nil || !*p.PersistCandidates {
		t.Fatalf("expected persist_candidates=true, got %+v", p.PersistCandidates)
	}
	if p.TagsFile != "custom-tags.json" {
		t.Fatalf("unexpected tags_file: %q", p.TagsFile)
	}
}

func TestDecodeAndValidateCommand_ProposeMemoryAcceptsTagsFile(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"propose_memory",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"req-87654321",
			"tags_file":".acm/acm-tags.yaml",
			"memory":{
				"category":"decision",
				"subject":"Use shared logger",
				"content":"Prefer one wrapper",
				"related_pointer_keys":["rule:my-cool-app/rule-1"],
				"tags":["logging"],
				"confidence":4,
				"evidence_pointer_keys":["rule:my-cool-app/rule-1"]
			}
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(ProposeMemoryPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", p.TagsFile)
	}
}

func TestDecodeAndValidateCommand_ReportCompletionAcceptsTagsFile(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"report_completion",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"receipt-1234",
			"tags_file":".acm/acm-tags.yaml",
			"files_changed":["src/main.go"],
			"outcome":"done"
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(ReportCompletionPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", p.TagsFile)
	}
}

func TestDecodeAndValidateCommand_EvalAcceptsTagsFile(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"eval",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"tags_file":".acm/acm-tags.yaml",
			"eval_suite_inline":[
				{"task_text":"Check sync","phase":"execute"}
			]
		}
	}`
	_, payload, errp := DecodeAndValidateCommand([]byte(json))
	if errp != nil {
		t.Fatalf("unexpected error: %+v", errp)
	}
	p, ok := payload.(EvalPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", payload)
	}
	if p.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", p.TagsFile)
	}
}

func TestDecodeAndValidateCommand_GetContextRejectsOutOfRangeCaps(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"x",
			"phase":"execute",
			"caps":{
				"word_budget_limit":50
			}
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_FetchRejectsDuplicateKeys(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"fetch",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"keys":["plan:req-12345678","plan:req-12345678"]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_ProposeMemoryRejectsDuplicateTags(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"propose_memory",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"req-87654321",
			"memory":{
				"category":"decision",
				"subject":"Use shared logger",
				"content":"Prefer one wrapper",
				"related_pointer_keys":["rule:my-cool-app/rule-1"],
				"tags":["logging","logging"],
				"confidence":4,
				"evidence_pointer_keys":["rule:my-cool-app/rule-1"]
			}
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_ReportCompletionRejectsMissingFilesChanged(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"report_completion",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"receipt_id":"receipt-1234",
			"outcome":"done"
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_HealthFixRejectsDuplicateFixers(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"health_fix",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"fixers":["sync_ruleset","sync_ruleset"]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}

func TestDecodeAndValidateCommand_VerifyRejectsEmptyFilesChangedWhenProvided(t *testing.T) {
	json := `{
		"version":"acm.v1",
		"command":"verify",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"phase":"execute",
			"files_changed":[]
		}
	}`
	_, _, errp := DecodeAndValidateCommand([]byte(json))
	if errp == nil {
		t.Fatal("expected validation error")
	}
	if errp.Code != "INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", errp.Code)
	}
}
