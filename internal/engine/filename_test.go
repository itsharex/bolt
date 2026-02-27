package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectFilename_UserProvided(t *testing.T) {
	got := DetectFilename("custom.zip", `attachment; filename="server.zip"`, "https://example.com/url.zip")
	if got != "custom.zip" {
		t.Errorf("got %q, want %q", got, "custom.zip")
	}
}

func TestDetectFilename_ContentDisposition(t *testing.T) {
	got := DetectFilename("", `attachment; filename="report.pdf"`, "https://example.com/dl?id=42")
	if got != "report.pdf" {
		t.Errorf("got %q, want %q", got, "report.pdf")
	}
}

func TestDetectFilename_URL(t *testing.T) {
	got := DetectFilename("", "", "https://example.com/files/photo%20album.jpg")
	if got != "photo album.jpg" {
		t.Errorf("got %q, want %q", got, "photo album.jpg")
	}
}

func TestDetectFilename_Fallback(t *testing.T) {
	got := DetectFilename("", "", "")
	if !strings.HasPrefix(got, "download_") {
		t.Errorf("got %q, want prefix %q", got, "download_")
	}
	// "download_" (9 chars) + 10 ULID chars = 19
	if len(got) != 19 {
		t.Errorf("len = %d, want 19", len(got))
	}
}

func TestDetectFilename_Priority(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		cd      string
		url     string
		wantPfx string
	}{
		{
			name: "user wins over all",
			user: "my.file",
			cd:   `attachment; filename="cd.file"`,
			url:  "https://example.com/url.file",
			wantPfx: "my.file",
		},
		{
			name: "cd wins over url",
			user: "",
			cd:   `attachment; filename="cd.file"`,
			url:  "https://example.com/url.file",
			wantPfx: "cd.file",
		},
		{
			name: "url wins over fallback",
			user: "",
			cd:   "",
			url:  "https://example.com/url.file",
			wantPfx: "url.file",
		},
		{
			name: "fallback",
			user: "",
			cd:   "",
			url:  "",
			wantPfx: "download_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFilename(tt.user, tt.cd, tt.url)
			if !strings.HasPrefix(got, tt.wantPfx) {
				t.Errorf("got %q, want prefix %q", got, tt.wantPfx)
			}
		})
	}
}

func TestDetectFilename_ContentDispositionRFC5987(t *testing.T) {
	// mime.ParseMediaType handles RFC 5987 filename* by decoding and
	// placing the result in params["filename"].
	cd := `attachment; filename*=UTF-8''%E4%B8%AD%E6%96%87%E6%96%87%E4%BB%B6.txt`
	got := DetectFilename("", cd, "")
	want := "\u4e2d\u6587\u6587\u4ef6.txt" // Chinese characters
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDetectFilename_URLWithQuery(t *testing.T) {
	got := DetectFilename("", "", "https://example.com/files/data.csv?token=abc&v=2")
	if got != "data.csv" {
		t.Errorf("got %q, want %q", got, "data.csv")
	}
}

func TestDeduplicateFilename(t *testing.T) {
	dir := t.TempDir()

	// No collision -- returns original name.
	got := DeduplicateFilename(dir, "unique.txt")
	if got != "unique.txt" {
		t.Errorf("got %q, want %q", got, "unique.txt")
	}

	// Create the file so next call collides.
	if err := os.WriteFile(filepath.Join(dir, "unique.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = DeduplicateFilename(dir, "unique.txt")
	if got != "unique(1).txt" {
		t.Errorf("got %q, want %q", got, "unique(1).txt")
	}

	// Create unique(1).txt too.
	if err := os.WriteFile(filepath.Join(dir, "unique(1).txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = DeduplicateFilename(dir, "unique.txt")
	if got != "unique(2).txt" {
		t.Errorf("got %q, want %q", got, "unique(2).txt")
	}
}

func TestDeduplicateFilename_DoubleExtension(t *testing.T) {
	dir := t.TempDir()

	// Create archive.tar.gz so next call must deduplicate.
	if err := os.WriteFile(filepath.Join(dir, "archive.tar.gz"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DeduplicateFilename(dir, "archive.tar.gz")
	if got != "archive(1).tar.gz" {
		t.Errorf("got %q, want %q", got, "archive(1).tar.gz")
	}

	// Create archive(1).tar.gz too.
	if err := os.WriteFile(filepath.Join(dir, "archive(1).tar.gz"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = DeduplicateFilename(dir, "archive.tar.gz")
	if got != "archive(2).tar.gz" {
		t.Errorf("got %q, want %q", got, "archive(2).tar.gz")
	}
}

func TestDeduplicateFilename_NoExtension(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DeduplicateFilename(dir, "README")
	if got != "README(1)" {
		t.Errorf("got %q, want %q", got, "README(1)")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal filename",
			input: "file.txt",
			want:  "file.txt",
		},
		{
			name:  "path separators replaced",
			input: "path/to\\file.txt",
			want:  "path_to_file.txt",
		},
		{
			name:  "null bytes removed",
			input: "file\x00name.txt",
			want:  "filename.txt",
		},
		{
			name:  "leading dots stripped",
			input: "...hidden",
			want:  "hidden",
		},
		{
			name:  "whitespace trimmed",
			input: "  file.txt  ",
			want:  "file.txt",
		},
		{
			name:  "empty after sanitization",
			input: "...",
			want:  "download",
		},
		{
			name:  "long filename capped",
			input: strings.Repeat("a", 300),
			want:  strings.Repeat("a", 255),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
