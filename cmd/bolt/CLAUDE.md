# cmd/bolt

CLI entry point. Parses commands and dispatches to `internal/cli`.

## Commands

| Command | Description |
|---|---|
| `bolt add <url> [flags]` | Add and start a download |
| `bolt list [flags]` | List downloads |
| `bolt status <id>` | Show download details |
| `bolt pause <id>` | Pause a download |
| `bolt resume <id\|all>` | Resume paused download(s) |
| `bolt cancel <id> [--delete-file]` | Cancel and remove a download |
| `bolt refresh <id> <url>` | Update URL for a failed download |
| `bolt version` | Show version |

## Signal Handling

SIGINT/SIGTERM triggers `engine.Shutdown()` which persists all segment progress and sets downloads to paused. This enables resume after Ctrl+C.

## Phase 2 Changes

This file will be refactored. In Phase 2, most commands will become HTTP clients talking to a daemon instead of embedding the engine directly.
