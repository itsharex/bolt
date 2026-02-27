package model

import (
	"fmt"
	"strings"
)

func FormatBytes(b int64) string {
	if b < 0 {
		return "unknown"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), suffixes[exp])
}

func FormatSpeed(bytesPerSec int64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	return FormatBytes(bytesPerSec) + "/s"
}

func FormatETA(seconds int) string {
	if seconds < 0 {
		return "unknown"
	}
	if seconds == 0 {
		return "0s"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// ParseRate parses a human-readable rate string like "10MB" or "1.5GB" into bytes.
func ParseRate(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	s = strings.ToUpper(s)
	s = strings.TrimSuffix(s, "/S")

	// Check longest suffixes first to avoid "B" matching before "MB"
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"TB", 1024 * 1024 * 1024 * 1024},
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, entry := range suffixes {
		if strings.HasSuffix(s, entry.suffix) {
			numStr := strings.TrimSpace(strings.TrimSuffix(s, entry.suffix))
			var val float64
			if _, err := fmt.Sscanf(numStr, "%f", &val); err != nil {
				return 0, fmt.Errorf("invalid rate: %q", s)
			}
			return int64(val * float64(entry.mult)), nil
		}
	}

	var val float64
	if _, err := fmt.Sscanf(s, "%f", &val); err != nil {
		return 0, fmt.Errorf("invalid rate: %q", s)
	}
	return int64(val), nil
}
