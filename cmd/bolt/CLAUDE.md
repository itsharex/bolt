# cmd/bolt

CLI entry point with two modes: daemon (server) and client.

## Mode Detection

- `bolt` (no args) or `bolt start` → `launchHeadless()` daemon mode
- `bolt stop` → send SIGTERM to running daemon
- `bolt add/list/status/pause/resume/cancel/refresh` → HTTP client mode via `runWithClient()`

## Daemon Mode (`launchHeadless`)

Startup sequence:
1. Load config
2. Check PID file → error if already running
3. Write PID file, defer Remove
4. Open SQLite database
5. Create event bus + engine + queue manager
6. Wire queue completion (subscribe to bus, call OnDownloadComplete on completed/failed/paused)
7. Create HTTP server
8. Start queue manager goroutine
9. Start HTTP server goroutine
10. Resume interrupted downloads
11. Block on SIGINT/SIGTERM
12. Shutdown: server → engine → cancel context → PID remove

## Client Mode (`runWithClient`)

1. Create CLI client (loads config for port + token)
2. Check daemon is running (GET /api/stats)
3. Set up signal handler (SIGINT → cancel context)
4. Run the command function

## Commands

| Command | Description |
|---|---|
| `bolt` / `bolt start` | Start daemon |
| `bolt stop` | Stop daemon |
| `bolt add <url> [flags]` | Add download |
| `bolt list [flags]` | List downloads |
| `bolt status <id>` | Show download details |
| `bolt pause <id>` | Pause a download |
| `bolt resume <id>` | Resume a download |
| `bolt cancel <id> [--delete-file]` | Cancel and remove |
| `bolt refresh <id> <url>` | Update URL |
| `bolt version` | Show version |
