# CLAUDE.md — gem-query

**Organization rules (mandatory): https://github.com/nlink-jp/.github/blob/main/CONVENTIONS.md**

## Project overview

Natural language data analysis CLI for DuckDB/SQLite using Vertex AI Gemini.
Users ask questions in natural language, the tool generates SQL, validates
it via dry-run, and executes interactively. Part of util-series.

## Non-negotiable rules

- **Tests are mandatory** — write them with the implementation.
- **Never `go build` directly** — always `make build` (outputs to `dist/`).
- **Docs in sync** — update `README.md` and `README.ja.md` together.
- **Small, typed commits** — `feat:`, `fix:`, `test:`, `chore:`, etc.
- **Security first** — prompt injection defense (nlk/guard), no secrets in code.
- **SELECT only** — never generate or execute write queries (INSERT/UPDATE/DELETE/DDL).

## Build & test

```bash
make build          # → dist/gem-query (CGO_ENABLED=1 required for DuckDB)
make test           # or: go test ./...
make check          # vet → test → build
make build-all      # cross-compile (requires podman/docker for Linux/Windows)
```

## Configuration

Settings are loaded: defaults → TOML file → env vars → CLI flags.

- **Config file**: `~/.config/gem-query/config.toml` (or `-c` flag)
- **Env vars**: `GEMQUERY_*` (tool-specific) > `GOOGLE_CLOUD_*` (generic fallback)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GEMQUERY_PROJECT` | Yes | — | GCP project ID |
| `GEMQUERY_LOCATION` | No | `us-central1` | Vertex AI region |
| `GEMQUERY_MODEL` | No | `gemini-2.5-flash` | Model name |

## Key dependencies

- `google.golang.org/genai` — Google Gemini SDK (Vertex AI backend)
- `github.com/marcboeker/go-duckdb` — DuckDB Go driver (CGO)
- `github.com/nlink-jp/nlk` — guard, backoff, strip, jsonfix
- `github.com/spf13/cobra` — CLI framework
- `github.com/BurntSushi/toml` — config file parsing

## Architecture

- `cmd/` — Cobra CLI (interactive + one-shot modes)
- `internal/config/` — TOML + environment variable configuration
- `internal/gemini/` — Vertex AI Gemini client with retry
- `internal/query/` — SQL generation, dry-run validation, execution engine
- `internal/shell/` — Interactive REPL shell
- `internal/output/` — Table/JSON/CSV formatters
- `internal/security/` — nlk/guard prompt injection defense
