# AGENTS.md — gem-query

## Project summary

Natural language data analysis CLI for DuckDB/SQLite using Vertex AI Gemini.
Generates SQL from questions, validates via dry-run, and executes interactively.
Part of util-series.

## Build commands

```bash
make build          # Build → dist/gem-query (CGO_ENABLED=1)
make test           # Run all tests
make test-linux     # same suite on Linux (container)
make build-all      # Cross-compile (requires podman/docker for Linux/Windows)
make verify-release  # gate: notarized, fresh, runs at this version, clean linux archives (run before upload)
make check          # vet → test → build
make clean          # Remove dist/
```

## Module path

`github.com/nlink-jp/gem-query`

## Key structure

```
gem-query/
├── main.go                 ← entry point
├── cmd/root.go             ← cobra command, flag definitions, mode routing
├── internal/
│   ├── config/             ← TOML + env var configuration
│   ├── gemini/             ← Vertex AI Gemini client with retry
│   ├── query/              ← SQL generation, dry-run, auto-fix, execution
│   ├── shell/              ← interactive REPL, jviz integration, clipboard
│   ├── output/             ← table/JSON/CSV formatters
│   └── security/           ← nlk/guard prompt injection defense
├── Makefile
└── docs/                   ← RFP documents
```

## Configuration

Config loaded: defaults → TOML (`~/.config/gem-query/config.toml`) → env vars → CLI flags.

- `GEMQUERY_PROJECT` (required) — GCP project ID
- `GEMQUERY_LOCATION` (optional, default: us-central1) — Vertex AI region
- `GEMQUERY_MODEL` (optional, default: gemini-2.5-flash) — model name
- `GEMQUERY_JVIZ_PATH` (optional) — explicit path to jviz binary
- `GOOGLE_CLOUD_PROJECT` / `GOOGLE_CLOUD_LOCATION` — generic fallback

## Query pipeline

1. User enters natural language question
2. LLM generates SQL (schema + conversation context)
3. Dry-run validation via `EXPLAIN`
4. Auto-fix loop on syntax error (up to 3 attempts)
5. User confirms (accept/reject/edit)
6. Execute and display results
7. Context carried to next question

## Gotchas

- DuckDB requires CGO — `CGO_ENABLED=1` is mandatory for build.
  Cross-compilation needs Podman/Docker for Linux/Windows targets.
- DuckDB DECIMAL type returns a struct (`duckdb.Decimal`), not a float.
  The query engine normalizes this to `float64` before display.
- The driver is `github.com/duckdb/duckdb-go/v2` (`marcboeker/go-duckdb` is
  frozen). Its version encodes DuckDB's: v2.20000.x = DuckDB v2.0.x. MAP
  scans into `duckdb.OrderedMap`, whose `String()` is a debugging form, so
  `normalizeValue` turns it into `map[string]any`. UUID arrives as 16 raw
  bytes; `Execute` formats it by the column's DuckDB type (`UUID`), never by
  the value's shape, so a 16-byte BLOB stays raw.
- Generated SQL must be a single SELECT as classified by DuckDB's parser
  (`selectConn`, via the driver's `Prepare` + `StatementType`) before both the
  dry-run and execution: the driver runs the leading statements of a
  multi-statement query while preparing it, so `db.Exec("EXPLAIN " + sql)`
  alone executed `SELECT 1; DROP TABLE t` before the user confirmed.
- `openDatabase` opens the user's database read-only (DuckDB
  `?access_mode=read_only`; SQLite via a connector that runs `ATTACH IF NOT
  EXISTS … (TYPE sqlite, READ_ONLY)` and `USE db` on every new connection,
  since USE is per connection). Read-only does not stop `COPY … TO` or a
  SELECT of `enable_profiling` / `enable_logging` writing files, so the SELECT
  check stays and `openDatabase` locks the configuration (`lockStatements`:
  extension autoinstall/autoload off, `lock_configuration`). Nothing may `SET`
  after open.
- The native libraries come from `go.duckdb.org/libduckdb/*` through the Go
  module proxy; a restricted `GOPROXY` must allow it.
- Gemini `SystemInstruction` must be nil (not empty string) when no
  system prompt is provided, otherwise the API returns 400.
- Only SELECT queries are generated. The system prompt explicitly
  prohibits INSERT/UPDATE/DELETE/DDL.
- jviz integration requires an explicit binary path (config or `--jviz` flag)
  for security — no PATH auto-detection.
- Authentication via ADC (`gcloud auth application-default login`).
