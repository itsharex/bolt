package pid

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/fhsinchy/bolt/internal/config"
)

// Path returns the full path to the PID file.
func Path() string {
	return filepath.Join(config.Dir(), "bolt.pid")
}

// Write writes the current process's PID to the PID file.
func Write() error {
	return os.WriteFile(Path(), []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// Read returns the PID stored in the PID file.
// Returns 0 and an error if the file does not exist or cannot be parsed.
func Read() (int, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// IsRunning returns true if the PID file exists and the process is alive.
func IsRunning() bool {
	pid, err := Read()
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if the process exists without sending a real signal.
	return proc.Signal(syscall.Signal(0)) == nil
}

// Remove deletes the PID file.
func Remove() {
	_ = os.Remove(Path())
}
