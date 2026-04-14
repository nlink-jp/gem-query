// Package query provides the core SQL generation and execution engine.
package query

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/marcboeker/go-duckdb"
	"github.com/nlink-jp/gem-query/internal/gemini"
	"github.com/nlink-jp/gem-query/internal/security"
	"github.com/nlink-jp/nlk/jsonfix"
	"google.golang.org/genai"
)

const maxFixAttempts = 3

// Result holds the query execution result.
type Result struct {
	Columns []string
	Rows    [][]any
	SQL     string
}

// Engine orchestrates SQL generation and execution.
type Engine struct {
	db      *sql.DB
	llm     *gemini.Client
	history []*genai.Content
	schema  string
}

// NewEngine creates a new query engine.
func NewEngine(db *sql.DB, llm *gemini.Client) (*Engine, error) {
	e := &Engine{db: db, llm: llm}
	schema, err := e.loadSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	e.schema = schema
	return e, nil
}

// GenerateSQL generates SQL from a natural language question.
func (e *Engine) GenerateSQL(ctx context.Context, question string) (string, error) {
	sysPrompt, wrappedQuestion, err := security.WrapPrompt(question)
	if err != nil {
		return "", fmt.Errorf("wrap prompt: %w", err)
	}

	// System instruction: guard + time + schema + context + output constraint
	now := time.Now()
	fullSysPrompt := fmt.Sprintf("%s\n\n"+
		"Current time: %s (timezone: %s)\n\n"+
		"Database schema:\n%s\n\n"+
		"Instructions:\n"+
		"- Respond with ONLY the SQL query, no explanation, no markdown fences.\n"+
		"- Use the conversation history to understand context when the user "+
		"refers to previous queries (e.g. \"break that down by month\", "+
		"\"filter that further\", \"show me the same for region X\").\n"+
		"- When the user uses relative time expressions (e.g. \"yesterday\", "+
		"\"last month\", \"past 7 days\"), calculate the correct dates based on the current time above.",
		sysPrompt,
		now.Format("2006-01-02 15:04:05"), now.Format("MST"),
		e.schema)

	// Build conversation: history (plain text) + current question (guard-wrapped)
	contents := make([]*genai.Content, len(e.history))
	copy(contents, e.history)
	contents = append(contents, genai.NewContentFromText(wrappedQuestion, genai.Role("user")))

	text, err := e.llm.GenerateWithHistory(ctx, fullSysPrompt, contents)
	if err != nil {
		return "", err
	}

	return cleanSQL(text), nil
}

// DryRun validates SQL syntax by using EXPLAIN.
func (e *Engine) DryRun(sqlStr string) error {
	_, err := e.db.Exec("EXPLAIN " + sqlStr)
	return err
}

// FixSQL attempts to fix SQL using LLM feedback from the dry-run error.
func (e *Engine) FixSQL(ctx context.Context, sqlStr string, dryRunErr error) (string, error) {
	prompt := fmt.Sprintf("The following DuckDB SQL has an error:\n\n%s\n\nError: %s\n\n"+
		"Database schema:\n%s\n\n"+
		"Fix the SQL. Respond with ONLY the corrected SQL, no explanation, no markdown fences.",
		sqlStr, dryRunErr.Error(), e.schema)

	text, err := e.llm.GenerateSQL(ctx, "", prompt)
	if err != nil {
		return "", err
	}
	return cleanSQL(text), nil
}

// GenerateAndValidate generates SQL, dry-runs it, and auto-fixes if needed.
func (e *Engine) GenerateAndValidate(ctx context.Context, question string) (string, error) {
	// Record the user's original question with timestamp in history
	e.addToHistory("user", fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), question))

	sqlStr, err := e.GenerateSQL(ctx, question)
	if err != nil {
		return "", err
	}

	for attempt := range maxFixAttempts {
		if err := e.DryRun(sqlStr); err == nil {
			return sqlStr, nil
		} else if attempt < maxFixAttempts-1 {
			fixed, fixErr := e.FixSQL(ctx, sqlStr, err)
			if fixErr != nil {
				return sqlStr, fmt.Errorf("dry-run failed and fix failed: %w", fixErr)
			}
			sqlStr = fixed
		} else {
			return sqlStr, fmt.Errorf("dry-run failed after %d fix attempts: %w", maxFixAttempts, err)
		}
	}

	return sqlStr, nil
}

// Execute runs a SQL query and returns the result.
func (e *Engine) Execute(ctx context.Context, sqlStr string) (*Result, error) {
	rows, err := e.db.QueryContext(ctx, sqlStr)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var resultRows [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		scanArgs := make([]any, len(columns))
		for i := range values {
			scanArgs[i] = &values[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		for i, v := range values {
			values[i] = normalizeValue(v)
		}
		resultRows = append(resultRows, values)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	// Record in history for context continuity
	// The model's response: the SQL it generated + result summary with sample data
	resultSummary := formatResultSummary(columns, resultRows, sqlStr)
	e.addToHistory("model", resultSummary)

	return &Result{
		Columns: columns,
		Rows:    resultRows,
		SQL:     sqlStr,
	}, nil
}

// Summarize asks the LLM to summarize a query result.
func (e *Engine) Summarize(ctx context.Context, result *Result) (string, error) {
	data, err := jsonfix.Extract(resultToJSON(result))
	if err != nil {
		data = resultToJSON(result)
	}

	prompt := fmt.Sprintf("Summarize the following query result concisely:\n\nSQL: %s\n\nData:\n%s",
		result.SQL, data)

	return e.llm.GenerateSQL(ctx, "You are a data analyst. Summarize query results concisely.", prompt)
}

func (e *Engine) addToHistory(role, text string) {
	e.history = append(e.history, genai.NewContentFromText(text, genai.Role(role)))
	// Keep history manageable — retain last 10 exchanges (20 messages)
	// Always trim to even count so user/model pairs stay aligned
	if len(e.history) > 20 {
		e.history = e.history[len(e.history)-20:]
	}
}

// formatResultSummary builds a rich context string for the conversation history.
func formatResultSummary(columns []string, rows [][]any, sqlStr string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Generated SQL: %s\n", sqlStr)
	fmt.Fprintf(&sb, "Result: %d rows, columns: %s\n", len(rows), strings.Join(columns, ", "))

	// Include up to 5 sample rows so the LLM can reference actual data
	sampleCount := len(rows)
	if sampleCount > 5 {
		sampleCount = 5
	}
	if sampleCount > 0 {
		sb.WriteString("Sample data:\n")
		for i := 0; i < sampleCount; i++ {
			sb.WriteString("  ")
			for j, col := range columns {
				if j > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(&sb, "%s=%v", col, rows[i][j])
			}
			sb.WriteString("\n")
		}
		if len(rows) > sampleCount {
			fmt.Fprintf(&sb, "  ... and %d more rows\n", len(rows)-sampleCount)
		}
	}
	return sb.String()
}

func (e *Engine) loadSchema() (string, error) {
	// Try information_schema first (works for native DuckDB)
	schema, err := e.loadSchemaFromInfoSchema()
	if err == nil && schema != "" {
		return schema, nil
	}

	// Fallback: use SHOW TABLES + DESCRIBE (works for attached SQLite)
	return e.loadSchemaFromDescribe()
}

func (e *Engine) loadSchemaFromInfoSchema() (string, error) {
	rows, err := e.db.Query("SELECT table_name, column_name, data_type FROM information_schema.columns ORDER BY table_name, ordinal_position")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var sb strings.Builder
	currentTable := ""
	for rows.Next() {
		var table, column, dataType string
		if err := rows.Scan(&table, &column, &dataType); err != nil {
			return "", err
		}
		if table != currentTable {
			if currentTable != "" {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "TABLE %s:\n", table)
			currentTable = table
		}
		fmt.Fprintf(&sb, "  %s %s\n", column, dataType)
	}
	return sb.String(), rows.Err()
}

func (e *Engine) loadSchemaFromDescribe() (string, error) {
	tableRows, err := e.db.Query("SHOW TABLES")
	if err != nil {
		return "", err
	}

	var tables []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			tableRows.Close()
			return "", err
		}
		tables = append(tables, name)
	}
	tableRows.Close()

	var sb strings.Builder
	for i, table := range tables {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "TABLE %s:\n", table)

		colRows, err := e.db.Query("DESCRIBE " + table)
		if err != nil {
			continue
		}
		for colRows.Next() {
			// DESCRIBE returns: column_name, column_type, null, key, default, extra
			var colName, colType string
			var extra1, extra2, extra3, extra4 sql.NullString
			if err := colRows.Scan(&colName, &colType, &extra1, &extra2, &extra3, &extra4); err != nil {
				colRows.Close()
				break
			}
			fmt.Fprintf(&sb, "  %s %s\n", colName, colType)
		}
		colRows.Close()
	}

	return sb.String(), nil
}

func resultToJSON(r *Result) string {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i, row := range r.Rows {
		sb.WriteString("  {")
		for j, col := range r.Columns {
			if j > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%q: %v", col, row[j])
		}
		sb.WriteString("}")
		if i < len(r.Rows)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("]")
	return sb.String()
}

// normalizeValue converts DuckDB-specific types to display-friendly values.
func normalizeValue(v any) any {
	switch d := v.(type) {
	case duckdb.Decimal:
		if d.Value == nil {
			return 0
		}
		// Convert to float64: value / 10^scale
		f := new(big.Float).SetInt(d.Value)
		divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d.Scale)), nil))
		result, _ := new(big.Float).Quo(f, divisor).Float64()
		return result
	case []byte:
		return string(d)
	default:
		return v
	}
}

func cleanSQL(text string) string {
	text = strings.TrimSpace(text)
	// Remove markdown fences if present
	text = strings.TrimPrefix(text, "```sql")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	return text
}
