# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.2.0] - 2026-04-14

### Added

- SQLite file support — auto-detect by extension (`.db`, `.sqlite`, `.sqlite3`) or magic bytes
- Automatic `INSTALL sqlite` + `LOAD sqlite` for DuckDB sqlite extension
- Schema loading fallback via `SHOW TABLES` + `DESCRIBE` for attached SQLite databases

### Fixed

- Windows cross-compilation Makefile quoting for LDFLAGS
- Bump container Go image to 1.26.2

## [0.1.0] - 2026-04-14

### Added

- Interactive REPL shell for natural language DB queries
- One-shot mode with pipe-friendly output (`--format json|csv|table`)
- Vertex AI Gemini SQL generation with schema-aware prompts
- SQL dry-run validation via `EXPLAIN` with auto-fix loop (up to 3 attempts)
- User confirmation before SQL execution (accept/reject/edit)
- Conversation context continuity across queries
- DuckDB DECIMAL type normalization for display
- `/export` command (JSON/CSV to file or clipboard)
- `/sql` command (display, clipboard, file export)
- `/summarize` command (LLM-powered result summary)
- `/jviz` live mode (auto-updates browser visualization on each query)
- `/format` command to switch display format
- `--summarize` flag for one-shot mode
- `--jviz` flag for explicit jviz binary path
- TOML config with env var overrides (`GEMQUERY_*`, `GOOGLE_CLOUD_*`)
- Prompt injection defense via nlk/guard nonce-tagged XML wrapping
- Exponential backoff retry for Gemini API (nlk/backoff)
- Cross-compilation support with CGO (DuckDB) via Podman/Docker
