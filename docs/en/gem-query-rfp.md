# RFP: gem-query

> Generated: 2026-04-14
> Status: Draft

## 1. Problem Statement

gem-query is a CLI tool that enables engineers unfamiliar with SQL to interactively analyze DuckDB / SQLite data using natural language. The tool dynamically generates SQL from natural language instructions using an LLM (Vertex AI Gemini), considering the table schema, validates syntax via dry-run with automatic correction, and then executes the query. Results are displayed as raw data by default, with summarization available as an option.

**Target user:** Engineers who can read SQL to some extent but are not comfortable writing complex queries from scratch.

## 2. Functional Specification

### Commands / API Surface

**Launch modes:**

- **Interactive mode (default):** `gem-query ./data.duckdb` launches a DB shell
- **One-shot mode:** `gem-query ./data.duckdb "top 10 sales"` for pipe-friendly single execution

**Interactive flow:**

1. User enters a natural language question
2. LLM generates SQL from schema + conversation context and presents it as a proposal
3. User accepts / rejects / edits
4. Dry-run syntax check — automatic correction loop on error
5. Execute — display results as table + retain structured data internally
6. Conversation context carries over (previous SQL / results inform the next question)

**Shell commands:**

| Command | Description |
|---------|-------------|
| `/export json <file>` | Export results to JSON file |
| `/export csv <file>` | Export results to CSV file |
| `/export json --clipboard` | Copy results as JSON to clipboard |
| `/export csv --clipboard` | Copy results as CSV to clipboard |
| `/sql` | Display the last generated SQL |
| `/sql --clipboard` | Copy the last SQL to clipboard |
| `/sql <file>` | Save the last SQL to file |
| `/summarize` | Summarize results using LLM |
| `/format <table\|json\|csv>` | Switch display format |

**One-shot mode flags:**

| Flag | Description |
|------|-------------|
| `--format json\|csv\|table` | Output format (default: table) |
| `--summarize` | Summarize results with LLM |
| `-c, --config <path>` | Path to config.toml |

### Input / Output

- **Input:** DuckDB file path (launch argument), natural language query (interactive or argument)
- **Output:**
  - Interactive: table display (default), switchable to JSON / CSV via commands, export to file or clipboard
  - One-shot: stdout as table / JSON / CSV (pipe-friendly)
- **Pipe integration:** `gem-query ./data.duckdb "monthly sales" --format json | jviz`

### Configuration

Follows the Vertex AI config.toml unified pattern.

**config.toml:**

```toml
[gcp]
project  = "your-project-id"
location = "us-central1"

[model]
name = "gemini-2.5-flash"
```

**Default path:** `~/.config/gem-query/config.toml`

**Environment variables (priority: env > TOML > defaults):**

- `GEMQUERY_PROJECT` / `GOOGLE_CLOUD_PROJECT`
- `GEMQUERY_LOCATION` / `GOOGLE_CLOUD_LOCATION`
- `GEMQUERY_MODEL`

### External Dependencies

| Dependency | Purpose |
|-----------|---------|
| Vertex AI Gemini API | Natural language to SQL generation, summarization |
| DuckDB | Database engine (local file) |

## 3. Design Decisions

**Language: Go**
- Consistency with the Vertex AI Gemini SDK (google.golang.org/genai), DuckDB (go-duckdb), and nlk library. Same ecosystem as existing gem-search / gem-image.

**TUI: readline-based simple REPL**
- No bubbletea adoption precedent in util-series; avoids maintenance overhead. A DB shell-style prompt-input-result loop is sufficient.

**DB: go-duckdb (database/sql compliant)**
- Standard database/sql interface enables future DB backend extension.

**LLM: google.golang.org/genai (Vertex AI unified SDK)**
- Uses the latest unified SDK, not the deprecated vertexai/genai.

**nlk integration:**

| Package | Purpose |
|---------|---------|
| `nlk/guard` | Prompt injection defense (nonce-tagged XML wrapping) |
| `nlk/backoff` | Gemini API retry (exponential backoff) |
| `nlk/strip` | Strip thinking tags from LLM responses |
| `nlk/jsonfix` | Extract and repair JSON from LLM responses |
| `nlk/validate` | Output validation |

**Differentiation from gem-rag:**
- gem-rag = RAG search over unstructured documents
- gem-query = natural language analysis of structured data (DB)

**Out of scope:**
- DB write operations (INSERT / UPDATE / DELETE) — SELECT only
- Remote DB connections (PostgreSQL, etc.) — not needed at this time
- Dashboard-style visualization — delegated to jviz

**Future integration:**
- jviz (visualization via JSON output pipeline) — planned for Phase 2+

## 4. Development Plan

### Phase 1: Core

- Project scaffold (Cobra CLI, config.toml, Makefile)
- DuckDB connection + table schema retrieval
- Gemini integration (natural language to SQL generation, nlk/guard integration)
- SQL dry-run + automatic correction loop
- Interactive REPL shell (question → SQL proposal → confirmation → execution → table display)
- Conversation context continuity (carry over previous SQL / results)
- Unit tests

**Independently reviewable**

### Phase 2: Features

- One-shot mode (pipe support, `--format`, `--summarize`)
- `/export`, `/sql`, `/summarize` shell commands
- Clipboard integration (macOS: pbcopy, Linux: xclip, Windows: clip)
- jviz integration (JSON output pipe)
- Additional tests

**Independently reviewable**

### Phase 3: Release

- README.md / README.ja.md
- CHANGELOG.md / AGENTS.md
- config.example.toml
- E2E tests (simulation with real data)
- Release (tag, gh release, zip upload)
- Update util-series submodule pointer

## 5. Required API Scopes / Permissions

| Item | Value |
|------|-------|
| Auth method | ADC (Application Default Credentials) |
| Setup | `gcloud auth application-default login` |
| IAM role | `roles/aiplatform.user` |
| OAuth scope | `https://www.googleapis.com/auth/cloud-platform` |

DuckDB uses local file access only. No additional API permissions required.

## 6. Series Placement

**Series:** util-series

**Reason:** Fits naturally with the pipe-friendly data transformation and processing CLI family (gem-search, gem-image, gem-rag, jviz, etc.). gem-query is a data analysis CLI that shares the same design philosophy.

## 7. External Platform Constraints

| Constraint | Mitigation |
|-----------|------------|
| Vertex AI Gemini rate limits (RPM/TPM) | Exponential backoff retry via nlk/backoff |
| DuckDB requires CGO | `CGO_ENABLED=1` needed for cross-compilation; may limit target platforms for build-all |
| Clipboard operations are OS-dependent | Detect and switch between macOS (`pbcopy`), Linux (`xclip`/`xsel`), Windows (`clip`) |

---

## Discussion Log

### Tool naming
Adopted `gem-query` for consistency with gem-search / gem-image / gem-rag. Alternatives considered: `ask-db`, `nlq`, `datawise`.

### TUI framework evaluation
Initially considered bubbletea (Charm ecosystem), but investigation revealed no TUI library adoption in util-series. Combined with DuckDB's CGO dependency increasing build complexity, a readline-based simple REPL was chosen. This is more appropriate for the DB shell-style UX.

### SQL execution confirmation flow
Per user requirement, LLM-generated SQL is never auto-executed; it is always presented to the user for confirmation. Clipboard / file export of generated SQL was added to enhance SQL reusability.

### Result summarization approach
LLM summarization is not performed by default — only via explicit option (`/summarize` command or `--summarize` flag). In the data analysis context, reviewing raw data takes priority.

### jviz integration
Visualization is out of scope for gem-query itself; handled via pipe integration with jviz (`--format json | jviz`). Planned for Phase 2 implementation.

### DuckDB selection rationale
DuckDB was chosen because it can also read SQLite files. Focused on local file-based analysis; remote DB connections are out of scope for now.
