# Bolt — Download Manager

Fast, segmented download manager built with Go. See `bolt-prd.md` and `bolt-trd.md` for full specs.

## Project Info

- **Module:** `github.com/fhsinchy/bolt`
- **Go version:** 1.23+
- **Author:** github.com/fhsinchy
- **SQLite driver:** `modernc.org/sqlite` (pure Go, no CGO)
- **ULID library:** `github.com/oklog/ulid/v2`
- **Test framework:** stdlib `testing` + `net/http/httptest` (no external test deps)

## TRD Errata

- TRD §3.1 says `github.com/farhanishmam/bolt` — this is wrong. The correct module path is `github.com/fhsinchy/bolt`.

## Development Phases

### Phase 1: Download Engine + CLI (COMPLETE)
Standalone binary with embedded engine. No HTTP server, no GUI, no browser extension.

**Exit criteria (met):** Can download a file in 16 segments, pause, kill the process, restart, and resume to completion. Verified by `TestIntegration_ExitCriteria`.

**What was built:**
- Step 1: Project scaffolding + models — `internal/model/`
- Step 2: Configuration management — `internal/config/`
- Step 3: Database layer (SQLite/WAL) — `internal/db/`
- Step 4: Event bus (pub/sub) — `internal/event/`
- Step 5: Probe + filename detection + HTTP client — `internal/engine/{probe,filename,httpclient}.go`
- Step 6: Segment downloader + progress aggregator — `internal/engine/{segment,progress}.go`
- Step 7: Engine core (lifecycle orchestration) — `internal/engine/{engine,refresh}.go`
- Step 8: Queue manager — `internal/queue/`
- Step 9: CLI interface — `internal/cli/`, `cmd/bolt/`
- Step 10: Integration tests + Makefile

### Phase 2: HTTP Server + Daemon (NOT STARTED)
Add HTTP server, refactor CLI to become HTTP client, WebSocket for real-time progress.

### Phase 3: Wails GUI + Svelte Frontend (NOT STARTED)
Desktop app with system tray, Wails v2 bindings.

### Phase 4: Browser Extension (NOT STARTED)
Manifest V3 extension for download capture.

## Key Phase 1 Design Decision

The CLI embeds the engine directly (no HTTP server/daemon). Commands like `bolt add <url>` open the DB, create the engine, run the download, and show terminal progress. In Phase 2, we add the HTTP server and refactor the CLI to become an HTTP client. The engine interface stays identical — only the calling layer changes.

## Commands

```
make build       # produces ./bolt binary
make test        # run all tests
make test-race   # run all tests with race detector
make test-v      # run all tests verbose
make clean       # remove binary, clear test cache
```

## Architecture

```
cmd/bolt/main.go          CLI entry point
internal/
  model/                   Shared types, ID generation, formatting
  config/                  config.json management
  db/                      SQLite data access layer
  event/                   Event bus (pub/sub)
  engine/                  Download engine (core business logic)
  queue/                   Queue manager
  cli/                     CLI command implementations
  testutil/                Test helpers (httptest server)
```
