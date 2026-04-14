# gem-query

Natural language data analysis CLI for DuckDB/SQLite using Vertex AI Gemini.

Ask questions in natural language, and the tool generates SQL, validates it
via dry-run, and executes it interactively. Designed as both an interactive
DB shell and a pipe-friendly one-shot CLI.

## Prerequisites

- **Google Cloud project** with the Vertex AI API enabled
- **Application Default Credentials** — run `gcloud auth application-default login`
- **DuckDB** database file (`.duckdb` or `.sqlite`)

## Installation

```bash
git clone https://github.com/nlink-jp/gem-query.git
cd gem-query
make build
# Binary: dist/gem-query
```

> **Note:** DuckDB requires CGO. `make build` sets `CGO_ENABLED=1` automatically.

## Configuration

Settings are loaded in this order (higher priority wins):

1. **Defaults** — built-in values
2. **TOML file** — `~/.config/gem-query/config.toml` (or `-c` flag)
3. **Environment variables** — `GEMQUERY_*` (tool-specific) > `GOOGLE_CLOUD_*` (generic)
4. **CLI flags** — highest priority

### Config file

Copy the example and edit:

```bash
mkdir -p ~/.config/gem-query
cp config.example.toml ~/.config/gem-query/config.toml
```

```toml
[gcp]
project  = "your-project-id"
location = "us-central1"

[model]
name = "gemini-2.5-flash"

[tools]
# jviz_path = "/usr/local/bin/jviz"
```

### Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GEMQUERY_PROJECT` | Yes | — | GCP project ID |
| `GEMQUERY_LOCATION` | No | `us-central1` | Vertex AI region |
| `GEMQUERY_MODEL` | No | `gemini-2.5-flash` | Gemini model name |
| `GEMQUERY_JVIZ_PATH` | No | — | Path to jviz binary |
| `GOOGLE_CLOUD_PROJECT` | — | — | Fallback for `GEMQUERY_PROJECT` |
| `GOOGLE_CLOUD_LOCATION` | — | — | Fallback for `GEMQUERY_LOCATION` |

## Usage

### Interactive mode

```bash
gem-query ./data.duckdb
```

```
gem-query> Show me top 10 customers by total sales

[SQL]
  SELECT customer_name, SUM(amount) AS total
  FROM sales GROUP BY customer_name
  ORDER BY total DESC LIMIT 10;

Execute? [Y/n/e(dit)]: y

+---------------+--------+
| customer_name | total  |
+---------------+--------+
| Acme Corp     | 6500   |
| ...           | ...    |
+---------------+--------+
10 rows

gem-query> /jviz
jviz started. Query results will auto-update in the browser.

gem-query> Now break it down by month
  → SQL generated, executed, table displayed, jviz auto-updates

gem-query> /export json result.json
gem-query> /sql --clipboard
gem-query> /quit
```

### Shell commands

| Command | Description |
|---------|-------------|
| `/sql` | Show last generated SQL |
| `/sql --clipboard` | Copy last SQL to clipboard |
| `/sql <file>` | Save last SQL to file |
| `/export <json\|csv> <file>` | Export result to file |
| `/export <json\|csv> --clipboard` | Export result to clipboard |
| `/summarize` | Summarize last result with LLM |
| `/jviz` | Start live jviz (auto-updates on each query) |
| `/jviz --port <port>` | Start jviz on a specific port |
| `/jviz off` | Stop jviz |
| `/format <table\|json\|csv>` | Change display format |
| `/help` | Show help |
| `/quit` | Exit |

### One-shot mode

```bash
# Table output (default)
gem-query ./data.duckdb "top 10 sales by customer"

# JSON output (pipe-friendly)
gem-query ./data.duckdb "monthly revenue" --format json

# CSV output
gem-query ./data.duckdb "sales by region" --format csv

# With LLM summary
gem-query ./data.duckdb "category breakdown" --summarize

# Pipe to jviz for visualization
gem-query ./data.duckdb "monthly revenue" --format json | jviz
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-c, --config` | `~/.config/gem-query/config.toml` | Config file path |
| `-m, --model` | (from config) | Model name override |
| `--format` | `table` | Output format: `table`, `json`, `csv` |
| `--jviz` | (from config) | Path to jviz binary |
| `--summarize` | `false` | Summarize results with LLM |
| `--debug` | `false` | Enable debug output |

## How It Works

```
Question → LLM generates SQL (with schema context)
            → Dry-run validation (EXPLAIN)
              → Auto-fix loop if syntax error
                → User confirms SQL
                  → Execute → Display results
                    → Context carried to next question
```

1. **Schema awareness** — On startup, loads all table/column metadata from DuckDB
2. **SQL generation** — Sends natural language + schema to Gemini, receives SQL
3. **Dry-run validation** — Runs `EXPLAIN` to catch syntax errors before execution
4. **Auto-fix loop** — If dry-run fails, feeds the error back to Gemini for correction (up to 3 attempts)
5. **User confirmation** — Always shows the proposed SQL and asks for approval
6. **Context continuity** — Previous SQL/results are carried into subsequent questions
7. **Security** — User input is wrapped with nonce-tagged XML (nlk/guard) to prevent prompt injection; only SELECT queries are generated

## Building

```bash
make build       # Build for current platform → dist/gem-query
make build-all   # Cross-compile (requires podman/docker for Linux/Windows)
make test        # Run all tests
make check       # vet → test → build
make clean       # Remove dist/
```

## Documentation

- [RFP](docs/en/gem-query-rfp.md) — Requirements document

## License

See [LICENSE](LICENSE).
