package pid

import (
	"os"
	"testing"
)

func TestWriteRead(t *testing.T) {
	// Use a temp dir so we don't clobber real config.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	pid, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("got pid %d, want %d", pid, os.Getpid())
	}
}

func TestIsRunning_CurrentProcess(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !IsRunning() {
		t.Fatal("IsRunning should return true for current process")
	}
}

func TestIsRunning_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if IsRunning() {
		t.Fatal("IsRunning should return false when no PID file exists")
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	Remove()

	_, err := Read()
	if err == nil {
		t.Fatal("Read should fail after Remove")
	}
}
