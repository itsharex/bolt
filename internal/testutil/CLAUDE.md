# internal/testutil

Test helpers shared across packages.

## TestServer

`NewTestServer(size, opts...)` creates an `httptest.Server` that serves deterministic data generated from a seeded PRNG (seed=42). Supports:

- HEAD requests (returns Content-Length + Accept-Ranges)
- Range requests (206 Partial Content with correct Content-Range)
- Full GET (200 with all data)

## Options

| Option | Effect |
|---|---|
| `WithNoRangeSupport()` | Disables range requests, always serves full data |
| `WithLatency(d)` | Adds delay to each request (use for pause/resume tests) |
| `WithFailAfterBytes(n, count)` | Serves only `n` bytes then abruptly closes, for `count` requests |
| `WithContentDisposition(cd)` | Sets Content-Disposition header |
| `WithRedirects(urls)` | Creates redirect chain |
| `WithStatusOverride(code)` | Returns fixed status code for all requests |

## GenerateData

`GenerateData(size)` produces deterministic bytes using `rand.NewSource(42)`. Used for byte-for-byte verification in tests — generate the same data in the test and compare against the downloaded file.
