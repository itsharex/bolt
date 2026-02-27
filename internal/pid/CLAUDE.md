# internal/pid

PID file management for daemon process tracking.

## Functions

| Function | Description |
|---|---|
| `Path()` | Returns `~/.config/bolt/bolt.pid` |
| `Write()` | Writes current process PID to file |
| `Read()` | Reads PID from file, returns (int, error) |
| `IsRunning()` | Checks if PID file exists and process is alive (signal 0 probe) |
| `Remove()` | Deletes PID file |

## Usage

Used by the daemon startup sequence:
1. `IsRunning()` — check if another instance is running
2. `Write()` — write our PID on startup
3. `defer Remove()` — clean up on shutdown
