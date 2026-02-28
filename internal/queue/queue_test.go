package queue

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fhsinchy/bolt/internal/db"
	"github.com/fhsinchy/bolt/internal/event"
	"github.com/fhsinchy/bolt/internal/model"
)

func openTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func insertQueuedDownload(t *testing.T, store *db.Store, id string, order int) {
	t.Helper()
	ctx := context.Background()
	dl := &model.Download{
		ID:           id,
		URL:          "https://example.com/" + id,
		Filename:     id + ".bin",
		Dir:          t.TempDir(),
		TotalSize:    1024,
		Status:       model.StatusQueued,
		SegmentCount: 1,
		QueueOrder:   order,
	}
	if err := store.InsertDownload(ctx, dl); err != nil {
		t.Fatal(err)
	}
}

func TestQueue_MaxConcurrent(t *testing.T) {
	store := openTestStore(t)
	bus := event.NewBus()

	var mu sync.Mutex
	started := make([]string, 0)

	startFn := func(ctx context.Context, id string) error {
		mu.Lock()
		started = append(started, id)
		mu.Unlock()
		return nil
	}

	mgr := New(store, bus, 3, startFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mgr.Run(ctx)

	// Insert 5 queued downloads
	for i := 0; i < 5; i++ {
		id := model.NewDownloadID()
		insertQueuedDownload(t, store, id, i)
		mgr.Enqueue(id)
	}

	// Wait for evaluation
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(started)
	mu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 started, got %d", count)
	}

	if mgr.ActiveCount() != 3 {
		t.Errorf("active count = %d, want 3", mgr.ActiveCount())
	}
}

func TestQueue_CompleteTriggersNext(t *testing.T) {
	store := openTestStore(t)
	bus := event.NewBus()

	var mu sync.Mutex
	started := make([]string, 0)

	startFn := func(ctx context.Context, id string) error {
		mu.Lock()
		started = append(started, id)
		mu.Unlock()
		return nil
	}

	mgr := New(store, bus, 2, startFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mgr.Run(ctx)

	ids := make([]string, 4)
	for i := 0; i < 4; i++ {
		ids[i] = model.NewDownloadID()
		insertQueuedDownload(t, store, ids[i], i)
		mgr.Enqueue(ids[i])
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count1 := len(started)
	mu.Unlock()
	if count1 != 2 {
		t.Fatalf("expected 2 started initially, got %d", count1)
	}

	// Complete one download (set status so it's no longer queued)
	_ = store.UpdateDownloadStatus(ctx, ids[0], model.StatusCompleted, "")
	mgr.OnDownloadComplete(ids[0])

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count2 := len(started)
	mu.Unlock()
	if count2 != 3 {
		t.Errorf("expected 3 started after completion, got %d", count2)
	}
}

func TestQueue_EmptyQueue(t *testing.T) {
	store := openTestStore(t)
	bus := event.NewBus()

	called := false
	startFn := func(ctx context.Context, id string) error {
		called = true
		return nil
	}

	mgr := New(store, bus, 3, startFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mgr.Run(ctx)

	// Signal with empty queue
	mgr.signal()
	time.Sleep(100 * time.Millisecond)

	if called {
		t.Error("startFn should not have been called with empty queue")
	}
}

func TestMaxConcurrentChanged_MidFlight(t *testing.T) {
	store := openTestStore(t)
	bus := event.NewBus()

	var mu sync.Mutex
	started := make([]string, 0)

	startFn := func(ctx context.Context, id string) error {
		mu.Lock()
		started = append(started, id)
		mu.Unlock()
		return nil
	}

	mgr := New(store, bus, 2, startFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mgr.Run(ctx)

	// Insert 5 queued downloads and enqueue them.
	for i := 0; i < 5; i++ {
		id := model.NewDownloadID()
		insertQueuedDownload(t, store, id, i)
		mgr.Enqueue(id)
	}

	// Wait for initial evaluation with MaxConcurrent=2.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count1 := len(started)
	mu.Unlock()
	if count1 != 2 {
		t.Fatalf("expected 2 started initially, got %d", count1)
	}

	// Raise concurrency limit mid-flight.
	mgr.SetMaxConcurrent(5)

	// Wait for re-evaluation.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count2 := len(started)
	mu.Unlock()
	if count2 < 4 {
		t.Errorf("expected at least 4 started after SetMaxConcurrent(5), got %d", count2)
	}
}
