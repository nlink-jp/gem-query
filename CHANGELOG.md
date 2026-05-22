# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.3.2] - 2026-05-22

### Added

- **Pre-built binary releases for the first time.** A new `package`
  target produces zipped binaries for darwin/amd64, darwin/arm64,
  linux/amd64, linux/arm64, and windows/amd64. Previously gem-query
  was installed via `go install` only. Asset naming:
  `gem-query-vX.Y.Z-<os>-<arch>.zip`.
- **Darwin builds are Developer ID signed and Apple-notarized.**
  `make package` runs `scripts/codesign-darwin.sh` per darwin
  binary and `scripts/notarize-darwin.sh` per darwin zip, following
  the org-wide convention in `nlink-jp/.github` CONVENTIONS.md
  §Code Signing. End users no longer need to bypass Gatekeeper
  with right-click → Open; local Dropbox-synced
  (FileProvider-managed) install paths no longer SIGKILL the
  binary on launch.

### Fixed

- **Windows CGO cross-compile now works inside the build
  container.** The build-windows target was previously broken due
  to ABI mismatches between the prebuilt `libduckdb.a`
  (UCRT-based) and Debian's default `gcc-mingw-w64-x86-64`
  toolchain (MSVCRT-based). Switched to the UCRT64 mingw variant
  (`gcc-mingw-w64-ucrt64` + `g++-mingw-w64-ucrt64`) and added a
  one-shot `ranlib` pass over all toolchain archives to fix
  Debian's archive-without-index packaging quirk. Result:
  go-duckdb's Windows symbols (`std::basic_streambuf::seekpos`
  with `_Mbstatet`, pthread, etc.) now resolve correctly.

No behaviour change to the binary itself — feature-wise this is
identical to v0.3.1.

## [0.3.1] - 2026-05-03

### Fixed

- Bump nlk to v0.5.2 to pick up the strip fix: think-tag handling
  no longer truncates LLM responses that explain the literal
  `<think>` tag inside a markdown inline-code span.

## [0.3.0] - 2026-04-14

### Added

- Current date/time and timezone in system prompt for relative time resolution ("yesterday", "last month")
- Timestamped user questions in conversation history
- Rich result context in history (SQL + sample data up to 5 rows)
- Explicit LLM instruction to use conversation history for follow-up queries

### Fixed

- Multi-turn context continuity — LLM now correctly resolves references like "filter that further", "break that down by month"

## [0.2.1] - 2026-04-14

### Fixed

- Backspace key now works correctly with multibyte (Japanese/CJK) characters
- Replaced `bufio.Reader` with `peterh/liner` for proper terminal line editing

### Added

- Command history navigation (Up/Down arrow keys)
- Ctrl-C to cancel input, Ctrl-D to exit

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
