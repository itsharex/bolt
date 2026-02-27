# internal/queue

FIFO queue manager with configurable max concurrent downloads.

## Design

- Uses a `StartFunc` callback to avoid circular import with engine
- `notify` channel (buffered at 1) — "something changed" signal
- Queue order persisted via `queue_order` column in downloads table
- `GetNextQueued` from DB returns lowest queue_order download

## Evaluation Loop

When signaled, `evaluate()` loops: while `activeCount < maxConcurrent`, fetch next queued download from DB and call `startFn`. If start fails, mark download as error and try next.

## Signals

Queue re-evaluates when:
- New download enqueued (`Enqueue`)
- Active download completes/fails/pauses (`OnDownloadComplete`)
- Max concurrent setting changes (`SetMaxConcurrent`)
