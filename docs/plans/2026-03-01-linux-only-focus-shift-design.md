# Linux-Only Focus Shift — Design Document

**Date:** March 1, 2026

---

## Rationale

Linux lacks a good, modern download manager. Windows has IDM, Free Download Manager, and many others. macOS has Folx and Gopeed. Linux has uGet (dated GTK2 UI), Persepolis (unmaintained aria2 frontend), and XDM (Java). Bolt can fill this gap by going deep on the platform rather than spreading thin across three.

Going Linux-only enables:
- Deep desktop integration (D-Bus notifications, XDG portals, systemd socket activation)
- Steam Deck / SteamOS optimization with a Decky Loader plugin
- Simplified codebase — no cross-platform abstractions for notifications, file opening, packaging
- Distribution through AUR, Flatpak, deb/rpm (future)
- Every future feature built Linux-native from the start

## Approach: Soft Shift

Remove Windows/macOS code paths and update all documentation. Keep the codebase structurally the same. Stdlib functions like `os.UserConfigDir()` that work correctly on Linux stay as-is.

## Code Changes

### `internal/notify/notify.go`

Remove the `runtime.GOOS` switch. Replace with a direct `notify-send` call:

```go
package notify

import "os/exec"

func Send(title, message string) error {
    return exec.Command("notify-send", title, message).Start()
}
```

### `internal/app/app.go` — `openPath()`

Remove the `runtime.GOOS` switch. Replace with a direct `xdg-open` call:

```go
func openPath(path string) error {
    return exec.Command("xdg-open", path).Start()
}
```

Both functions drop `"runtime"` and `"fmt"` imports.

## Roadmap Changes

### New Phase Structure

| Phase | Name | Status |
|-------|------|--------|
| 1 | Engine + CLI | COMPLETE |
| 2 | HTTP Server + Daemon | COMPLETE |
| 3 | Wails GUI + Frontend | COMPLETE |
| 4 | Browser Extension | COMPLETE |
| **5** | **Linux-Only Focus Shift** | **NEW** |
| 6 | P1 Features (remaining) | Renumbered from Phase 5 |
| 7 | P2 Features | Renumbered from Phase 6 |
| 8 | P3 Features | Renumbered from Phase 7 |
| **9** | **Steam Deck + Decky Plugin** | **NEW** |

### Phase 5 Deliverables (Linux-Only Focus Shift)

- Remove Windows/macOS code paths from `notify.go` and `app.go`
- Update bolt-prd.md: platform targets to Linux only
- Update bolt-trd.md: remove cross-platform build targets
- Update README.md: Linux-only messaging, remove macOS/Windows build instructions
- Update STATUS.md: reflect new phase numbering
- Update CLAUDE.md: reflect the shift

### Phase 9 Deliverables (Steam Deck + Decky Plugin)

- Decky Loader plugin (Python backend + React frontend) as a thin client to Bolt's REST API
- Bolt daemon running as systemd service on SteamOS
- QAM panel showing download progress, pause/resume controls
- Documentation for SteamOS / Steam Deck setup

## Document Updates

### bolt-prd.md
- Platform targets: Linux (x86_64, aarch64) only
- Remove Windows/macOS-specific mentions
- Add Linux-Only Rationale section
- Add Steam Deck phase to roadmap

### bolt-trd.md
- Remove `build-windows` and `build-darwin` targets
- Simplify notification and file-opening sections
- Remove Windows/macOS build instructions

### README.md
- Position as "the Linux download manager"
- Remove macOS and Windows build instructions
- Mention Steam Deck as a future goal

### STATUS.md
- Renumber phases
- Add Phase 5 (Linux-Only Shift) and Phase 9 (Steam Deck)

### CLAUDE.md
- Note Linux-only in Project Info
- Update Development Phases with new numbering
- Add Linux-Only Decision to Key Design Decisions

## What Stays Unchanged

- `os.UserConfigDir()` — works correctly on Linux
- `energye/systray` — functional, tested
- Wails framework — builds for Linux only, no replacement needed
- Browser extensions — browser-level, not OS-level
- Engine, server, database, queue, event bus — no platform-specific code
- Frontend — standard web APIs
- Makefile — already Linux-only in practice
- Config fields (proxy, clipboard_monitor, etc.) — unimplemented features, not platform-specific

## Future Linux-Specific Features (Not In This Shift)

These are deferred to their respective roadmap phases:
- D-Bus desktop notifications (richer than `notify-send`: icons, actions, progress)
- XDG Desktop Portal for file dialogs
- systemd socket activation
- StatusNotifierItem for system tray (if `energye/systray` proves insufficient)
- Clipboard monitoring via D-Bus
- Sound on completion via PulseAudio/PipeWire
