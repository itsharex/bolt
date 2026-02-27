package model

import (
	"sort"
	"strings"
	"testing"
)

func TestNewDownloadID(t *testing.T) {
	id := NewDownloadID()
	if !strings.HasPrefix(id, "d_") {
		t.Errorf("expected prefix d_, got %s", id)
	}
	// d_ + 26 char ULID = 28 total
	if len(id) != 28 {
		t.Errorf("expected length 28, got %d (%s)", len(id), id)
	}
}

func TestNewDownloadID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewDownloadID()
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestNewDownloadID_Sortable(t *testing.T) {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = NewDownloadID()
	}
	if !sort.StringsAreSorted(ids) {
		t.Error("IDs are not lexicographically sorted")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{-1, "unknown"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.input)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B/s"},
		{-1, "0 B/s"},
		{1024, "1.0 KB/s"},
		{10485760, "10.0 MB/s"},
	}
	for _, tt := range tests {
		got := FormatSpeed(tt.input)
		if got != tt.want {
			t.Errorf("FormatSpeed(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatETA(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{-1, "unknown"},
		{0, "0s"},
		{5, "5s"},
		{65, "1m5s"},
		{3661, "1h1m1s"},
	}
	for _, tt := range tests {
		got := FormatETA(tt.input)
		if got != tt.want {
			t.Errorf("FormatETA(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseRate(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"", 0},
		{"0", 0},
		{"1024", 1024},
		{"10KB", 10240},
		{"10kb", 10240},
		{"1MB", 1048576},
		{"1.5MB", 1572864},
		{"1GB", 1073741824},
		{"10MB/s", 10485760},
	}
	for _, tt := range tests {
		got, err := ParseRate(tt.input)
		if err != nil {
			t.Errorf("ParseRate(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseRate(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseRate_Invalid(t *testing.T) {
	_, err := ParseRate("abc")
	if err == nil {
		t.Error("expected error for invalid rate")
	}
}
