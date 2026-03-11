package v1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSharedSchemaCommandEnumMatchesCommandCatalog(t *testing.T) {
	raw := readSchemaFixture(t, "shared.schema.json")

	var doc struct {
		Defs struct {
			CommandName struct {
				Enum []string `json:"enum"`
			} `json:"commandName"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal shared schema: %v", err)
	}

	if got, want := doc.Defs.CommandName.Enum, CommandNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("command enum drift: got %v want %v", got, want)
	}
}

func TestCommandAndResultSchemasParseAndRejectRemovedLegacyNames(t *testing.T) {
	for _, filename := range []string{"cli.command.schema.json", "cli.result.schema.json"} {
		t.Run(filename, func(t *testing.T) {
			raw := readSchemaFixture(t, filename)

			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("unmarshal %s: %v", filename, err)
			}

			for _, removed := range []string{`"get_context"`, `"propose_memory"`, `"report_completion"`, `"history_search"`, `"bootstrapTemplateResult"`, `"bootstrapTemplateConflict"`} {
				if strings.Contains(string(raw), removed) {
					t.Fatalf("%s still contains removed legacy command %s", filename, removed)
				}
			}
		})
	}
}

func TestCommandSchema_MemoryAndDoneRequireReceiptOrPlanSelection(t *testing.T) {
	raw := readSchemaFixture(t, "cli.command.schema.json")

	var doc struct {
		Defs map[string]map[string]any `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal cli command schema: %v", err)
	}

	assertReceiptOrPlanSelection := func(defName string) {
		t.Helper()

		def, ok := doc.Defs[defName]
		if !ok {
			t.Fatalf("missing schema def %q", defName)
		}
		properties, ok := def["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s.properties missing or invalid", defName)
		}
		if _, ok := properties["receipt_id"]; !ok {
			t.Fatalf("%s missing receipt_id property", defName)
		}
		if _, ok := properties["plan_key"]; !ok {
			t.Fatalf("%s missing plan_key property", defName)
		}

		required, _ := def["required"].([]any)
		for _, value := range required {
			if value == "receipt_id" {
				t.Fatalf("%s should not require receipt_id directly once plan_key is supported", defName)
			}
		}

		allOf, ok := def["allOf"].([]any)
		if !ok {
			t.Fatalf("%s.allOf missing or invalid", defName)
		}
		foundSelection := false
		for _, rawClause := range allOf {
			clause, ok := rawClause.(map[string]any)
			if !ok {
				continue
			}
			anyOf, ok := clause["anyOf"].([]any)
			if !ok {
				continue
			}
			requiresReceipt := false
			requiresPlan := false
			for _, rawOption := range anyOf {
				option, ok := rawOption.(map[string]any)
				if !ok {
					continue
				}
				requiredList, ok := option["required"].([]any)
				if !ok || len(requiredList) != 1 {
					continue
				}
				switch requiredList[0] {
				case "receipt_id":
					requiresReceipt = true
				case "plan_key":
					requiresPlan = true
				}
			}
			if requiresReceipt && requiresPlan {
				foundSelection = true
				break
			}
		}
		if !foundSelection {
			t.Fatalf("%s missing receipt_id|plan_key selection guard", defName)
		}
	}

	assertReceiptOrPlanSelection("memoryPayload")
	assertReceiptOrPlanSelection("donePayload")
}

func TestSharedAndResultSchemasExposeSupersededStatusesAndStatusWarnings(t *testing.T) {
	sharedRaw := readSchemaFixture(t, "shared.schema.json")
	var sharedDoc struct {
		Defs struct {
			WorkItemStatus struct {
				Enum []string `json:"enum"`
			} `json:"workItemStatus"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(sharedRaw, &sharedDoc); err != nil {
		t.Fatalf("unmarshal shared schema: %v", err)
	}
	if !reflect.DeepEqual(sharedDoc.Defs.WorkItemStatus.Enum, []string{"pending", "in_progress", "complete", "blocked", "superseded"}) {
		t.Fatalf("unexpected work item status enum: %v", sharedDoc.Defs.WorkItemStatus.Enum)
	}

	resultRaw := readSchemaFixture(t, "cli.result.schema.json")
	var resultDoc struct {
		Defs map[string]map[string]any `json:"$defs"`
	}
	if err := json.Unmarshal(resultRaw, &resultDoc); err != nil {
		t.Fatalf("unmarshal cli result schema: %v", err)
	}

	statusSummary, ok := resultDoc.Defs["statusSummary"]
	if !ok {
		t.Fatal("missing statusSummary schema")
	}
	properties, ok := statusSummary["properties"].(map[string]any)
	if !ok {
		t.Fatal("statusSummary.properties missing or invalid")
	}
	if _, ok := properties["warning_count"]; !ok {
		t.Fatal("statusSummary missing warning_count property")
	}

	statusResult, ok := resultDoc.Defs["statusResult"]
	if !ok {
		t.Fatal("missing statusResult schema")
	}
	resultProperties, ok := statusResult["properties"].(map[string]any)
	if !ok {
		t.Fatal("statusResult.properties missing or invalid")
	}
	if _, ok := resultProperties["warnings"]; !ok {
		t.Fatal("statusResult missing warnings property")
	}

	for _, defName := range []string{"reviewResult", "workResult"} {
		def, ok := resultDoc.Defs[defName]
		if !ok {
			t.Fatalf("missing %s schema", defName)
		}
		properties, ok := def["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s.properties missing or invalid", defName)
		}
		planStatus, ok := properties["plan_status"].(map[string]any)
		if !ok {
			t.Fatalf("%s.plan_status missing or invalid", defName)
		}
		enumValues, ok := planStatus["enum"].([]any)
		if !ok {
			t.Fatalf("%s.plan_status enum missing or invalid", defName)
		}
		values := make([]string, 0, len(enumValues))
		for _, value := range enumValues {
			values = append(values, value.(string))
		}
		if !containsString(values, "superseded") {
			t.Fatalf("%s.plan_status enum missing superseded: %v", defName, values)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func readSchemaFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("..", "..", "..", "spec", "v1", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}
