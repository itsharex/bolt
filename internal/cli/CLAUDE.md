# internal/cli

CLI command implementations. Embeds the engine directly (Phase 1 design).

## CLI Struct

`New()` opens DB, loads config, creates engine + queue manager. `Close()` releases DB.

## Commands

| Method | CLI Command | Behavior |
|---|---|---|
| `Add` | `bolt add <url>` | Probe → add → start → show progress until done |
| `List` | `bolt list` | Tabwriter output with ID, filename, size, progress%, status |
| `Status` | `bolt status <id>` | Detailed view with per-segment info |
| `Pause` | `bolt pause <id>` | Pause active or queued download |
| `Resume` | `bolt resume <id\|all>` | Resume with progress display |
| `Cancel` | `bolt cancel <id>` | Stop + delete from DB, optional `--delete-file` |
| `Refresh` | `bolt refresh <id> <url>` | Update URL for failed download |

## Progress Display (`progress.go`)

Terminal progress bar: `\r[filename] ████░░ 47% | 50 MB/100 MB | 12.3 MB/s | ETA 4s`

Subscribes to event bus, renders on `Progress` events, prints final line on `DownloadCompleted`.

## Phase 2 Changes

This package will be refactored to make HTTP calls to the daemon instead of embedding the engine. The user-facing output stays the same.
