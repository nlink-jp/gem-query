package cmd

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// TestOpenDatabaseDuckDBIsReadOnly opens an existing DuckDB file the way
// gem-query does: reads work, writes are refused by the engine.
func TestOpenDatabaseDuckDBIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.duckdb")
	w, err := sql.Open("duckdb", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Exec("CREATE TABLE t AS SELECT 1 AS a"); err != nil {
		t.Fatal(err)
	}
	w.Close()

	db, err := openDatabase(path, false)
	if err != nil {
		t.Fatalf("openDatabase: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM t").Scan(&n); err != nil || n != 1 {
		t.Fatalf("read: n=%d err=%v", n, err)
	}
	if _, err := db.Exec("INSERT INTO t VALUES (2)"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("INSERT on gem-query's connection = %v, want a read-only error", err)
	}
}

// TestOpenDatabaseMissingFileFails keeps a mistyped path from silently
// creating an empty database.
func TestOpenDatabaseMissingFileFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.duckdb")
	if db, err := openDatabase(path, false); err == nil {
		db.Close()
		t.Fatal("openDatabase(missing) = nil error, want failure")
	}
}

// TestOpenDatabaseSQLiteIsReadOnly attaches a SQLite file through DuckDB's
// sqlite extension. Every pooled connection must see the file (USE is per
// connection) and refuse writes. Skips when the extension cannot be
// installed (offline).
func TestOpenDatabaseSQLiteIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.sqlite")
	w, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, q := range []string{"INSTALL sqlite", "LOAD sqlite"} {
		if _, err := w.Exec(q); err != nil {
			t.Skipf("sqlite extension unavailable: %v", err)
		}
	}
	for _, q := range []string{
		"ATTACH '" + path + "' AS s (TYPE sqlite)",
		"CREATE TABLE s.t (a INTEGER)",
		"INSERT INTO s.t VALUES (1)",
		"DETACH s",
	} {
		if _, err := w.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	db, err := openDatabase(path, false)
	if err != nil {
		t.Fatalf("openDatabase: %v", err)
	}
	defer db.Close()
	conns := make([]*sql.Conn, 3)
	for i := range conns {
		c, err := db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		conns[i] = c
		var n int
		if err := c.QueryRowContext(t.Context(), "SELECT count(*) FROM t").Scan(&n); err != nil || n != 1 {
			t.Fatalf("connection %d: n=%d err=%v", i, n, err)
		}
	}
	if _, err := conns[2].ExecContext(t.Context(), "INSERT INTO t VALUES (2)"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("INSERT = %v, want a read-only error", err)
	}
}
