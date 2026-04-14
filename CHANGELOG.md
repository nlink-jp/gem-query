# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

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
