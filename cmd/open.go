package cmd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strings"
	"sync"

	duckdb "github.com/duckdb/duckdb-go/v2"
)

// openDatabase opens the database at dbPath read-only. gem-query only
// ever reads, so the engine refuses any write to the user's data even if
// a generated query slips past the SELECT check: a DuckDB file is opened
// with access_mode=read_only, and a SQLite file is attached READ_ONLY to
// an in-memory DuckDB. A missing DuckDB file is an error instead of a new
// empty database.
func openDatabase(dbPath string, debug bool) (*sql.DB, error) {
	if !isSQLiteFile(dbPath) {
		db, err := sql.Open("duckdb", dbPath+"?access_mode=read_only")
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("connect database: %w", err)
		}
		return db, nil
	}

	// SQLite files go through DuckDB's sqlite extension, attached to an
	// in-memory DuckDB. ATTACH is shared by the whole database but USE is
	// per connection, so every new connection attaches (once) and selects
	// the file itself.
	escapedPath := strings.ReplaceAll(dbPath, "'", "''")
	var install sync.Once
	connector, err := duckdb.NewConnector("", func(execer driver.ExecerContext) error {
		ctx := context.Background()
		install.Do(func() {
			if _, err := execer.ExecContext(ctx, "INSTALL sqlite", nil); err != nil && debug {
				fmt.Fprintf(os.Stderr, "[debug] INSTALL sqlite: %v\n", err)
			}
		})
		for _, stmt := range []string{
			"LOAD sqlite",
			"ATTACH IF NOT EXISTS '" + escapedPath + "' AS db (TYPE sqlite, READ_ONLY)",
			"USE db",
		} {
			if _, err := execer.ExecContext(ctx, stmt, nil); err != nil {
				return fmt.Errorf("attach sqlite database: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db := sql.OpenDB(connector)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
