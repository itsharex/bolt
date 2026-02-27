# internal/model

Shared types used across all packages. No external dependencies except `oklog/ulid/v2`.

## Key Types

- **`Status`** — string enum: `queued`, `active`, `paused`, `completed`, `error`, `refresh`
- **`Download`** — full download record with URL, filename, size, headers, status, timestamps
- **`Segment`** — byte range within a download (DownloadID, Index, StartByte, EndByte, Downloaded, Done)
- **`ProbeResult`** — HEAD request response: size, range support, filename, ETag, content type
- **`ProgressUpdate`** / **`SegmentProgress`** — real-time progress data for event emission
- **`AddRequest`** / **`ListFilter`** — input types for engine API
- **`Checksum`** — algorithm + value pair

## ID Generation (`id.go`)

Uses ULID with `d_` prefix. Monotonic entropy from `crypto/rand` ensures IDs are:
- Globally unique
- Lexicographically sortable by creation time
- URL-safe (no hyphens)

## Formatting (`format.go`)

- `FormatBytes(int64)` — human-readable bytes (e.g., "1.5 MB")
- `FormatSpeed(int64)` — bytes/sec display (e.g., "10.0 MB/s")
- `FormatETA(int)` — seconds to human time (e.g., "1h2m3s")
- `ParseRate(string)` — parses "10MB" or "1.5GB/s" to bytes

## Sentinel Errors (`errors.go`)

`ErrNotFound`, `ErrAlreadyActive`, `ErrAlreadyPaused`, `ErrAlreadyCompleted`, `ErrInvalidURL`, `ErrInvalidSegments`, `ErrMaxRetriesExceeded`, `ErrSizeMismatch`, `ErrProbeRejected`, `ErrDuplicateURL`
