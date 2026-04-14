// Package cmd implements the gem-query CLI.
package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nlink-jp/gem-query/internal/config"
	"github.com/nlink-jp/gem-query/internal/gemini"
	"github.com/nlink-jp/gem-query/internal/output"
	"github.com/nlink-jp/gem-query/internal/query"
	"github.com/nlink-jp/gem-query/internal/shell"
	"github.com/spf13/cobra"

	_ "github.com/marcboeker/go-duckdb"
)

var version string

// CLI flags
var (
	flagConfigPath string
	flagModel      string
	flagFormat     string
	flagJvizPath   string
	flagSummarize  bool
	flagDebug      bool
)

// Exit codes
const (
	exitOK           = 0
	exitGeneralError = 1
	exitInputError   = 2
	exitAPIError     = 3
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gem-query <database> [question]",
		Short: "Natural language data analysis for DuckDB",
		Long: `gem-query lets you analyze DuckDB/SQLite data using natural language.
It generates SQL from your questions, validates it, and executes it interactively.

Interactive mode:
  gem-query ./data.duckdb

One-shot mode:
  gem-query ./data.duckdb "top 10 sales by customer"
  gem-query ./data.duckdb "monthly revenue" --format json | jviz`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE:         run,
	}

	cmd.Flags().StringVarP(&flagConfigPath, "config", "c", "", "config file path")
	cmd.Flags().StringVarP(&flagModel, "model", "m", "", "model name override")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "output format: table, json, csv (one-shot mode)")
	cmd.Flags().StringVar(&flagJvizPath, "jviz", "", "path to jviz binary for visualization")
	cmd.Flags().BoolVar(&flagSummarize, "summarize", false, "summarize results with LLM (one-shot mode)")
	cmd.Flags().BoolVar(&flagDebug, "debug", false, "enable debug output")

	return cmd
}

// Execute runs the root command.
func Execute(v string) {
	version = v
	cmd := newRootCmd()
	cmd.Version = version

	if err := cmd.Execute(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			os.Exit(ee.code)
		}
		os.Exit(exitGeneralError)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// 1. Load configuration
	cfg, err := config.Load(flagConfigPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	cfg.ApplyFlags(flagModel, flagJvizPath)

	dbPath := args[0]

	// 2. Open DuckDB
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return exitWithCode(fmt.Errorf("open database: %w", err), exitInputError)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return exitWithCode(fmt.Errorf("connect database: %w", err), exitInputError)
	}

	// 3. Create Gemini client
	ctx := context.Background()
	llm, err := gemini.New(ctx, cfg.GCP.Project, cfg.GCP.Location, cfg.Model.Name)
	if err != nil {
		return exitWithCode(fmt.Errorf("gemini client: %w", err), exitAPIError)
	}

	// 4. Create query engine
	engine, err := query.NewEngine(db, llm)
	if err != nil {
		return fmt.Errorf("query engine: %w", err)
	}

	if flagDebug {
		fmt.Fprintf(os.Stderr, "[debug] db=%s model=%s\n", dbPath, cfg.Model.Name)
	}

	// 5. Interactive or one-shot mode
	if len(args) == 1 {
		return runInteractive(ctx, engine, cfg.Tools.JvizPath)
	}
	return runOneShot(ctx, engine, args[1])
}

func runInteractive(ctx context.Context, engine *query.Engine, jvizPath string) error {
	sh := shell.New(engine, jvizPath)
	return sh.Run(ctx)
}

func runOneShot(ctx context.Context, engine *query.Engine, question string) error {
	sqlStr, err := engine.GenerateAndValidate(ctx, question)
	if err != nil {
		return exitWithCode(fmt.Errorf("generate SQL: %w", err), exitAPIError)
	}

	fmt.Fprintf(os.Stderr, "SQL: %s\n", sqlStr)

	result, err := engine.Execute(ctx, sqlStr)
	if err != nil {
		return exitWithCode(fmt.Errorf("execute: %w", err), exitGeneralError)
	}

	switch strings.ToLower(flagFormat) {
	case "json":
		_ = output.FormatJSON(os.Stdout, result)
	case "csv":
		_ = output.FormatCSV(os.Stdout, result)
	default:
		output.FormatTable(os.Stdout, result)
	}

	if flagSummarize {
		summary, err := engine.Summarize(ctx, result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "summarize: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "\n%s\n", summary)
		}
	}

	return nil
}

type exitError struct {
	err  error
	code int
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func exitWithCode(err error, code int) error {
	return &exitError{err: err, code: code}
}
