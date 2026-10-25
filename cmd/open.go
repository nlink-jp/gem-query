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

// lockStatements run once the database is open. A plain SELECT of
// enable_profiling / enable_logging writes files, and reading a URL
// installs an extension; with autoinstall/autoload off and the
// configuration locked, DuckDB refuses both. gem-query changes no
// setting after this point.
var lockStatements = []string{
	"SET autoinstall_known_extensions = false",
	"SET autoload_known_extensions = false",
	"SET lock_configuration = true",
}

// openDatabase opens the database at dbPath read-only. gem-query only
// ever reads, so the engine refuses any write to the user's data even if
// a generated query slips past the SELECT check: a DuckDB file is opened
// with access_mode=read_only, and a SQLite file is attached READ_ONLY to
// an in-memory DuckDB. The configuration is then locked (lockStatements).
// A missing DuckDB file is an error instead of a new empty database.
func openDatabase(dbPath string, debug bool) (*sql.DB, error) {
	db, err := openReadOnly(dbPath, debug)
	if err != nil {
		return nil, err
	}
	for _, stmt := range lockStatements {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("lock configuration (%s): %w", stmt, err)
		}
	}
	return db, nil
}

func openReadOnly(dbPath string, debug bool) (*sql.DB, error) {
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
