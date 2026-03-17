package testutil

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ServerOption func(*serverConfig)

type serverConfig struct {
	noRangeSupport     bool
	latency            time.Duration
	failAfterBytes     int64
	failCount          int
	contentDisposition string
	redirectURLs       []string
	statusOverride     int
	probeRejection     bool
}

func WithNoRangeSupport() ServerOption {
	return func(c *serverConfig) { c.noRangeSupport = true }
}

func WithLatency(d time.Duration) ServerOption {
	return func(c *serverConfig) { c.latency = d }
}

func WithFailAfterBytes(n int64, count int) ServerOption {
	return func(c *serverConfig) { c.failAfterBytes = n; c.failCount = count }
}

func WithContentDisposition(cd string) ServerOption {
	return func(c *serverConfig) { c.contentDisposition = cd }
}

func WithRedirects(urls []string) ServerOption {
	return func(c *serverConfig) { c.redirectURLs = urls }
}

func WithStatusOverride(code int) ServerOption {
	return func(c *serverConfig) { c.statusOverride = code }
}

func WithProbeRejection() ServerOption {
	return func(c *serverConfig) { c.probeRejection = true }
}

// NewTestServer creates an httptest.Server that serves deterministic data of
// the given size. The data is generated from a seeded PRNG so it can be
// reproduced for verification. The server supports HEAD requests and byte
// range requests unless WithNoRangeSupport is used.
func NewTestServer(size int64, opts ...ServerOption) *httptest.Server {
	cfg := &serverConfig{}
	for _, o := range opts {
		o(cfg)
	}

	data := GenerateData(size)

	var mu sync.Mutex
	failsRemaining := cfg.failCount

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.latency > 0 {
			time.Sleep(cfg.latency)
		}

		if cfg.probeRejection {
			if r.Method == http.MethodHead || r.Header.Get("Range") != "" {
				w.WriteHeader(http.StatusGone)
				return
			}
			// Serve full data without Content-Length (worst case)
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		if cfg.statusOverride > 0 {
			w.WriteHeader(cfg.statusOverride)
			return
		}

		if cfg.contentDisposition != "" {
			w.Header().Set("Content-Disposition", cfg.contentDisposition)
		}

		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			if !cfg.noRangeSupport {
				w.Header().Set("Accept-Ranges", "bytes")
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		if cfg.noRangeSupport {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		start, end, err := parseRange(rangeHeader, size)
		if err != nil {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		length := end - start + 1
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)

		chunk := data[start : end+1]

		if cfg.failAfterBytes > 0 {
			mu.Lock()
			shouldFail := failsRemaining > 0
			if shouldFail {
				failsRemaining--
			}
			mu.Unlock()

			if shouldFail && cfg.failAfterBytes < length {
				w.Write(chunk[:cfg.failAfterBytes])
				return // abrupt close
			}
		}

		w.Write(chunk)
	})

	return httptest.NewServer(handler)
}

// NewRedirectServer creates a chain of redirect servers. Each server in the
// chain redirects to the next, and the last one redirects to finalURL.
func NewRedirectServer(finalURL string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalURL, http.StatusFound)
	}))
}

// GenerateData produces deterministic data of the given size using a seeded PRNG.
func GenerateData(size int64) []byte {
	rng := rand.New(rand.NewSource(42))
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rng.Intn(256))
	}
	return data
}

func parseRange(header string, totalSize int64) (start, end int64, err error) {
	// Expected format: "bytes=START-END" or "bytes=START-"
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range header: %s", header)
	}
	spec := strings.TrimPrefix(header, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range spec: %s", spec)
	}

	start, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start: %w", err)
	}

	if parts[1] == "" {
		end = totalSize - 1
	} else {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end: %w", err)
		}
	}

	if start > end || start >= totalSize {
		return 0, 0, fmt.Errorf("range out of bounds: %d-%d/%d", start, end, totalSize)
	}
	if end >= totalSize {
		end = totalSize - 1
	}

	return start, end, nil
}
