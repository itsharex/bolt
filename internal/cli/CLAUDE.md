# internal/cli

CLI client that talks to the bolt daemon via HTTP API and WebSocket.

## Client Struct

`NewClient()` loads config for server port + auth token. No DB access — all operations go through the HTTP API.

## Commands

| Method | CLI Command | Behavior |
|---|---|---|
| `Add` | `bolt add <url>` | POST /api/downloads → print info → WebSocket progress |
| `List` | `bolt list` | GET /api/downloads → tabwriter output |
| `Status` | `bolt status <id>` | GET /api/downloads/{id} → detailed view |
| `Pause` | `bolt pause <id>` | POST /api/downloads/{id}/pause |
| `Resume` | `bolt resume <id>` | POST /api/downloads/{id}/resume → optional WebSocket progress |
| `Cancel` | `bolt cancel <id>` | DELETE /api/downloads/{id} |
| `Refresh` | `bolt refresh <id> <url>` | POST /api/downloads/{id}/refresh |
| `Stop` | `bolt stop` | Read PID file → SIGTERM → poll until stopped |

## Progress Display (`progress.go`)

Terminal progress bar: `\r[filename] ████░░ 47% | 50 MB/100 MB | 12.3 MB/s | ETA 4s`

Connects to WebSocket, filters events by download ID, renders progress until completed/failed.

## HTTP Helpers

`get`, `post`, `put`, `del` — all attach Bearer token. `readError` extracts error from JSON response.
