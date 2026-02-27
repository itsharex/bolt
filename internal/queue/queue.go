package queue

import (
	"context"
	"sync"

	"github.com/fhsinchy/bolt/internal/db"
	"github.com/fhsinchy/bolt/internal/event"
	"github.com/fhsinchy/bolt/internal/model"
)

// StartFunc is a callback invoked when the queue decides to start a download.
type StartFunc func(ctx context.Context, id string) error

// Manager implements a FIFO queue with configurable max concurrent downloads.
type Manager struct {
	store         *db.Store
	bus           *event.Bus
	maxConcurrent int
	startFn       StartFunc

	mu          sync.Mutex
	activeCount int
	notify      chan struct{}
}

// New creates a new queue Manager.
func New(store *db.Store, bus *event.Bus, maxConcurrent int, startFn StartFunc) *Manager {
	return &Manager{
		store:         store,
		bus:           bus,
		maxConcurrent: maxConcurrent,
		startFn:       startFn,
		notify:        make(chan struct{}, 1),
	}
}

// Enqueue adds a download to the queue and signals evaluation.
func (m *Manager) Enqueue(id string) {
	m.signal()
}

// OnDownloadComplete decrements the active count and signals the queue
// to evaluate whether the next queued download can start.
func (m *Manager) OnDownloadComplete(id string) {
	m.mu.Lock()
	if m.activeCount > 0 {
		m.activeCount--
	}
	m.mu.Unlock()
	m.signal()
}

// Run is the main loop that evaluates the queue when signaled.
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.notify:
			m.evaluate(ctx)
		}
	}
}

func (m *Manager) evaluate(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for m.activeCount < m.maxConcurrent {
		dl, err := m.store.GetNextQueued(ctx)
		if err != nil || dl == nil {
			return
		}

		if err := m.startFn(ctx, dl.ID); err != nil {
			// If start fails, mark as error and try the next one
			_ = m.store.UpdateDownloadStatus(ctx, dl.ID, model.StatusError, err.Error())
			continue
		}

		m.activeCount++
	}
}

// SetMaxConcurrent updates the maximum number of concurrent downloads
// and re-evaluates the queue.
func (m *Manager) SetMaxConcurrent(max int) {
	m.mu.Lock()
	m.maxConcurrent = max
	m.mu.Unlock()
	m.signal()
}

// ActiveCount returns the current number of active downloads tracked by the queue.
func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeCount
}

func (m *Manager) signal() {
	select {
	case m.notify <- struct{}{}:
	default:
	}
}
