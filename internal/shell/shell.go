// Package shell provides the interactive REPL for gem-query.
package shell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nlink-jp/gem-query/internal/output"
	"github.com/nlink-jp/gem-query/internal/query"
)

// Shell is the interactive REPL.
type Shell struct {
	engine     *query.Engine
	lastResult *query.Result
	format     string
	reader     *bufio.Reader
	out        io.Writer
	errOut     io.Writer
	jviz       jvizState
}

// New creates a new interactive shell.
// jvizPath specifies the path to the jviz binary; if empty, /jviz command is disabled.
func New(engine *query.Engine, jvizPath string) *Shell {
	return &Shell{
		engine: engine,
		format: "table",
		reader: bufio.NewReader(os.Stdin),
		out:    os.Stdout,
		errOut: os.Stderr,
		jviz:   jvizState{binPath: jvizPath},
	}
}

// Run starts the REPL loop.
func (s *Shell) Run(ctx context.Context) error {
	defer s.jviz.stop()

	fmt.Fprintln(s.out, "gem-query interactive shell. Type /help for commands, /quit to exit.")
	fmt.Fprintln(s.out)

	for {
		fmt.Fprint(s.out, "gem-query> ")
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(s.out)
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "/") {
			if quit := s.handleCommand(ctx, line); quit {
				return nil
			}
			continue
		}

		s.handleQuery(ctx, line)
	}
}

func (s *Shell) handleCommand(ctx context.Context, line string) (quit bool) {
	parts := strings.Fields(line)
	cmd := parts[0]

	switch cmd {
	case "/quit", "/exit", "/q":
		return true

	case "/help":
		s.printHelp()

	case "/sql":
		s.handleSQLCommand(parts[1:])

	case "/export":
		s.handleExportCommand(parts[1:])

	case "/summarize":
		s.handleSummarize(ctx)

	case "/jviz":
		s.handleJvizCommand(parts[1:])

	case "/format":
		if len(parts) < 2 {
			fmt.Fprintf(s.errOut, "current format: %s\n", s.format)
			return false
		}
		switch parts[1] {
		case "table", "json", "csv":
			s.format = parts[1]
			fmt.Fprintf(s.errOut, "format set to %s\n", s.format)
		default:
			fmt.Fprintf(s.errOut, "unknown format: %s (use table, json, or csv)\n", parts[1])
		}

	default:
		fmt.Fprintf(s.errOut, "unknown command: %s (type /help)\n", cmd)
	}

	return false
}

func (s *Shell) handleQuery(ctx context.Context, question string) {
	fmt.Fprintf(s.errOut, "Generating SQL...\n")

	sqlStr, err := s.engine.GenerateAndValidate(ctx, question)
	if err != nil {
		fmt.Fprintf(s.errOut, "error: %v\n", err)
		return
	}

	// Show SQL proposal
	fmt.Fprintf(s.out, "\n[SQL]\n  %s\n\n", sqlStr)
	fmt.Fprint(s.out, "Execute? [Y/n/e(dit)]: ")

	response, _ := s.reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "n", "no":
		fmt.Fprintln(s.errOut, "cancelled.")
		return
	case "e", "edit":
		fmt.Fprint(s.out, "Enter SQL: ")
		edited, _ := s.reader.ReadString('\n')
		sqlStr = strings.TrimSpace(edited)
		if sqlStr == "" {
			fmt.Fprintln(s.errOut, "cancelled.")
			return
		}
	case "", "y", "yes":
		// proceed
	default:
		fmt.Fprintln(s.errOut, "cancelled.")
		return
	}

	// Execute
	fmt.Fprintf(s.errOut, "Executing...\n")
	result, err := s.engine.Execute(ctx, sqlStr)
	if err != nil {
		fmt.Fprintf(s.errOut, "error: %v\n", err)
		return
	}

	s.lastResult = result
	s.displayResult(result)
}

func (s *Shell) displayResult(r *query.Result) {
	fmt.Fprintln(s.out)
	switch s.format {
	case "json":
		_ = output.FormatJSON(s.out, r)
	case "csv":
		_ = output.FormatCSV(s.out, r)
	default:
		output.FormatTable(s.out, r)
	}
	fmt.Fprintln(s.out)

	// Auto-update jviz if active
	if s.jviz.active {
		if err := s.jviz.update(r); err != nil {
			fmt.Fprintf(s.errOut, "jviz update: %v\n", err)
		}
	}
}

func (s *Shell) handleSQLCommand(args []string) {
	if s.lastResult == nil {
		fmt.Fprintln(s.errOut, "no query executed yet")
		return
	}

	if len(args) == 0 {
		fmt.Fprintln(s.out, s.lastResult.SQL)
		return
	}

	switch args[0] {
	case "--clipboard":
		if err := toClipboard(s.lastResult.SQL); err != nil {
			fmt.Fprintf(s.errOut, "clipboard: %v\n", err)
		} else {
			fmt.Fprintln(s.errOut, "SQL copied to clipboard.")
		}
	default:
		// Treat as file path
		if err := os.WriteFile(args[0], []byte(s.lastResult.SQL+"\n"), 0o644); err != nil {
			fmt.Fprintf(s.errOut, "write file: %v\n", err)
		} else {
			fmt.Fprintf(s.errOut, "SQL saved to %s\n", args[0])
		}
	}
}

func (s *Shell) handleExportCommand(args []string) {
	if s.lastResult == nil {
		fmt.Fprintln(s.errOut, "no query result to export")
		return
	}

	if len(args) < 2 {
		fmt.Fprintln(s.errOut, "usage: /export <json|csv> <file|--clipboard>")
		return
	}

	format := args[0]
	target := args[1]

	var buf strings.Builder
	switch format {
	case "json":
		_ = output.FormatJSON(&buf, s.lastResult)
	case "csv":
		_ = output.FormatCSV(&buf, s.lastResult)
	default:
		fmt.Fprintf(s.errOut, "unknown format: %s (use json or csv)\n", format)
		return
	}

	if target == "--clipboard" {
		if err := toClipboard(buf.String()); err != nil {
			fmt.Fprintf(s.errOut, "clipboard: %v\n", err)
		} else {
			fmt.Fprintf(s.errOut, "%s copied to clipboard.\n", strings.ToUpper(format))
		}
	} else {
		if err := os.WriteFile(target, []byte(buf.String()), 0o644); err != nil {
			fmt.Fprintf(s.errOut, "write file: %v\n", err)
		} else {
			fmt.Fprintf(s.errOut, "exported to %s\n", target)
		}
	}
}

func (s *Shell) handleSummarize(ctx context.Context) {
	if s.lastResult == nil {
		fmt.Fprintln(s.errOut, "no query result to summarize")
		return
	}

	fmt.Fprintf(s.errOut, "Summarizing...\n")
	summary, err := s.engine.Summarize(ctx, s.lastResult)
	if err != nil {
		fmt.Fprintf(s.errOut, "summarize: %v\n", err)
		return
	}
	fmt.Fprintf(s.out, "\n%s\n\n", summary)
}

func (s *Shell) handleJvizCommand(args []string) {
	if len(args) > 0 && args[0] == "off" {
		if !s.jviz.active {
			fmt.Fprintln(s.errOut, "jviz is not active")
			return
		}
		s.jviz.stop()
		fmt.Fprintln(s.errOut, "jviz stopped.")
		return
	}

	if s.jviz.active {
		fmt.Fprintln(s.errOut, "jviz is already active (use /jviz off to stop)")
		return
	}

	var port string
	for i, arg := range args {
		if arg == "--port" && i+1 < len(args) {
			port = args[i+1]
		}
	}

	if err := s.jviz.start(port); err != nil {
		fmt.Fprintf(s.errOut, "jviz: %v\n", err)
		return
	}

	fmt.Fprintln(s.errOut, "jviz started. Query results will auto-update in the browser.")

	// Send current result if available
	if s.lastResult != nil {
		if err := s.jviz.update(s.lastResult); err != nil {
			fmt.Fprintf(s.errOut, "jviz update: %v\n", err)
		}
	}
}

func (s *Shell) printHelp() {
	help := `Commands:
  /sql                    Show last generated SQL
  /sql --clipboard        Copy last SQL to clipboard
  /sql <file>             Save last SQL to file
  /export <json|csv> <file>        Export result to file
  /export <json|csv> --clipboard   Export result to clipboard
  /summarize              Summarize last result with LLM
  /jviz                   Start live jviz (auto-updates on each query)
  /jviz --port <port>     Start jviz on a specific port
  /jviz off               Stop jviz
  /format <table|json|csv>  Change display format
  /help                   Show this help
  /quit                   Exit`
	fmt.Fprintln(s.out, help)
}
