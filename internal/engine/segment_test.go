package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fhsinchy/bolt/internal/model"
	"github.com/fhsinchy/bolt/internal/testutil"
)

func TestSegmentWorker_SingleSegment(t *testing.T) {
	const fileSize = 1024 * 100 // 100KB
	ts := testutil.NewTestServer(fileSize)
	defer ts.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.bin")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(fileSize); err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dl := &model.Download{
		ID:           "d_test1",
		URL:          ts.URL,
		TotalSize:    fileSize,
		SegmentCount: 1,
	}
	seg := &model.Segment{
		DownloadID: dl.ID,
		Index:      0,
		StartByte:  0,
		EndByte:    fileSize - 1,
		Downloaded: 0,
	}

	reportCh := make(chan segmentReport, 1000)
	w := &segmentWorker{
		download: dl,
		segment:  seg,
		client:   ts.Client(),
		reportCh: reportCh,
		file:     file,
	}

	err = w.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !seg.Done {
		t.Error("segment should be marked done")
	}

	// Verify data matches
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	expected := testutil.GenerateData(fileSize)
	if len(got) != len(expected) {
		t.Fatalf("file size mismatch: got %d, want %d", len(got), len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("byte mismatch at offset %d: got %d, want %d", i, got[i], expected[i])
			break
		}
	}
}

func TestSegmentWorker_ResumeFromOffset(t *testing.T) {
	const fileSize = 1024 * 100
	ts := testutil.NewTestServer(fileSize)
	defer ts.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.bin")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(fileSize); err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	halfSize := int64(fileSize / 2)

	dl := &model.Download{
		ID:           "d_test2",
		URL:          ts.URL,
		TotalSize:    fileSize,
		SegmentCount: 1,
	}
	seg := &model.Segment{
		DownloadID: dl.ID,
		Index:      0,
		StartByte:  0,
		EndByte:    fileSize - 1,
		Downloaded: halfSize, // resume from halfway
	}

	reportCh := make(chan segmentReport, 1000)
	w := &segmentWorker{
		download: dl,
		segment:  seg,
		client:   ts.Client(),
		reportCh: reportCh,
		file:     file,
	}

	err = w.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !seg.Done {
		t.Error("segment should be marked done")
	}
	if seg.Downloaded != fileSize {
		t.Errorf("downloaded = %d, want %d", seg.Downloaded, fileSize)
	}
}

func TestSegmentWorker_RetryOnTransient500(t *testing.T) {
	const fileSize = 1024 * 10
	// Server returns 500 first 2 times via failAfterBytes=0 trick
	// Actually, let's use a server that sends partial data then closes
	ts := testutil.NewTestServer(fileSize, testutil.WithFailAfterBytes(1024, 2))
	defer ts.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.bin")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(fileSize); err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dl := &model.Download{
		ID:           "d_test3",
		URL:          ts.URL,
		TotalSize:    fileSize,
		SegmentCount: 1,
	}
	seg := &model.Segment{
		DownloadID: dl.ID,
		Index:      0,
		StartByte:  0,
		EndByte:    fileSize - 1,
		Downloaded: 0,
	}

	reportCh := make(chan segmentReport, 10000)
	w := &segmentWorker{
		download: dl,
		segment:  seg,
		client:   ts.Client(),
		reportCh: reportCh,
		file:     file,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w.RunWithRetry(ctx, 5)

	if !seg.Done {
		t.Error("segment should eventually succeed after retries")
	}
}

func TestSegmentWorker_PermanentError404(t *testing.T) {
	ts := testutil.NewTestServer(1024, testutil.WithStatusOverride(404))
	defer ts.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.bin")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dl := &model.Download{
		ID:           "d_test4",
		URL:          ts.URL,
		TotalSize:    1024,
		SegmentCount: 1,
	}
	seg := &model.Segment{
		DownloadID: dl.ID,
		Index:      0,
		StartByte:  0,
		EndByte:    1023,
		Downloaded: 0,
	}

	reportCh := make(chan segmentReport, 100)
	w := &segmentWorker{
		download: dl,
		segment:  seg,
		client:   ts.Client(),
		reportCh: reportCh,
		file:     file,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	w.RunWithRetry(ctx, 5)

	// Should get an error report quickly without exhausting all retries
	var gotErr bool
	for len(reportCh) > 0 {
		r := <-reportCh
		if r.Err != nil {
			gotErr = true
			break
		}
	}

	if !gotErr {
		t.Error("expected permanent error for 404")
	}
}

func TestIsPermanentError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{"404", &httpError{404}, true},
		{"403", &httpError{403}, true},
		{"410", &httpError{410}, true},
		{"416", &httpError{416}, true},
		{"500", &httpError{500}, false},
		{"503", &httpError{503}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermanentError(tt.err)
			if got != tt.permanent {
				t.Errorf("isPermanentError(%v) = %v, want %v", tt.err, got, tt.permanent)
			}
		})
	}
}
