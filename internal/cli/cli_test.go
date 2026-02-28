package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fhsinchy/bolt/internal/model"
)

// newTestClient starts an httptest.Server backed by handler and returns
// a Client whose baseURL and token point at that server.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	return &Client{
		baseURL: ts.URL,
		token:   "test-token",
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// ---------- CheckDaemon ----------

func TestCheckDaemon_Success(t *testing.T) {
	var gotAuth string

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"active_count":    0,
			"queued_count":    0,
			"completed_count": 0,
			"version":         "test",
		})
	})

	if err := c.CheckDaemon(); err != nil {
		t.Fatalf("CheckDaemon returned error: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("expected auth header %q, got %q", "Bearer test-token", gotAuth)
	}
}

func TestCheckDaemon_NotRunning(t *testing.T) {
	// Client pointing at a port that is not listening.
	c := &Client{
		baseURL: "http://127.0.0.1:1", // port 1 should always refuse connections
		token:   "test-token",
		http:    &http.Client{Timeout: 2 * time.Second},
	}

	err := c.CheckDaemon()
	if err == nil {
		t.Fatal("expected error when daemon is not running, got nil")
	}

	if !strings.Contains(err.Error(), "daemon not running") {
		t.Errorf("expected error to contain %q, got %q", "daemon not running", err.Error())
	}
}

// ---------- Add ----------

func TestAdd_Success(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   model.AddRequest
		captured  bool
	)

	now := time.Now().Truncate(time.Second)
	dl := model.Download{
		ID:           "d_01TESTID00000000000000000",
		URL:          "http://example.com/file.bin",
		Filename:     "file.bin",
		Dir:          "/tmp/downloads",
		TotalSize:    1024 * 1024,
		Downloaded:   0,
		Status:       model.StatusQueued,
		SegmentCount: 8,
		CreatedAt:    now,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Only capture the first request (the POST to /api/downloads).
		// The Add method also attempts a WebSocket connection after success,
		// which arrives as a second request.
		if !captured {
			captured = true
			gotMethod = r.Method
			gotPath = r.URL.Path

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			json.Unmarshal(body, &gotBody)

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"download": dl,
			})
			return
		}

		// Subsequent requests (WebSocket upgrade attempt) get rejected.
		w.WriteHeader(http.StatusBadRequest)
	})

	ctx := context.Background()
	err := c.Add(ctx, AddOptions{
		URL:      "http://example.com/file.bin",
		Dir:      "/tmp/downloads",
		Filename: "file.bin",
		Segments: 8,
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected method POST, got %s", gotMethod)
	}
	if gotPath != "/api/downloads" {
		t.Errorf("expected path /api/downloads, got %s", gotPath)
	}
	if gotBody.URL != "http://example.com/file.bin" {
		t.Errorf("expected body URL %q, got %q", "http://example.com/file.bin", gotBody.URL)
	}
	if gotBody.Segments != 8 {
		t.Errorf("expected body Segments 8, got %d", gotBody.Segments)
	}
}

func TestAdd_ServerError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "internal failure",
			"code":  "INTERNAL_ERROR",
		})
	})

	ctx := context.Background()
	err := c.Add(ctx, AddOptions{
		URL: "http://example.com/file.bin",
	})
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}

	if !strings.Contains(err.Error(), "internal failure") {
		t.Errorf("expected error to contain %q, got %q", "internal failure", err.Error())
	}
}

// ---------- List ----------

func TestList_Success(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	downloads := []model.Download{
		{
			ID:        "d_01TESTID00000000000000001",
			URL:       "http://example.com/a.bin",
			Filename:  "a.bin",
			Dir:       "/tmp",
			TotalSize: 2048,
			Status:    model.StatusCompleted,
			CreatedAt: now,
		},
		{
			ID:        "d_01TESTID00000000000000002",
			URL:       "http://example.com/b.bin",
			Filename:  "b.bin",
			Dir:       "/tmp",
			TotalSize: 4096,
			Status:    model.StatusActive,
			CreatedAt: now,
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/downloads" {
			t.Errorf("expected path /api/downloads, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"downloads": downloads,
			"total":     len(downloads),
		})
	})

	ctx := context.Background()
	err := c.List(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}

func TestList_WithStatusFilter(t *testing.T) {
	var gotQuery string

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("status")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"downloads": []model.Download{},
			"total":     0,
		})
	})

	ctx := context.Background()
	err := c.List(ctx, ListOptions{Status: "active"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if gotQuery != "active" {
		t.Errorf("expected status query param %q, got %q", "active", gotQuery)
	}
}

// ---------- Status ----------

func TestStatus_Success(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	dl := model.Download{
		ID:           "d_01TESTID00000000000000003",
		URL:          "http://example.com/c.bin",
		Filename:     "c.bin",
		Dir:          "/tmp",
		TotalSize:    8192,
		Downloaded:   4096,
		Status:       model.StatusActive,
		SegmentCount: 4,
		CreatedAt:    now,
	}
	segs := []model.Segment{
		{DownloadID: dl.ID, Index: 0, StartByte: 0, EndByte: 2047, Downloaded: 2048, Done: true},
		{DownloadID: dl.ID, Index: 1, StartByte: 2048, EndByte: 4095, Downloaded: 2048, Done: true},
		{DownloadID: dl.ID, Index: 2, StartByte: 4096, EndByte: 6143, Downloaded: 0, Done: false},
		{DownloadID: dl.ID, Index: 3, StartByte: 6144, EndByte: 8191, Downloaded: 0, Done: false},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/downloads/"+dl.ID {
			t.Errorf("expected path /api/downloads/%s, got %s", dl.ID, r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"download": dl,
			"segments": segs,
		})
	})

	ctx := context.Background()
	err := c.Status(ctx, dl.ID)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
}

func TestStatus_NotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "download not found",
			"code":  "NOT_FOUND",
		})
	})

	ctx := context.Background()
	err := c.Status(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain %q, got %q", "not found", err.Error())
	}
}

// ---------- Pause ----------

func TestPause_Success(t *testing.T) {
	const dlID = "d_01TESTID00000000000000004"
	var (
		gotMethod string
		gotPath   string
	)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
	})

	ctx := context.Background()
	err := c.Pause(ctx, dlID)
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected method POST, got %s", gotMethod)
	}
	if gotPath != "/api/downloads/"+dlID+"/pause" {
		t.Errorf("expected path %q, got %q", "/api/downloads/"+dlID+"/pause", gotPath)
	}
}

// ---------- Resume ----------

func TestResume_Success(t *testing.T) {
	const dlID = "d_01TESTID00000000000000005"
	var (
		gotMethod string
		gotPath   string
	)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
	})

	ctx := context.Background()
	// showProgress=false to avoid WebSocket connection attempt
	err := c.Resume(ctx, dlID, false)
	if err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected method POST, got %s", gotMethod)
	}
	if gotPath != "/api/downloads/"+dlID+"/resume" {
		t.Errorf("expected path %q, got %q", "/api/downloads/"+dlID+"/resume", gotPath)
	}
}

// ---------- Cancel ----------

func TestCancel_WithDeleteFile(t *testing.T) {
	const dlID = "d_01TESTID00000000000000006"
	var (
		gotMethod      string
		gotPath        string
		gotDeleteParam string
	)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotDeleteParam = r.URL.Query().Get("delete_file")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
	})

	ctx := context.Background()
	err := c.Cancel(ctx, dlID, true)
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Errorf("expected method DELETE, got %s", gotMethod)
	}
	if gotPath != "/api/downloads/"+dlID {
		t.Errorf("expected path %q, got %q", "/api/downloads/"+dlID, gotPath)
	}
	if gotDeleteParam != "true" {
		t.Errorf("expected delete_file=true, got %q", gotDeleteParam)
	}
}

// ---------- Refresh ----------

func TestRefresh_Success(t *testing.T) {
	const dlID = "d_01TESTID00000000000000007"
	const newURL = "http://mirror.example.com/file.bin"
	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]string
	)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		json.Unmarshal(body, &gotBody)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "refreshed"})
	})

	ctx := context.Background()
	err := c.Refresh(ctx, dlID, newURL)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected method POST, got %s", gotMethod)
	}
	if gotPath != "/api/downloads/"+dlID+"/refresh" {
		t.Errorf("expected path %q, got %q", "/api/downloads/"+dlID+"/refresh", gotPath)
	}
	if gotBody["url"] != newURL {
		t.Errorf("expected body url %q, got %q", newURL, gotBody["url"])
	}
}
