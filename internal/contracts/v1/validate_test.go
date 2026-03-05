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
}
