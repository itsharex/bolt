package notify

import "os/exec"

// Send displays a desktop notification via notify-send. Best-effort, non-blocking.
func Send(title, message string) error {
	return exec.Command("notify-send", title, message).Start()
}
