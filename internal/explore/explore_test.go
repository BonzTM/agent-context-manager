package explore

import (
	"strings"
	"testing"
)

func TestDescribeJSONObject(t *testing.T) {
	t.Parallel()
	content := `{"name":"widget","count":3,"tags":["a","b"],"nested":{"x":1},"ok":true,"none":null}`
	sum, extractor, ok := Describe(content)
	if !ok || extractor != ExtractorJSON {
		t.Fatalf("Describe = (%q, %q, %v), want json extractor", sum, extractor, ok)
	}
	for _, want := range []string{"JSON object, 6 keys", "count (number)", "tags (array[2])", "nested (object[1])", "none (null)"} {
		if !strings.Contains(sum, want) {
			t.Errorf("summary missing %q:\n%s", want, sum)
		}
	}
}

func TestDescribeJSONArray(t *testing.T) {
	t.Parallel()
	sum, extractor, ok := Describe(`[{"id":1,"label":"x"},{"id":2,"label":"y"}]`)
	if !ok || extractor != ExtractorJSON {
		t.Fatalf("Describe = (%q, %q, %v)", sum, extractor, ok)
	}
	if !strings.Contains(sum, "array, 2 elements") || !strings.Contains(sum, "id") {
		t.Fatalf("array summary lacks shape: %s", sum)
	}
}

func TestDescribeCSV(t *testing.T) {
	t.Parallel()
	content := "id,name,score\n1,alpha,10\n2,beta,20\n3,gamma,30"
	sum, extractor, ok := Describe(content)
	if !ok || extractor != ExtractorCSV {
		t.Fatalf("Describe = (%q, %q, %v), want csv extractor", sum, extractor, ok)
	}
	if !strings.Contains(sum, "3 rows × 3 columns") || !strings.Contains(sum, "header: id, name, score") {
		t.Fatalf("csv summary wrong: %s", sum)
	}
}

func TestDescribeTSV(t *testing.T) {
	t.Parallel()
	sum, extractor, ok := Describe("a\tb\n1\t2\n3\t4")
	if !ok || extractor != ExtractorCSV || !strings.Contains(sum, "TSV") {
		t.Fatalf("Describe = (%q, %q, %v), want TSV", sum, extractor, ok)
	}
}

func TestDescribeSQL(t *testing.T) {
	t.Parallel()
	content := `CREATE TABLE widgets (id TEXT PRIMARY KEY);
INSERT INTO widgets VALUES ('a');
SELECT w.id FROM widgets w JOIN orders o ON o.widget_id = w.id;`
	sum, extractor, ok := Describe(content)
	if !ok || extractor != ExtractorSQL {
		t.Fatalf("Describe = (%q, %q, %v), want sql extractor", sum, extractor, ok)
	}
	for _, want := range []string{"3 statements", "1 CREATE", "1 INSERT", "1 SELECT", "widgets", "orders"} {
		if !strings.Contains(sum, want) {
			t.Errorf("sql summary missing %q:\n%s", want, sum)
		}
	}
}

func TestDescribeCode(t *testing.T) {
	t.Parallel()
	content := `package widgets

import "fmt"

type Widget struct {
	ID string
}

func New(id string) *Widget {
	return &Widget{ID: id}
}

func (w *Widget) Print() {
	fmt.Println(w.ID)
}`
	sum, extractor, ok := Describe(content)
	if !ok || extractor != ExtractorCode {
		t.Fatalf("Describe = (%q, %q, %v), want code extractor", sum, extractor, ok)
	}
	for _, want := range []string{"top-level declarations", "package widgets", "type Widget struct", "func New(id string) *Widget"} {
		if !strings.Contains(sum, want) {
			t.Errorf("code summary missing %q:\n%s", want, sum)
		}
	}
}

func TestDescribeProseFallsThrough(t *testing.T) {
	t.Parallel()
	prose := strings.Repeat("This is a long narrative paragraph about nothing in particular. ", 50)
	if sum, extractor, ok := Describe(prose); ok {
		t.Fatalf("prose should not match an extractor, got (%q, %q)", sum, extractor)
	}
	// A sentence that merely starts with a SQL keyword is not a SQL script.
	if _, _, ok := Describe("Select the best option from the list below and explain why."); ok {
		t.Fatal("keyword-leading prose misclassified as SQL")
	}
	if _, _, ok := Describe(""); ok {
		t.Fatal("empty content should not match")
	}
}

func TestDescribeClampsOutput(t *testing.T) {
	t.Parallel()
	// A very wide CSV header must clamp rather than balloon.
	header := make([]string, 200)
	for i := range header {
		header[i] = strings.Repeat("column", 3)
	}
	content := strings.Join(header, ",") + "\n" + strings.Join(header, ",")
	sum, _, ok := Describe(content)
	if !ok {
		t.Fatal("wide csv should still match")
	}
	if len([]rune(sum)) > maxSummaryChars+1 {
		t.Fatalf("summary not clamped: %d runes", len([]rune(sum)))
	}
}
