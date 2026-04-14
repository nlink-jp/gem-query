package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nlink-jp/gem-query/internal/query"
)

func TestFormatTable(t *testing.T) {
	r := &query.Result{
		Columns: []string{"name", "age"},
		Rows: [][]any{
			{"Alice", 30},
			{"Bob", 25},
		},
	}

	var buf bytes.Buffer
	FormatTable(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "Alice") {
		t.Error("table should contain Alice")
	}
	if !strings.Contains(out, "2 rows") {
		t.Error("table should show row count")
	}
}

func TestFormatTable_Empty(t *testing.T) {
	r := &query.Result{
		Columns: []string{"id"},
		Rows:    nil,
	}

	var buf bytes.Buffer
	FormatTable(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "0 rows") {
		t.Error("empty result should show 0 rows")
	}
}

func TestFormatJSON(t *testing.T) {
	r := &query.Result{
		Columns: []string{"name", "age"},
		Rows: [][]any{
			{"Alice", 30},
		},
	}

	var buf bytes.Buffer
	if err := FormatJSON(&buf, r); err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `"name"`) {
		t.Error("JSON should contain column name")
	}
	if !strings.Contains(out, "Alice") {
		t.Error("JSON should contain value")
	}
}

func TestFormatCSV(t *testing.T) {
	r := &query.Result{
		Columns: []string{"name", "age"},
		Rows: [][]any{
			{"Alice", 30},
			{"Bob", 25},
		},
	}

	var buf bytes.Buffer
	if err := FormatCSV(&buf, r); err != nil {
		t.Fatalf("FormatCSV: %v", err)
	}
	out := buf.String()

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Errorf("CSV should have 3 lines (header + 2 rows), got %d", len(lines))
	}
	if lines[0] != "name,age" {
		t.Errorf("CSV header = %q, want name,age", lines[0])
	}
}
