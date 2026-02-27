package cli

import (
	"fmt"
	"syscall"
	"time"

	"github.com/fhsinchy/bolt/internal/pid"
)

// Stop sends SIGTERM to the daemon and waits for it to shut down.
func (c *Client) Stop() error {
	p, err := pid.Read()
	if err != nil {
		return fmt.Errorf("no running daemon found (PID file missing)")
	}

	// Send SIGTERM.
	if err := syscall.Kill(p, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to pid %d: %w", p, err)
	}

	fmt.Printf("Sent SIGTERM to daemon (pid %d), waiting for shutdown...\n", p)

	// Poll /api/stats until it fails (meaning server is down).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)
		if err := c.CheckDaemon(); err != nil {
			fmt.Println("Daemon stopped.")
			return nil
		}
	}

	return fmt.Errorf("daemon did not stop within 10 seconds")
}
