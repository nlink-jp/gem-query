package query

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"

	duckdb "github.com/duckdb/duckdb-go/v2"
)

// errNotSelect is returned for generated SQL that is not exactly one
// SELECT statement. gem-query only reads; the database is also opened
// read-only, but that does not stop COPY ... TO, ATTACH or INSTALL, and
// the driver runs every statement but the last of a multi-statement
// query while preparing it — before any dry-run or confirmation.
var errNotSelect = errors.New("only a single SELECT statement is allowed; a second statement after a semicolon, INSERT/UPDATE/DELETE, COPY, ATTACH, SET and other statements are rejected")

// selectConn reserves a connection and checks, with DuckDB's own parser,
// that query is exactly one statement of type SELECT. The caller runs
// query on the returned connection and closes it.
func selectConn(ctx context.Context, db *sql.DB, query string) (*sql.Conn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	typ, err := statementType(conn, query)
	if err == nil && typ != duckdb.STATEMENT_TYPE_SELECT {
		err = errNotSelect
	}
	if err != nil {
		conn.Close()
		// The driver refuses a multi-statement query with a message
		// pointing at PrepareContext; say what the SQL must change.
		if strings.Contains(err.Error(), "multi-statement") {
			err = errNotSelect
		}
		return nil, err
	}
	return conn, nil
}

// statementType parses query with the driver's Prepare, which refuses a
// multi-statement query instead of executing its leading statements, and
// returns the type DuckDB assigns.
func statementType(conn *sql.Conn, query string) (duckdb.StmtType, error) {
	var typ duckdb.StmtType
	err := conn.Raw(func(dc any) error {
		st, err := dc.(driver.Conn).Prepare(query)
		if err != nil {
			return err
		}
		defer st.Close()
		typ, err = st.(*duckdb.Stmt).StatementType()
		return err
	})
	return typ, err
}
