package db

import (
	"context"
	"testing"

	"github.com/fhsinchy/bolt/internal/model"
)

// insertTestDownloadForSegments inserts a minimal download so foreign keys are satisfied.
func insertTestDownloadForSegments(t *testing.T, store *Store, id string) {
	t.Helper()
	d := newTestDownload(id)
	if err := store.InsertDownload(context.Background(), d); err != nil {
		t.Fatalf("insert parent download: %v", err)
	}
}

func TestInsertAndGetSegments(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	insertTestDownloadForSegments(t, store, "d_seg001")

	segments := []model.Segment{
		{DownloadID: "d_seg001", Index: 0, StartByte: 0, EndByte: 255, Downloaded: 0, Done: false},
		{DownloadID: "d_seg001", Index: 1, StartByte: 256, EndByte: 511, Downloaded: 100, Done: false},
		{DownloadID: "d_seg001", Index: 2, StartByte: 512, EndByte: 767, Downloaded: 256, Done: true},
	}

	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("insert segments: %v", err)
	}

	got, err := store.GetSegments(ctx, "d_seg001")
	if err != nil {
		t.Fatalf("get segments: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d segments, want 3", len(got))
	}

	// Verify ordering by index.
	for i, seg := range got {
		if seg.Index != i {
			t.Errorf("segment[%d].Index = %d, want %d", i, seg.Index, i)
		}
		if seg.DownloadID != "d_seg001" {
			t.Errorf("segment[%d].DownloadID = %q, want d_seg001", i, seg.DownloadID)
		}
	}

	// Verify specific values.
	if got[0].StartByte != 0 || got[0].EndByte != 255 {
		t.Errorf("segment[0] range = [%d, %d], want [0, 255]", got[0].StartByte, got[0].EndByte)
	}
	if got[1].Downloaded != 100 {
		t.Errorf("segment[1].Downloaded = %d, want 100", got[1].Downloaded)
	}
	if !got[2].Done {
		t.Error("segment[2].Done = false, want true")
	}
	if got[0].Done {
		t.Error("segment[0].Done = true, want false")
	}
}

func TestInsertSegments_Empty(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Inserting empty slice should be a no-op.
	if err := store.InsertSegments(ctx, nil); err != nil {
		t.Fatalf("insert empty segments: %v", err)
	}
	if err := store.InsertSegments(ctx, []model.Segment{}); err != nil {
		t.Fatalf("insert empty slice: %v", err)
	}
}

func TestGetSegments_Empty(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	got, err := store.GetSegments(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("get segments: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d segments, want 0", len(got))
	}
}

func TestBatchUpdateSegments(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	insertTestDownloadForSegments(t, store, "d_bup001")

	segments := []model.Segment{
		{DownloadID: "d_bup001", Index: 0, StartByte: 0, EndByte: 511, Downloaded: 0, Done: false},
		{DownloadID: "d_bup001", Index: 1, StartByte: 512, EndByte: 1023, Downloaded: 0, Done: false},
	}
	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("insert segments: %v", err)
	}

	// Update progress.
	updates := []model.Segment{
		{DownloadID: "d_bup001", Index: 0, Downloaded: 512, Done: true},
		{DownloadID: "d_bup001", Index: 1, Downloaded: 256, Done: false},
	}
	if err := store.BatchUpdateSegments(ctx, updates); err != nil {
		t.Fatalf("batch update: %v", err)
	}

	got, err := store.GetSegments(ctx, "d_bup001")
	if err != nil {
		t.Fatalf("get segments: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d segments, want 2", len(got))
	}
	if got[0].Downloaded != 512 {
		t.Errorf("segment[0].Downloaded = %d, want 512", got[0].Downloaded)
	}
	if !got[0].Done {
		t.Error("segment[0].Done = false, want true")
	}
	if got[1].Downloaded != 256 {
		t.Errorf("segment[1].Downloaded = %d, want 256", got[1].Downloaded)
	}
	if got[1].Done {
		t.Error("segment[1].Done = true, want false")
	}
}

func TestBatchUpdateSegments_Empty(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.BatchUpdateSegments(ctx, nil); err != nil {
		t.Fatalf("batch update empty: %v", err)
	}
}

func TestDeleteSegments(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	insertTestDownloadForSegments(t, store, "d_dseg001")

	segments := []model.Segment{
		{DownloadID: "d_dseg001", Index: 0, StartByte: 0, EndByte: 511},
		{DownloadID: "d_dseg001", Index: 1, StartByte: 512, EndByte: 1023},
	}
	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("insert segments: %v", err)
	}

	if err := store.DeleteSegments(ctx, "d_dseg001"); err != nil {
		t.Fatalf("delete segments: %v", err)
	}

	got, err := store.GetSegments(ctx, "d_dseg001")
	if err != nil {
		t.Fatalf("get segments after delete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d segments after delete, want 0", len(got))
	}
}

func TestDeleteSegments_Nonexistent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Deleting segments for a non-existent download should not error.
	if err := store.DeleteSegments(ctx, "nonexistent"); err != nil {
		t.Fatalf("delete nonexistent: %v", err)
	}
}

func TestSegments_PreservedAfterBatchUpdate(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	insertTestDownloadForSegments(t, store, "d_pres001")

	segments := []model.Segment{
		{DownloadID: "d_pres001", Index: 0, StartByte: 0, EndByte: 255, Downloaded: 0, Done: false},
		{DownloadID: "d_pres001", Index: 1, StartByte: 256, EndByte: 511, Downloaded: 0, Done: false},
		{DownloadID: "d_pres001", Index: 2, StartByte: 512, EndByte: 767, Downloaded: 0, Done: false},
	}
	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Only update segment 1.
	updates := []model.Segment{
		{DownloadID: "d_pres001", Index: 1, Downloaded: 128, Done: false},
	}
	if err := store.BatchUpdateSegments(ctx, updates); err != nil {
		t.Fatalf("batch update: %v", err)
	}

	got, err := store.GetSegments(ctx, "d_pres001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("segments = %d, want 3", len(got))
	}

	// Segment 0 and 2 should be unchanged.
	if got[0].Downloaded != 0 {
		t.Errorf("segment[0].Downloaded = %d, want 0", got[0].Downloaded)
	}
	if got[1].Downloaded != 128 {
		t.Errorf("segment[1].Downloaded = %d, want 128", got[1].Downloaded)
	}
	if got[2].Downloaded != 0 {
		t.Errorf("segment[2].Downloaded = %d, want 0", got[2].Downloaded)
	}
}
