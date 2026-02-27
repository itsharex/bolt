# internal/event

Pub/sub event bus for progress reporting and status changes.

## Event Types

| Type | When Emitted |
|---|---|
| `Progress` | Every 500ms during active download |
| `DownloadAdded` | New download created |
| `DownloadCompleted` | All segments finished |
| `DownloadFailed` | Permanent error encountered |
| `DownloadPaused` | Download paused |
| `DownloadResumed` | Download resumed |
| `DownloadRemoved` | Download deleted |
| `RefreshNeeded` | Dead link detected |

## Bus Design

- `sync.RWMutex`: Publish takes RLock (concurrent publishes OK), Subscribe/Unsubscribe take Lock
- Subscribers: `map[int]chan Event` with auto-incrementing ID
- Buffer size: 256 per subscriber
- Non-blocking publish: drops events on full buffer (backpressure safety)
- `Unsubscribe` closes the channel — safe to range over
