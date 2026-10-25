package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestExecuteNormalizesDriverTypes pins what each DuckDB type looks like
// after normalizeValue, so a driver upgrade that changes the Go value a
// type scans into shows up here instead of in the user's output.
func TestExecuteNormalizesDriverTypes(t *testing.T) {
	db := openTestDB(t)
	e, err := NewEngine(db, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	cases := []struct {
		expr string
		want string
	}{
		{"42::INTEGER", "int32(42)"},
		{"42::BIGINT", "int64(42)"},
		{"1.5::DOUBLE", "float64(1.5)"},
		{"12.34::DECIMAL(10,2)", "float64(12.34)"},
		{"-0.05::DECIMAL(18,3)", "float64(-0.05)"},
		{"'hello'::VARCHAR", `string("hello")`},
		{"true", "bool(true)"},
		{"NULL", "<nil>(<nil>)"},
		{`'{"a":1}'::JSON`, "map[string]interface {}(map[a:1])"},
		{"'\\xAA\\xBB'::BLOB", `string("\xaa\xbb")`},
		{"'00112233-4455-6677-8899-aabbccddeeff'::UUID", `string("\x00\x11\"3DUfw\x88\x99\xaa\xbb\xcc\xdd\xee\xff")`},
		{"170141183460469231731687303715884105727::HUGEINT", "*big.Int(170141183460469231731687303715884105727)"},
		{"DATE '2026-01-02'", "time.Time(2026-01-02 00:00:00 +0000 UTC)"},
		{"TIMESTAMP '2026-01-02 03:04:05'", "time.Time(2026-01-02 03:04:05 +0000 UTC)"},
		{"INTERVAL 3 DAY", "duckdb.Interval({3 0 0})"},
		{"[1, 2]", "[]interface {}([1 2])"},
		{"{'k': 'v'}", "map[string]interface {}(map[k:v])"},
		{"MAP {'k': 1}", "map[string]interface {}(map[k:1])"},
		{"MAP {1: 'x', 2: 'y'}", "map[string]interface {}(map[1:x 2:y])"},
		{"[MAP {'k': 1}]", "[]interface {}([map[k:1]])"},
		{"{'m': MAP {'k': 1}}", "map[string]interface {}(map[m:map[k:1]])"},
		{"[1.25::DECIMAL(4,2)]", "[]interface {}([1.25])"},
	}
	for _, c := range cases {
		t.Run(c.expr, func(t *testing.T) {
			r, err := e.Execute(context.Background(), "SELECT "+c.expr+" AS v")
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(r.Rows) != 1 || len(r.Rows[0]) != 1 {
				t.Fatalf("want 1x1 result, got %v", r.Rows)
			}
			v := r.Rows[0][0]
			got := fmt.Sprintf("%T(%v)", v, v)
			if s, ok := v.(string); ok {
				got = fmt.Sprintf("string(%q)", s)
			}
			if got != c.want {
				t.Errorf("SELECT %s: got %s, want %s", c.expr, got, c.want)
			}
		})
	}
}

// TestExecuteMapEncodesAsJSON guards --format json: a MAP column must encode
// as a JSON object rather than fail the whole result.
func TestExecuteMapEncodesAsJSON(t *testing.T) {
	db := openTestDB(t)
	e, err := NewEngine(db, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	r, err := e.Execute(context.Background(), "SELECT MAP {'a': 1, 'b': 2} AS m")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	b, err := json.Marshal(r.Rows[0][0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if got, want := string(b), `{"a":1,"b":2}`; got != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}

// TestExecuteAndDryRunRejectWrites drives generated SQL that is not a single
// SELECT through both DryRun (which runs before the user confirms) and
// Execute, and checks the database and filesystem are unchanged — the
// driver runs the leading statements of a multi-statement query while
// preparing it, so an error alone would not prove nothing ran.
func TestExecuteAndDryRunRejectWrites(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec("CREATE TABLE t AS SELECT 1 AS a"); err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(db, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.csv")
	for _, q := range []string{
		"SELECT 1; DROP TABLE t",
		"SELECT * FROM t; INSERT INTO t VALUES (2)",
		"WITH c AS (SELECT 2) INSERT INTO t SELECT * FROM c",
		"DELETE FROM t",
		"COPY t TO '" + out + "'",
		"ATTACH ':memory:' AS m",
		"SET threads = 1",
		"INSTALL httpfs",
		"CALL pragma_version()",
		"EXPLAIN ANALYZE DELETE FROM t",
	} {
		if err := e.DryRun(q); err == nil {
			t.Errorf("DryRun(%q) = nil, want rejected", q)
		}
		if _, err := e.Execute(context.Background(), q); err == nil {
			t.Errorf("Execute(%q) = nil, want rejected", q)
		}
		var n, dbs int
		if err := db.QueryRow("SELECT count(*) FROM t").Scan(&n); err != nil || n != 1 {
			t.Fatalf("after %q: table t changed (rows=%d, err=%v)", q, n, err)
		}
		db.QueryRow("SELECT count(*) FROM duckdb_databases() WHERE NOT internal").Scan(&dbs)
		if dbs != 1 {
			t.Fatalf("after %q: %d attached databases, want 1", q, dbs)
		}
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("COPY wrote %s", out)
	}
	if err := e.DryRun("SELECT 1; DROP TABLE t"); !errors.Is(err, errNotSelect) {
		t.Errorf("multi-statement DryRun error = %v, want errNotSelect", err)
	}
	for _, q := range []string{"SELECT * FROM t", "FROM t", "WITH c AS (SELECT 1) SELECT * FROM c", "DESCRIBE t", "SELECT a FROM t;"} {
		if err := e.DryRun(q); err != nil {
			t.Errorf("DryRun(%q) = %v, want allowed", q, err)
		}
		if _, err := e.Execute(context.Background(), q); err != nil {
			t.Errorf("Execute(%q) = %v, want allowed", q, err)
		}
	}
}

// TestNewEngineLoadsSchemaFromDatabaseFile opens an on-disk database the way
// gem-query does for a .duckdb argument and checks the schema reaches the
// prompt.
func TestNewEngineLoadsSchemaFromDatabaseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.duckdb")
	db, err := sql.Open("duckdb", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE sales (region VARCHAR, amount DECIMAL(10,2))"); err != nil {
		t.Fatalf("create: %v", err)
	}
	e, err := NewEngine(db, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	want := "TABLE sales:\n  region VARCHAR\n  amount DECIMAL(10,2)\n"
	if e.schema != want {
		t.Errorf("schema = %q, want %q", e.schema, want)
	}
}

// TestDryRunRejectsInvalidSQL keeps the EXPLAIN-based validation honest: a
// parser change must still reject what the auto-fix loop relies on catching.
func TestDryRunRejectsInvalidSQL(t *testing.T) {
	db := openTestDB(t)
	e, err := NewEngine(db, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := e.DryRun("SELECT 1"); err != nil {
		t.Errorf("DryRun(valid) = %v", err)
	}
	if err := e.DryRun("SELEC 1 FROM"); err == nil {
		t.Error("DryRun(invalid) = nil, want error")
	}
	if err := e.DryRun("SELECT * FROM no_such_table"); err == nil {
		t.Error("DryRun(missing table) = nil, want error")
	}
}
