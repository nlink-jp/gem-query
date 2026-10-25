# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- **DuckDB driver moved to `github.com/duckdb/duckdb-go/v2` v2.20000.0, which
  bundles DuckDB v2.0.0 LTS** (was `github.com/marcboeker/go-duckdb` v1.8.5 with
  DuckDB v1.1.3). The old module path no longer receives releases, so it could
  never take a DuckDB fix. DuckDB 2.0 uses a new SQL parser by default; the
  dry-run / auto-fix loop is unchanged and still catches what it rejects.
  Database files gem-query opens stay readable by older DuckDB versions; only
  files *created* by DuckDB 2.0 are not.
- **A MAP column now prints as `map[k:1]` and encodes as a JSON object.** The
  new driver returns MAP as an ordered map whose own text form is a debugging
  string; gem-query turns it into a string-keyed map, which also lets
  `--format json` encode a MAP column the old driver's value could not.
- **Values inside a LIST or STRUCT are normalized like top-level ones.** A MAP
  nested in a list prints as `[map[k:1]]` rather than the driver's debugging
  form, and a DECIMAL inside a list prints as a number instead of its raw
  struct.

### Security

- **CVE-2026-111497 (Brotli decoder out-of-bounds write) is not fixed yet.**
  DuckDB's parquet extension vendors Brotli 1.1.0, and every DuckDB release so
  far — v2.0.0 included — still does; the upstream fix (duckdb/duckdb#27791) was
  unmerged on 2026-10-25. It is reachable only when a query reads a crafted
  Brotli-compressed Parquet file (`read_parquet`). This release moves gem-query
  onto the maintained driver line so the fix lands as a dependency bump once
  DuckDB ships it.
- **Go dependencies with known vulnerabilities reachable from the binary are
  updated**: `golang.org/x/net` v0.60.0 (GO-2026-6617, -6612, -6611, -6610,
  -6603, -5026, -4918), `google.golang.org/grpc` v1.83.1 (GO-2026-6348,
  -6061), `golang.org/x/text` v0.42.0 (GO-2026-5970) and
  `go.opentelemetry.io/otel` v1.44.0 (GO-2026-5506), and `grpc` further to
  v1.83.2 (GO-2026-6443). `govulncheck` now reports none reachable.
- **The Linux and Windows binaries are built with Go 1.26.10** (was 1.26.2,
  whose standard library has reachable vulnerabilities in `net/http`,
  `crypto/tls` and `crypto/x509`, fixed by 1.26.9 / 1.26.10). The container
  image is now `golang:1.26.10`.

### Fixed

- **`make verify-release` now fails closed.** Its last block chained unzip, the
  packaged binary's `--version` and `spctl` with `&&` and ended the whole chain
  in `|| true`, so a zip that did not unpack or a binary that did not run exited
  0 and the upload proceeded. Each step is now judged on its own, the packaged
  binary's `--version` must contain the tag being released, and only the
  informational `spctl` line may be ignored. Matches the org template
  (CONVENTIONS.md §Code Signing → Verifying a release).
- **The Linux archives no longer carry macOS file metadata.** macOS `tar` wrote
  each bundled file's extended attributes (`com.apple.provenance`, and a Dropbox
  attribute where the tree is synced) into the `.tar.gz` twice: as AppleDouble
  `._` members, which GNU tar extracts as stray `._<name>` files beside the real
  ones, and as `LIBARCHIVE.xattr.*` / `SCHILY.xattr.*` pax headers, which it
  reports as unknown keywords. `make package` now archives with
  `COPYFILE_DISABLE=1 tar --no-xattrs`; each setting stops one of the two.
  Archives already published still carry them; the files themselves are
  unaffected.

### Internal

- `make verify-release` also judges each Linux archive: no AppleDouble or other
  macOS metadata members — listed with `--options 'tar:!mac-ext'`, because a
  plain macOS listing folds `._` members away — no extended attributes as pax
  headers, and exactly the canonical binary, `README.md` and `LICENSE`, compared
  in the C locale.
- The Linux-archive check in `make verify-release` reads each archive's pax
  headers with Python's `tarfile` instead of grepping the decompressed stream,
  which also matched file text that names the keywords (a bundled CHANGELOG,
  for one).

## [0.4.0] - 2026-07-12

### Removed

- **darwin/amd64 (Intel) pre-built binary.** macOS releases now ship
  **arm64 only**, per the org-wide policy (darwin is Apple-Silicon only; no
  universal binaries). Intel Mac users can build from source.

### Changed

- **Linux release archives are now `.tar.gz`** (darwin/windows remain `.zip`),
  per `nlink-jp/.github` CONVENTIONS.md §Release Archive Standard.
- **`LICENSE` is now bundled** in every release archive alongside `README.md`.
- **darwin code-signature identifier** is now the canonical `gem-query`
  (was `gem-query-darwin-arm64`), set via `codesign -i` so it stays stable
  after the archived binary is renamed to its canonical name.

No change to the binary's behaviour — a packaging / build-config release.

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
