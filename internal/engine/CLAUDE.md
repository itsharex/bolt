# internal/engine

Core download engine — orchestrates the full download lifecycle.

## Files

| File | Purpose |
|---|---|
| `engine.go` | Engine struct, AddDownload, Start/Pause/Resume/Cancel/Shutdown |
| `segment.go` | segmentWorker — per-segment goroutine with retry logic |
| `progress.go` | progressAggregator — collects reports, emits events, persists to DB |
| `probe.go` | HEAD request probing (with GET fallback on 405) |
| `filename.go` | Filename detection + deduplication |
| `httpclient.go` | HTTP client factory with DisableCompression, cookie jar |
| `refresh.go` | Tier 3 manual URL refresh with size validation |

## Download Lifecycle

1. `AddDownload` — validate → probe URL → detect filename → dedup → generate ID → compute segments → insert DB
2. `StartDownload` — load from DB → open file → truncate → launch segment goroutines + progress aggregator
3. Segment workers write via `file.WriteAt` at non-overlapping offsets (no mutex needed)
4. Progress aggregator collects reports, emits events every 500ms, persists to DB every 2s
5. On completion: `SetCompleted` in DB, emit `DownloadCompleted`
6. On error: set status to `error`, emit `DownloadFailed`

## Critical Design: Progress Aggregator Separation

The aggregator maintains its **own copy** of per-segment downloaded/done state, separate from the segment workers' state. This avoids a double-counting bug where both the worker and aggregator would increment the same `Downloaded` field. The worker updates its own `segment.Downloaded` (used for write offsets), and the aggregator tracks independently via `segmentReport` channel messages.

## Retry Logic

- Exponential backoff: 1s → 2s → 4s → 8s → 16s (capped at 60s)
- **Permanent errors** (fail immediately): 404, 403, 410, 416
- **Transient errors** (retry): 5xx, timeout, connection reset, DNS failure, TLS error, io.UnexpectedEOF
- Max retries configurable via `config.MaxRetries`

## Single-Connection Fallback

When `AcceptsRanges=false` or `TotalSize=-1`: use 1 segment, no Range header.

## Graceful Shutdown

`Shutdown()` cancels all contexts → waits for goroutines (10s timeout) → persists progress → sets status to paused → closes files.

## Phase 2 Changes

Engine interface stays identical. Only the calling layer changes — CLI will become an HTTP client instead of embedding the engine directly.
