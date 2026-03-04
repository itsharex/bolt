# Phase 9: Steam Deck Optimization — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Bolt a first-class Steam Deck application — touch-optimized Desktop Mode GUI, gamescope window handling, user-space SteamOS install, and a Decky Loader QAM plugin for Gaming Mode.

**Architecture:** Four independent components: (1) Go `gui.go` detects Steam Deck and maximizes the window; (2) Svelte frontend gets touch CSS + ↑/↓ queue reordering for coarse-pointer devices; (3) `make install-deck` enables the systemd service; (4) a new `extensions/decky/` Decky Loader plugin talks to Bolt's existing REST API + WebSocket.

**Tech Stack:** Go 1.23, Svelte 5, Tailwind CSS 4, Python 3.11 (Decky backend), React 17 + `@decky/ui` + `@decky/api` (Decky frontend), `@decky/cli` (Decky build tool), pnpm.

**Design doc:** `docs/plans/2026-03-03-steam-deck-phase9-design.md`

---

## Codebase Landmarks

| File | Role |
|------|------|
| `cmd/bolt/gui.go` | Wails window launch — add Deck detection + maximize |
| `frontend/src/app.css` | Global CSS — add touch-action + coarse-pointer rules |
| `frontend/src/lib/components/ActionButtons.svelte` | Per-download buttons — needs larger touch targets |
| `frontend/src/lib/components/DownloadList.svelte` | DnD reordering — needs touch ↑/↓ fallback |
| `frontend/src/lib/components/DownloadRow.svelte` | Row component — pass touch mode to child |
| `frontend/src/lib/components/AddDownloadDialog.svelte` | URL input — add inputmode="url" |
| `Makefile` | Add install-deck + uninstall-deck + build-decky |
| `extensions/decky/` | New directory — Decky plugin (all files) |

**Key insight:** `make install` already installs to user-space paths (`~/.local/bin`, `~/.config/systemd/user`, etc.) and `bolt.service` already uses `%h/.local/bin/bolt`. The only missing piece for SteamOS is enabling/starting the service automatically.

---

## Task 1: Steam Deck Detection in gui.go

**Files:**
- Modify: `cmd/bolt/gui.go`

### Step 1: Add isSteamDeck helper function

Add this function to `cmd/bolt/gui.go` before `launchGUI()`:

```go
// isSteamDeck reports whether the process is running on a Steam Deck.
// It checks /etc/os-release for VARIANT_ID=steamdeck, which Valve sets on SteamOS.
func isSteamDeck() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "VARIANT_ID=steamdeck" {
			return true
		}
	}
	return false
}
```

Add required imports to the import block in `gui.go`:
```go
"os"
"strings"
```

### Step 2: Use isSteamDeck in launchGUI

Locate the `wails.Run(&options.App{...})` call in `launchGUI()`. Change `Width: 960, Height: 640` to a variable, and add a post-startup maximize on Deck. Replace the current `onStartup` closure to also maximize on Deck.

The current `onStartup` is:
```go
onStartup := func(ctx context.Context) {
    application.OnStartup(ctx)
    tray.Start(tray.Callbacks{ ... })
}
```

Change it to:

```go
onDeck := isSteamDeck()

onStartup := func(ctx context.Context) {
    application.OnStartup(ctx)

    if onDeck {
        wailsRuntime.WindowMaximise(ctx)
    } else {
        tray.Start(tray.Callbacks{
            OnShow: func() {
                wailsRuntime.WindowShow(ctx)
            },
            OnHide: func() {
                wailsRuntime.WindowHide(ctx)
            },
            OnPauseAll: func() {
                _ = application.PauseAll()
            },
            OnResumeAll: func() {
                _ = application.ResumeAll()
            },
            OnSettings: func() {
                wailsRuntime.WindowShow(ctx)
                tray.SetVisible(true)
                wailsRuntime.EventsEmit(ctx, "open_settings")
            },
            OnQuit: func() {
                quitting = true
                tray.Quit()
                wailsRuntime.Quit(ctx)
            },
        })
    }
}
```

Also update `onShutdown` to skip `tray.Quit()` on Deck:
```go
onShutdown := func(ctx context.Context) {
    if !onDeck {
        tray.Quit()
    }
    application.OnShutdown(ctx)
}
```

And `OnBeforeClose` to always allow close on Deck (no minimize-to-tray):
```go
OnBeforeClose: func(ctx context.Context) (prevent bool) {
    if quitting || onDeck {
        return false
    }
    if d.cfg.MinimizeToTray {
        wailsRuntime.WindowHide(ctx)
        tray.SetVisible(false)
        return true
    }
    return false
},
```

### Step 3: Update window dimensions

In `wails.Run(&options.App{...})`, update:
```go
Width:     1280,
Height:    800,
MinWidth:  800,
MinHeight: 600,
```

(Was `Width: 960, Height: 640, MinWidth: 640, MinHeight: 480`)

### Step 4: Build to verify compilation

```bash
make build
```

Expected: binary builds without errors. The `isSteamDeck` function is not easily testable without a real Deck, so verify it compiles correctly.

### Step 5: Commit

```bash
git add cmd/bolt/gui.go
git commit -m "feat: detect Steam Deck, maximize window, skip tray on Deck"
```

---

## Task 2: Touch-Action Global CSS

**Files:**
- Modify: `frontend/src/app.css`

### Step 1: Add touch-action rule

Append to `frontend/src/app.css`:

```css
/* Eliminate 300ms tap delay on all interactive elements */
a,
button,
[role="button"],
input[type="checkbox"],
input[type="radio"],
input[type="range"],
select,
label {
  touch-action: manipulation;
}

/* On coarse-pointer devices (touch), enforce minimum 44px tap targets */
@media (pointer: coarse) {
  button,
  [role="button"],
  input[type="checkbox"],
  input[type="radio"],
  select {
    min-height: 44px;
    min-width: 44px;
  }
}
```

### Step 2: Build frontend to verify no CSS errors

```bash
cd frontend && pnpm build
```

Expected: build succeeds with no errors.

### Step 3: Commit

```bash
git add frontend/src/app.css
git commit -m "feat: add touch-action and 44px minimum tap target CSS for Steam Deck"
```

---

## Task 3: Larger Touch Targets in ActionButtons

**Files:**
- Modify: `frontend/src/lib/components/ActionButtons.svelte`

### Step 1: Understand the current button size

Current buttons use `class="p-1 rounded ..."` with `w-4 h-4` SVG icons. That's 4px+16px+4px = 24px — too small for touch. Need `p-3` (12px padding) = 40px, which is close enough. For exactly 44px, use `p-3.5` or `min-h-[44px] min-w-[44px]`.

### Step 2: Update all button classes in ActionButtons.svelte

Replace every button's class from `"p-1 rounded hover:..."` to `"p-2.5 rounded hover:... touch-action-manipulation"`. Since Tailwind doesn't have `touch-action-manipulation` utility in Tailwind 4, the global CSS from Task 2 handles that via the `button` selector.

The updated button template (apply to ALL buttons in the file):

```svelte
class="p-2.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-600 dark:text-gray-300 flex items-center justify-center"
```

For the cancel button:
```svelte
class="p-2.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-red-500 flex items-center justify-center"
```

Also increase SVG icon size from `w-4 h-4` to `w-5 h-5` for all icons — easier to see and tap on a 7" screen.

### Step 3: Build frontend

```bash
cd frontend && pnpm build
```

Expected: builds clean.

### Step 4: Commit

```bash
git add frontend/src/lib/components/ActionButtons.svelte
git commit -m "feat: increase ActionButtons tap targets for Steam Deck touch"
```

---

## Task 4: Touch-Compatible Queue Reordering

**Files:**
- Modify: `frontend/src/lib/components/DownloadList.svelte`
- Modify: `frontend/src/lib/components/DownloadRow.svelte`

### Step 1: Add touchMode detection to DownloadList.svelte

At the top of the `<script>` block in `DownloadList.svelte`, add:

```typescript
import { onMount } from "svelte";
import { getDownloads, reorderDownloads } from "../state/downloads.svelte";

// Detect coarse-pointer (touch) devices — set once on mount
let touchMode = $state(false);

onMount(() => {
  touchMode = window.matchMedia("(pointer: coarse)").matches;
});
```

Also add a move function for ↑/↓ buttons:

```typescript
async function moveDown(id: string) {
  const allDownloads = getDownloads();
  const ids = allDownloads.map((d) => d.id);
  const idx = ids.indexOf(id);
  if (idx === -1 || idx >= ids.length - 1) return;
  // Swap with next
  [ids[idx], ids[idx + 1]] = [ids[idx + 1], ids[idx]];
  await reorderDownloads(ids);
}

async function moveUp(id: string) {
  const allDownloads = getDownloads();
  const ids = allDownloads.map((d) => d.id);
  const idx = ids.indexOf(id);
  if (idx <= 0) return;
  // Swap with previous
  [ids[idx], ids[idx - 1]] = [ids[idx - 1], ids[idx]];
  await reorderDownloads(ids);
}
```

### Step 2: Update DownloadList template to pass touchMode and move callbacks

In the `{:else}` branch of the template, update the `DownloadRow` usage:

```svelte
{#each downloads as download, i (download.id)}
  <DownloadRow
    {download}
    isDragging={draggedId === download.id}
    isDropTarget={dropTargetId === download.id}
    {dropPosition}
    draggable={!isSearching && !touchMode}
    {touchMode}
    isFirst={i === 0}
    isLast={i === downloads.length - 1}
    onDragStart={(e) => handleDragStart(e, download.id)}
    onDragOver={(e) => handleDragOver(e, download.id)}
    onDragLeave={(e) => handleDragLeave(e, download.id)}
    onMoveUp={() => moveUp(download.id)}
    onMoveDown={() => moveDown(download.id)}
  />
{/each}
```

### Step 3: Update DownloadRow.svelte Props interface

Add new props to the `interface Props` block:

```typescript
interface Props {
  download: Download;
  isDragging?: boolean;
  isDropTarget?: boolean;
  dropPosition?: "above" | "below";
  draggable?: boolean;
  touchMode?: boolean;
  isFirst?: boolean;
  isLast?: boolean;
  onDragStart?: (e: DragEvent) => void;
  onDragOver?: (e: DragEvent) => void;
  onDragLeave?: (e: DragEvent) => void;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
}

let {
  download,
  isDragging = false,
  isDropTarget = false,
  dropPosition = "below",
  draggable = false,
  touchMode = false,
  isFirst = false,
  isLast = false,
  onDragStart,
  onDragOver,
  onDragLeave,
  onMoveUp,
  onMoveDown,
}: Props = $props();
```

### Step 4: Update DownloadRow drag handle section

Replace the current drag handle `{#if draggable}` block with a conditional that shows either the drag handle (mouse) or ↑/↓ buttons (touch):

```svelte
<!-- Reorder control: drag handle on mouse, ↑/↓ buttons on touch -->
{#if touchMode}
  <div class="flex flex-col gap-0.5 flex-shrink-0">
    <button
      onclick={onMoveUp}
      disabled={isFirst}
      class="p-1.5 rounded text-gray-400 dark:text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-30"
      title="Move up"
    >
      <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
        <polyline points="18 15 12 9 6 15" />
      </svg>
    </button>
    <button
      onclick={onMoveDown}
      disabled={isLast}
      class="p-1.5 rounded text-gray-400 dark:text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-30"
      title="Move down"
    >
      <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
        <polyline points="6 9 12 15 18 9" />
      </svg>
    </button>
  </div>
{:else if draggable}
  <span class="flex-shrink-0 text-gray-300 dark:text-gray-600 cursor-grab active:cursor-grabbing" title="Drag to reorder">
    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
      <circle cx="9" cy="5" r="1.5" />
      <circle cx="15" cy="5" r="1.5" />
      <circle cx="9" cy="12" r="1.5" />
      <circle cx="15" cy="12" r="1.5" />
      <circle cx="9" cy="19" r="1.5" />
      <circle cx="15" cy="19" r="1.5" />
    </svg>
  </span>
{/if}
```

### Step 5: Build frontend

```bash
cd frontend && pnpm build
```

Expected: builds clean, no TypeScript errors.

### Step 6: Commit

```bash
git add frontend/src/lib/components/DownloadList.svelte \
        frontend/src/lib/components/DownloadRow.svelte
git commit -m "feat: add touch-compatible queue reordering with up/down buttons"
```

---

## Task 5: Input Modes for Touch Keyboard

**Files:**
- Modify: `frontend/src/lib/components/AddDownloadDialog.svelte`

### Step 1: Update URL input

Find the URL input in `AddDownloadDialog.svelte` (around line 131):

```svelte
<input
  id="url-input"
  type="text"
  bind:value={url}
  ...
/>
```

Add `inputmode`, `autocomplete`, `autocorrect`, `autocapitalize` attributes:

```svelte
<input
  id="url-input"
  type="text"
  bind:value={url}
  onblur={probe}
  onkeydown={handleUrlKeydown}
  placeholder="https://example.com/file.zip"
  inputmode="url"
  autocomplete="off"
  autocorrect="off"
  autocapitalize="none"
  spellcheck="false"
  class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
  autofocus
/>
```

### Step 2: Update filename input (same file, around line 170)

```svelte
<input
  id="filename-input"
  type="text"
  bind:value={filename}
  placeholder="Auto-detected"
  autocomplete="off"
  autocorrect="off"
  autocapitalize="none"
  spellcheck="false"
  class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
/>
```

### Step 3: Build frontend

```bash
cd frontend && pnpm build
```

Expected: builds clean.

### Step 4: Commit

```bash
git add frontend/src/lib/components/AddDownloadDialog.svelte
git commit -m "feat: add inputmode and autocomplete hints for Steam Deck virtual keyboard"
```

---

## Task 6: `make install-deck` and `make uninstall-deck`

**Files:**
- Modify: `Makefile`

### Step 1: Examine current install target

The current `make install`:
- Copies binary to `~/.local/bin/`
- Copies `packaging/bolt.service` to `~/.config/systemd/user/`
- Copies `packaging/bolt.desktop` to `~/.local/share/applications/`
- Copies icon to `~/.local/share/icons/hicolor/256x256/apps/`
- Runs `systemctl --user daemon-reload`

It does NOT enable or start the service. On SteamOS, you want the daemon to auto-start on login.

### Step 2: Add install-deck and uninstall-deck targets

Add to `Makefile` after the `uninstall` target. Also update the `.PHONY` line:

Update `.PHONY` line:
```makefile
.PHONY: build build-gui build-extension build-extension-chrome build-extension-firefox dev test test-race test-v test-stress test-cover install install-deck uninstall uninstall-deck build-decky clean
```

Add targets after `uninstall`:
```makefile
# SteamOS / Steam Deck install — user-space, no root required.
# Installs to ~/.local/ paths, enables the systemd user service to
# auto-start on login. The existing make install already uses these
# paths; install-deck just also enables and starts the service.
install-deck: install
	systemctl --user enable --now bolt.service

uninstall-deck:
	-systemctl --user stop bolt.service
	-systemctl --user disable bolt.service
	rm -f ~/.local/bin/$(BINARY)
	rm -f ~/.config/systemd/user/bolt.service
	rm -f ~/.local/share/applications/bolt.desktop
	rm -f ~/.local/share/icons/hicolor/256x256/apps/bolt.png
	systemctl --user daemon-reload
```

### Step 3: Verify Makefile syntax

```bash
make -n install-deck 2>&1 | head -20
```

Expected: prints the commands that would be run without executing them. Should show `make install` steps followed by `systemctl --user enable --now bolt.service`.

### Step 4: Commit

```bash
git add Makefile
git commit -m "feat: add make install-deck target for SteamOS user-space install"
```

---

## Task 7: Decky Plugin Scaffold

**Files:**
- Create: `extensions/decky/plugin.json`
- Create: `extensions/decky/main.py`
- Create: `extensions/decky/package.json`
- Create: `extensions/decky/tsconfig.json`
- Create: `extensions/decky/src/index.tsx`
- Create: `extensions/decky/src/components/DownloadItem.tsx`
- Create: `extensions/decky/src/components/SpeedBadge.tsx`

### Step 1: Create plugin.json

```json
{
  "name": "Bolt",
  "author": "fhsinchy",
  "flags": [],
  "publish": {
    "tags": ["download", "tools"],
    "description": "Manage Bolt downloads from Gaming Mode"
  }
}
```

### Step 2: Create main.py

The Python backend runs inside Decky Loader and provides the auth token and port to the frontend (since the frontend can't read files directly). It also provides daemon start/stop.

```python
import asyncio
import json
import os
import subprocess


class Plugin:
    async def get_bolt_config(self) -> dict:
        """Read Bolt's config file and return auth_token and server_port."""
        config_path = os.path.expanduser("~/.config/bolt/config.json")
        try:
            with open(config_path) as f:
                cfg = json.load(f)
            return {
                "auth_token": cfg.get("auth_token", ""),
                "server_port": cfg.get("server_port", 9683),
            }
        except (OSError, json.JSONDecodeError):
            return {"auth_token": "", "server_port": 9683}

    async def start_daemon(self) -> bool:
        """Start the Bolt systemd user service."""
        try:
            result = subprocess.run(
                ["systemctl", "--user", "start", "bolt.service"],
                capture_output=True,
                timeout=5,
            )
            return result.returncode == 0
        except Exception:
            return False

    async def stop_daemon(self) -> bool:
        """Stop the Bolt systemd user service."""
        try:
            result = subprocess.run(
                ["systemctl", "--user", "stop", "bolt.service"],
                capture_output=True,
                timeout=5,
            )
            return result.returncode == 0
        except Exception:
            return False

    async def is_daemon_running(self) -> bool:
        """Check if the Bolt systemd service is active."""
        try:
            result = subprocess.run(
                ["systemctl", "--user", "is-active", "bolt.service"],
                capture_output=True,
                timeout=3,
            )
            return result.stdout.decode().strip() == "active"
        except Exception:
            return False

    # Required Decky lifecycle methods
    async def _main(self):
        pass

    async def _unload(self):
        pass
```

### Step 3: Create package.json

The Decky plugin frontend uses `@decky/cli` to build. The `@decky/ui` package provides Steam-themed components.

```json
{
  "name": "bolt-decky",
  "version": "0.1.0",
  "scripts": {
    "build": "decky-plugin build",
    "watch": "decky-plugin watch"
  },
  "devDependencies": {
    "@decky/cli": "^1.1.0",
    "@decky/api": "^1.0.2",
    "@decky/ui": "^4.2.2",
    "@types/react": "^17.0.0",
    "typescript": "^5.0.0"
  }
}
```

### Step 4: Create tsconfig.json

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true
  },
  "include": ["src"]
}
```

### Step 5: Commit scaffold

```bash
git add extensions/decky/
git commit -m "feat: add Decky Loader plugin scaffold for Steam Deck Gaming Mode"
```

---

## Task 8: Decky Plugin Frontend

**Files:**
- Create/Modify: `extensions/decky/src/index.tsx`
- Create/Modify: `extensions/decky/src/components/DownloadItem.tsx`
- Create/Modify: `extensions/decky/src/components/SpeedBadge.tsx`

### Step 1: Understand Decky UI components

Key Decky UI components (from `@decky/ui`):
- `PanelSection` — accordion section with title
- `PanelSectionRow` — a row inside a PanelSection
- `ButtonItem` — a tappable button row
- `ProgressBarItem` — row with a progress bar

The plugin root must export `default` as the main React component.

### Step 2: Create SpeedBadge.tsx

```tsx
import { FC } from "react";

interface Props {
  bytesPerSec: number;
}

function formatSpeed(bps: number): string {
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} MB/s`;
  if (bps >= 1_000) return `${(bps / 1_000).toFixed(0)} KB/s`;
  return `${bps} B/s`;
}

const SpeedBadge: FC<Props> = ({ bytesPerSec }) => {
  if (bytesPerSec <= 0) return null;
  return (
    <span style={{ fontSize: "12px", color: "#67c1f5" }}>
      ↓ {formatSpeed(bytesPerSec)}
    </span>
  );
};

export default SpeedBadge;
```

### Step 3: Create DownloadItem.tsx

```tsx
import { FC } from "react";
import { PanelSectionRow, ProgressBarItem, ButtonItem, Field } from "@decky/ui";

interface Download {
  id: string;
  filename: string;
  total_size: number;
  downloaded: number;
  speed: number;
  eta: number;
  status: "active" | "queued" | "paused" | "completed" | "error" | string;
}

interface Props {
  download: Download;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onCancel: (id: string) => void;
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}

function formatETA(seconds: number): string {
  if (seconds <= 0) return "";
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  return `${Math.round(seconds / 3600)}h`;
}

function formatBytes(bytes: number): string {
  if (bytes >= 1_000_000_000) return `${(bytes / 1_000_000_000).toFixed(1)} GB`;
  if (bytes >= 1_000_000) return `${(bytes / 1_000_000).toFixed(1)} MB`;
  if (bytes >= 1_000) return `${(bytes / 1_000).toFixed(0)} KB`;
  return `${bytes} B`;
}

const DownloadItem: FC<Props> = ({ download, onPause, onResume, onCancel }) => {
  const progress =
    download.total_size > 0
      ? download.downloaded / download.total_size
      : 0;

  const label = truncate(download.filename, 28);
  const eta = download.status === "active" && download.eta > 0
    ? ` — ${formatETA(download.eta)}`
    : "";
  const description = download.total_size > 0
    ? `${formatBytes(download.downloaded)} / ${formatBytes(download.total_size)}${eta}`
    : download.status;

  return (
    <>
      <ProgressBarItem
        label={label}
        description={description}
        progress={progress}
        indeterminate={download.total_size === 0 && download.status === "active"}
      />
      <PanelSectionRow>
        {download.status === "active" && (
          <ButtonItem layout="below" onClick={() => onPause(download.id)}>
            Pause
          </ButtonItem>
        )}
        {(download.status === "paused" || download.status === "queued") && (
          <ButtonItem layout="below" onClick={() => onResume(download.id)}>
            Resume
          </ButtonItem>
        )}
        <ButtonItem
          layout="below"
          onClick={() => onCancel(download.id)}
          style={{ color: "#e74c3c" }}
        >
          Cancel
        </ButtonItem>
      </PanelSectionRow>
    </>
  );
};

export default DownloadItem;
```

### Step 4: Create main index.tsx

```tsx
import { FC, useEffect, useState } from "react";
import {
  PanelSection,
  PanelSectionRow,
  ButtonItem,
  Field,
} from "@decky/ui";
import { callable } from "@decky/api";
import DownloadItem from "./components/DownloadItem";
import SpeedBadge from "./components/SpeedBadge";

// Decky backend callables
const getBoltConfig = callable<[], { auth_token: string; server_port: number }>(
  "get_bolt_config"
);
const startDaemon = callable<[], boolean>("start_daemon");
const stopDaemon = callable<[], boolean>("stop_daemon");
const isDaemonRunning = callable<[], boolean>("is_daemon_running");

interface Download {
  id: string;
  filename: string;
  total_size: number;
  downloaded: number;
  speed: number;
  eta: number;
  status: string;
}

const BoltPlugin: FC = () => {
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [running, setRunning] = useState(false);
  const [loading, setLoading] = useState(true);
  const [authToken, setAuthToken] = useState("");
  const [port, setPort] = useState(9683);
  const [totalSpeed, setTotalSpeed] = useState(0);

  // Build the API base URL
  const apiBase = `http://localhost:${port}/api`;

  async function fetchDownloads(token: string, apiPort: number) {
    try {
      const res = await fetch(`http://localhost:${apiPort}/api/downloads`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) return;
      const data: Download[] = await res.json();
      const active = data.filter((d) => ["active", "queued", "paused"].includes(d.status));
      setDownloads(active);
      const speed = active
        .filter((d) => d.status === "active")
        .reduce((sum, d) => sum + (d.speed || 0), 0);
      setTotalSpeed(speed);
      setRunning(true);
    } catch {
      setRunning(false);
      setDownloads([]);
      setTotalSpeed(0);
    }
  }

  async function handlePause(id: string) {
    await fetch(`${apiBase}/downloads/${id}/pause`, {
      method: "POST",
      headers: { Authorization: `Bearer ${authToken}` },
    });
    await fetchDownloads(authToken, port);
  }

  async function handleResume(id: string) {
    await fetch(`${apiBase}/downloads/${id}/resume`, {
      method: "POST",
      headers: { Authorization: `Bearer ${authToken}` },
    });
    await fetchDownloads(authToken, port);
  }

  async function handleCancel(id: string) {
    await fetch(`${apiBase}/downloads/${id}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${authToken}` },
    });
    await fetchDownloads(authToken, port);
  }

  async function handleStart() {
    await startDaemon();
    // Wait a moment for the service to come up, then re-check
    setTimeout(() => fetchDownloads(authToken, port), 1500);
  }

  useEffect(() => {
    let interval: ReturnType<typeof setInterval>;

    getBoltConfig().then((cfg) => {
      setAuthToken(cfg.auth_token);
      setPort(cfg.server_port);
      setLoading(false);

      // Initial fetch
      fetchDownloads(cfg.auth_token, cfg.server_port);

      // Poll every 2 seconds (simpler than WebSocket in a Decky plugin)
      interval = setInterval(
        () => fetchDownloads(cfg.auth_token, cfg.server_port),
        2000
      );
    });

    return () => {
      if (interval) clearInterval(interval);
    };
  }, []);

  if (loading) {
    return (
      <PanelSection title="Bolt">
        <PanelSectionRow>
          <Field label="Loading..." />
        </PanelSectionRow>
      </PanelSection>
    );
  }

  if (!running) {
    return (
      <PanelSection title="Bolt — Offline">
        <PanelSectionRow>
          <Field label="Bolt daemon is not running." />
        </PanelSectionRow>
        <PanelSectionRow>
          <ButtonItem layout="below" onClick={handleStart}>
            Start Bolt
          </ButtonItem>
        </PanelSectionRow>
      </PanelSection>
    );
  }

  return (
    <PanelSection
      title={
        <span>
          Bolt <SpeedBadge bytesPerSec={totalSpeed} />
        </span>
      }
    >
      {downloads.length === 0 ? (
        <PanelSectionRow>
          <Field label="No active downloads" />
        </PanelSectionRow>
      ) : (
        downloads.map((dl) => (
          <DownloadItem
            key={dl.id}
            download={dl}
            onPause={handlePause}
            onResume={handleResume}
            onCancel={handleCancel}
          />
        ))
      )}
    </PanelSection>
  );
};

export default BoltPlugin;
```

### Step 5: Commit frontend

```bash
git add extensions/decky/src/
git commit -m "feat: implement Decky QAM plugin frontend with live download list and controls"
```

---

## Task 9: Decky Build Target + Docs

**Files:**
- Modify: `Makefile`
- Modify: `README.md` (add Steam Deck section)

### Step 1: Add build-decky to Makefile

The `@decky/cli` tool is `decky-plugin` in the PATH when installed. Add to Makefile:

```makefile
build-decky:
	cd extensions/decky && pnpm install && pnpm build
```

Update `.PHONY` to include `build-decky` (already done in Task 6 step 2).

### Step 2: Verify build command structure

```bash
make -n build-decky
```

Expected output:
```
cd extensions/decky && pnpm install && pnpm build
```

### Step 3: Add README section

Add a new section to `README.md` after the Browser Extension section:

```markdown
## Steam Deck

Bolt runs in both Desktop Mode and Gaming Mode on the Steam Deck.

### Desktop Mode

Install Bolt for SteamOS (user-space, no root required):

```bash
make install-deck
```

This installs the binary to `~/.local/bin/`, the systemd user service to
`~/.config/systemd/user/`, and enables + starts the service automatically
on login. The Wails GUI opens maximized on the 7" display.

### Gaming Mode — Decky Plugin

The Bolt Decky Loader plugin shows active downloads in the Quick Access Menu
(QAM) without leaving Gaming Mode. Requires
[Decky Loader](https://decky.xyz) installed on your Steam Deck.

**Install the plugin:**

1. Build: `make build-decky` (produces `extensions/decky/dist/`)
2. Copy `extensions/decky/` to `~/homebrew/plugins/bolt/` on your Deck
3. Restart Decky Loader
4. Open the QAM (⋯ button) → Bolt icon

The plugin reads Bolt's config for the auth token automatically.
```

### Step 4: Commit

```bash
git add Makefile README.md
git commit -m "feat: add make build-decky target and Steam Deck README section"
```

---

## Verification Checklist

After all tasks are complete, verify each component manually:

### Desktop Mode (on the actual Deck or by inspecting code)

- [ ] `make build` succeeds
- [ ] GUI opens at 1280×800 window size on non-Deck; maximized on Deck (verify `isSteamDeck()` logic by temporarily returning `true`)
- [ ] ActionButtons buttons are larger — inspect element shows ≥ 44px height in dev tools
- [ ] URL input in AddDownloadDialog shows `inputmode="url"` in DOM

### Makefile

- [ ] `make -n install-deck` shows user-space paths with `systemctl --user enable --now`
- [ ] `make -n uninstall-deck` shows correct removal commands

### Decky Plugin (code review)

- [ ] `main.py` handles config read errors gracefully
- [ ] `index.tsx` handles Bolt offline state (shows Start button)
- [ ] `index.tsx` clears polling interval on unmount
- [ ] `DownloadItem.tsx` handles `total_size = 0` (indeterminate progress)

### Final commit (if all verification passes)

```bash
git log --oneline -10
```

Expected: 8-9 commits covering Tasks 1-9 in order.
