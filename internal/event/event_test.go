package event

import (
	"sync"
	"testing"
	"time"
)

func TestSubscribeAndPublish(t *testing.T) {
	bus := NewBus()

	ch, id := bus.Subscribe()
	defer bus.Unsubscribe(id)

	want := DownloadAdded{DownloadID: "d_1", Filename: "file.zip", TotalSize: 1024}
	bus.Publish(want)

	select {
	case got := <-ch:
		da, ok := got.(DownloadAdded)
		if !ok {
			t.Fatalf("expected DownloadAdded, got %T", got)
		}
		if da != want {
			t.Fatalf("expected %+v, got %+v", want, da)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	bus := NewBus()

	const count = 5
	channels := make([]<-chan Event, count)
	ids := make([]int, count)

	for i := 0; i < count; i++ {
		channels[i], ids[i] = bus.Subscribe()
	}
	defer func() {
		for _, id := range ids {
			bus.Unsubscribe(id)
		}
	}()

	want := DownloadCompleted{DownloadID: "d_2", Filename: "archive.tar.gz"}
	bus.Publish(want)

	for i, ch := range channels {
		select {
		case got := <-ch:
			dc, ok := got.(DownloadCompleted)
			if !ok {
				t.Fatalf("subscriber %d: expected DownloadCompleted, got %T", i, got)
			}
			if dc != want {
				t.Fatalf("subscriber %d: expected %+v, got %+v", i, want, dc)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i)
		}
	}
}

func TestBackpressure(t *testing.T) {
	bus := NewBus()

	ch, id := bus.Subscribe()
	defer bus.Unsubscribe(id)

	// Fill the subscriber buffer completely.
	for i := 0; i < subscriberBufferSize; i++ {
		bus.Publish(Progress{DownloadID: "d_3", Downloaded: int64(i)})
	}

	// The buffer is now full. This publish must not block; the event is dropped.
	done := make(chan struct{})
	go func() {
		bus.Publish(Progress{DownloadID: "d_3", Downloaded: 999})
		close(done)
	}()

	select {
	case <-done:
		// good, did not block
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on full subscriber buffer")
	}

	// Drain and verify we have exactly subscriberBufferSize events.
	received := 0
	for range subscriberBufferSize {
		select {
		case <-ch:
			received++
		case <-time.After(time.Second):
			t.Fatalf("expected %d buffered events, got %d", subscriberBufferSize, received)
		}
	}

	// Channel should be empty now; the extra event was dropped.
	select {
	case ev := <-ch:
		t.Fatalf("unexpected extra event in buffer: %+v", ev)
	default:
		// expected
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewBus()

	ch, id := bus.Subscribe()
	bus.Unsubscribe(id)

	// After Unsubscribe the channel must be closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed, but received a value")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}

	// Unsubscribing the same ID again must not panic.
	bus.Unsubscribe(id)
}

func TestConcurrentPublish(t *testing.T) {
	bus := NewBus()

	ch, id := bus.Subscribe()
	defer bus.Unsubscribe(id)

	const goroutines = 100
	const eventsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				bus.Publish(Progress{DownloadID: "d_race", Downloaded: int64(i)})
			}
		}()
	}

	wg.Wait()

	// Drain whatever was received. Because the buffer is 256 and we publish
	// 5000 events concurrently, some will be dropped. We just verify we got
	// at least 1 and at most subscriberBufferSize events, and nothing panicked.
	received := 0
drain:
	for {
		select {
		case <-ch:
			received++
		default:
			break drain
		}
	}

	if received == 0 {
		t.Fatal("expected at least one event from concurrent publishers")
	}
	if received > subscriberBufferSize {
		t.Fatalf("received %d events, which exceeds buffer size %d", received, subscriberBufferSize)
	}
}
