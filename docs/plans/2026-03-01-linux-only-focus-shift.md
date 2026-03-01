# Linux-Only Focus Shift Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove Windows/macOS code paths from the two files that contain them, making Bolt explicitly Linux-only.

**Architecture:** Two functions contain `runtime.GOOS` switches with Windows/macOS branches: `notify.Send()` and `app.openPath()`. Replace each with a direct Linux command call. No structural changes — just simplifying two functions.

**Tech Stack:** Go stdlib (`os/exec`)

---

### Task 1: Simplify `internal/notify/notify.go`

**Files:**
- Modify: `internal/notify/notify.go`

**Step 1: Read the current file and verify the cross-platform code**

Read `internal/notify/notify.go` and confirm it has a `runtime.GOOS` switch with `linux`, `darwin`, and `windows` cases.

**Step 2: Replace with Linux-only implementation**

Replace the entire file contents with:

```go
package notify

import "os/exec"

// Send displays a desktop notification via notify-send. Best-effort, non-blocking.
func Send(title, message string) error {
	return exec.Command("notify-send", title, message).Start()
}
```

**Step 3: Verify the build compiles**

Run: `go build ./internal/notify/`
Expected: Success, no errors.

**Step 4: Commit**

```bash
git add internal/notify/notify.go
git commit -m "Remove Windows/macOS notification code, keep notify-send only"
```

---

### Task 2: Simplify `openPath()` in `internal/app/app.go`

**Files:**
- Modify: `internal/app/app.go`

**Step 1: Read the current openPath function and confirm cross-platform code**

Read `internal/app/app.go` around lines 363-376 and confirm the `runtime.GOOS` switch with `linux`, `darwin`, `windows` cases.

**Step 2: Replace openPath with Linux-only implementation**

Replace the `openPath` function with:

```go
func openPath(path string) error {
	return exec.Command("xdg-open", path).Start()
}
```

**Step 3: Remove unused imports**

Check if `"runtime"` and `"fmt"` are still used elsewhere in the file. If `"runtime"` is no longer used anywhere in the file, remove it from the import block. `"fmt"` is likely still used by other functions — only remove if no other references exist.

**Step 4: Verify the build compiles**

Run: `go build ./internal/app/`
Expected: Success, no errors.

**Step 5: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests pass. (Tests don't exercise notify or openPath directly since they call external commands, but this confirms no import/compilation regressions.)

**Step 6: Commit**

```bash
git add internal/app/app.go
git commit -m "Remove Windows/macOS openPath code, keep xdg-open only"
```

---

### Task 3: Commit documentation updates

**Files:**
- Already modified: `bolt-prd.md`, `bolt-trd.md`, `README.md`, `STATUS.md`, `CLAUDE.md`
- Already created: `docs/plans/2026-03-01-linux-only-focus-shift-design.md`

**Step 1: Verify all doc changes look correct**

Run: `git diff` to review all pending documentation changes.

**Step 2: Commit all documentation updates**

```bash
git add bolt-prd.md bolt-trd.md README.md STATUS.md CLAUDE.md docs/plans/2026-03-01-linux-only-focus-shift-design.md docs/plans/2026-03-01-linux-only-focus-shift.md
git commit -m "Update all docs for Linux-only focus shift

- PRD: platform targets, build commands, categories, phases
- TRD: remove cross-platform build targets and paths
- README: Linux-only positioning, remove macOS/Windows build instructions
- STATUS: add Phase 5, renumber phases, add Steam Deck phase
- CLAUDE.md: Linux-only note, phase 5 + 9, design decision
- Design doc for the shift"
```

---

### Task 4: Verify everything works end-to-end

**Step 1: Run full test suite with race detector**

Run: `go test -race ./... -count=1`
Expected: All tests pass with no race conditions.

**Step 2: Build the full application**

Run: `make build`
Expected: Successful build producing `./bolt` binary.

**Step 3: Verify no cross-platform code remains**

Run: `grep -rn 'runtime.GOOS\|"darwin"\|"windows"\|osascript\|powershell\|cmd /c start' internal/`
Expected: No matches.
