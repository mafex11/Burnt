# Burnt for Windows

The Windows port of Burnt: a system tray icon that shows what today's Claude Code and
Codex usage cost, plus a popover dashboard. Go 1.25, no cgo, cross-compiles to
`windows/amd64` from macOS or Linux.

Design spec: [`docs/superpowers/specs/2026-09-06-windows-port-design.md`](../docs/superpowers/specs/2026-09-06-windows-port-design.md).

## Layout

```
windows/
  cmd/burnt/main.go              entry point + headless flags; rsrc_windows_amd64.syso (icon, manifest)
  internal/app/                  the shell: tray, popover, JS bridge, poll loop, updates, toasts
  internal/engine/               PORTABLE: locates and runs ccusage, aggregates a Summary
  internal/trayicon/             PORTABLE: renders "$12.34" into a multi-size ICO; flame frames
  internal/settings/             PORTABLE: %APPDATA%\Burnt\settings.json
  internal/update/               PORTABLE check against the GitHub releases API + Windows apply
  ui/                           the dashboard (index.html, app.js, styles.css) + embed.go
  winres/                        winres.json + icons; the .syso is generated and committed
```

`internal/app` is the only Windows-only package. Files ending `_windows.go` (or tagged
`//go:build windows`) hold the Win32 work; the untagged files hold the decisions and are
unit-tested on any OS:

| File | What it decides |
|---|---|
| `state.go` | the bridge state JSON, update-status transitions, `JSCall`/`ShowPageJS` escaping, whether an update check is due |
| `tray_text.go` | what the icon says, the tooltip, the two disabled menu rows, when to animate, when to fetch projects |
| `place.go` | dips → pixels, height clamping, and the cursor → window placement maths |
| `assets.go` | the loopback server that feeds the embedded UI to WebView2 |
| `logfile.go` | log rotation |

This split is deliberate: with no Windows machine in the loop, anything worth testing has
to live somewhere `go test` can reach it on macOS.

## Build

```sh
cd windows
go test ./... -count=1                       # runs everywhere
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...

GOOS=windows GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -H windowsgui -X main.version=1.3.0" \
  -o dist/burnt.exe ./cmd/burnt
```

- `-H windowsgui` drops the console, so the app never flashes a black window. It also
  means **stdout is gone at runtime**: anything diagnostic has to go to a file or the log.
- `-X main.version=` is the only version source on Windows; it comes from the git tag.
- `ccusage.exe` (from `@ccusage/ccusage-win32-x64@<engine.PinnedCcusageVersion>`) must sit
  next to `burnt.exe`. There is no Node fallback.
- The `.syso` next to `main.go` carries the exe icon, the per-monitor-v2 DPI manifest and
  the version resource. Regenerate it from `windows/` with
  `go-winres make --in winres/winres.json --out cmd/burnt/rsrc`, then commit it; CI does
  not build it.

## Flags

The tray app runs when there are no arguments. The flags exist so the binary can be
exercised without a desktop session — they are what CI actually runs, and off Windows
they work while `app.Run` prints "Burnt's tray app runs on Windows only" and exits 1.

| Command | Behaviour |
|---|---|
| `burnt --version` | prints `burnt <version>`, exit 0 |
| `burnt --render-icon "$12" out.ico` | writes the tray ICO for that label, exit 0 |
| `burnt --diagnose out.txt` | writes a ccusage health report; exit 0 when ccusage was found and `daily --json` succeeded, else 1 |

`--diagnose` reports the version, GOOS/GOARCH, the exe path, `ccusage: found <path>` or
`ccusage: not found`, `ccusage --version`, and the row count from `daily --json`. **Zero
rows is a pass** — a fresh machine simply has no usage yet. A failing `--version` is
reported but does not fail the run; only a missing binary or a failed `daily` does.

```sh
go build -o /tmp/burnt ./cmd/burnt && /tmp/burnt --diagnose /tmp/diag.txt; cat /tmp/diag.txt
```

## CI

`.github/workflows/windows.yml`:

1. **test (ubuntu)** — `go vet ./...`, `go test ./... -count=1`, and a cross-build for
   `windows/amd64`.
2. **build + smoke test (windows-latest)** — builds with the tag's version, fetches the
   pinned `ccusage.exe` from npm, then runs `--version` (output must contain the version),
   `--render-icon` (the `.ico` must exist) and `--diagnose` (must exit 0 and contain
   `ccusage: found`). Packages `Burnt-windows-x64.zip` and, on a `v*` tag, uploads it.

That smoke test is the only place the Windows-only code executes, so keep the flags
working and keep failures loud.

## How the tray works

`systray.Run` takes the main goroutine — it pumps its own Win32 message loop and its
window has to stay on the thread that created it. Everything else hangs off it:

- **Poll loop** — refresh immediately, then every 60 s, plus on demand (menu Refresh,
  `burnt_refresh`, popover open). Requests are coalesced through a 1-deep channel, so a
  burst of clicks is one `ccusage` run. Per-project attribution (a second subprocess plus
  a walk of every session log) is only requested when the popover is up *and* the
  dashboard style is Detailed.
- **Icon** — Windows tray icons carry no text, so the figure is rendered into the ICO on
  every refresh (`trayicon.RenderText`), coloured from
  `HKCU\…\Themes\Personalize\SystemUsesLightTheme`, which is re-read each time so a
  light/dark switch is picked up. Icon-only mode draws the flame, and `animateFlame`
  cycles six frames at 6 fps — only in icon-only mode, and the ticker is stopped otherwise.
- **Missing ccusage** is not fatal: the engine is built with a nil runner, the tray shows
  `—` with the tooltip "ccusage.exe not found next to burnt.exe", and the popover shows the
  error status.
- **Single instance** — a `Local\BurntSingleInstance` mutex; a second launch exits 0 quietly.
- **Log** — `%APPDATA%\Burnt\burnt.log`, truncated at startup past 1 MB. Goroutines are
  started through `safego`, which turns a panic into a log line instead of a dead tray.

## How the popover works

The dashboard is a frameless 320-dip WebView2 window created lazily on first open.

- **Threading.** The window and every WebView2 call live on one dedicated OS thread:
  `popover.thread` calls `runtime.LockOSThread`, creates the window, then blocks in
  `webview.Run()`. Everything else goes through `w.Dispatch`. JS bindings are invoked on
  that same thread, so none of them block — anything slow (ccusage, GitHub, the installer)
  is handed to a goroutine that pushes fresh state when it finishes.
- **Window.** Created by `webview2.NewWithOptions`, then restyled: `WS_POPUP |
  WS_CLIPCHILDREN` with `WS_EX_TOOLWINDOW | WS_EX_TOPMOST` (no taskbar button, no
  Alt-Tab entry), `DWMWA_WINDOW_CORNER_PREFERENCE = DWMWCP_ROUND` (ignored on Windows 10)
  and `SetWindowPos` with `SWP_FRAMECHANGED`.
- **Assets.** WebView2 can't load an embedded FS directly, so `ui.FS` is served over
  `127.0.0.1` on a random port under a random path prefix, and the window navigates there.
  `mock.js` (the browser-only fixture bridge) is *not* embedded; the server answers it with
  an empty 200 so the console has no 404.
- **Placement.** On open: `GetCursorPos`, `SystemParametersInfo(SPI_GETWORKAREA)`,
  `GetDpiForWindow`, then the window is placed with its bottom-right corner just above the
  cursor and clamped inside the work area (`PlacePopover`). `burnt_resize(h)` re-places it
  at the same anchor with the new height, capped at 720 dips, and is a **no-op when the
  height is unchanged** — app.js reports the same height several times per refresh and
  calling `SetWindowPos` each time makes the window twitch.
- **Dismissal.** A 200 ms poll compares `GetAncestor(GetForegroundWindow(), GA_ROOT)` with
  our HWND (the WebView2 render widget is a child, so it must be resolved to its root) and
  hides on focus loss, with a short grace period after showing so activation doesn't race.
  The window and its WebView2 instance stay alive, so reopening is instant.
- **If WebView2 is missing** creation fails, which is logged, a toast explains it and
  "Open Burnt" opens the install instructions instead. The tray keeps working.

### Bridge

Bindings and the state shape are documented in [`ui/README.md`](ui/README.md); Go's side is
`internal/app/bridge_windows.go`. Two things worth knowing:

- state is pushed with `window.burntOnState(<JSON string literal>)`, JSON-encoded so a
  project or model name can't break out of the literal;
- `burnt_setSettings` saves and re-renders the tray, but deliberately does **not** echo
  state back unless `Validate()` had to correct something. The daily-budget field commits
  on every keystroke without re-rendering, and an echo would destroy the caret.

The tray's "Settings…" uses `window.burntShowPage('settings')` via `ShowPageJS`, which
retries for two seconds because the first open races the document load.

## Settings, notifications, updates

- `%APPDATA%\Burnt\settings.json`, written on every change. Launch at login is
  `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\Burnt` (a quoted exe path), not part
  of the settings file.
- Toasts go through `beeep`, with the flame written once to `%APPDATA%\Burnt\flame.png`.
  Which notifications are due is decided by `engine.EvaluateNotifications`, with the fired
  set persisted at `%APPDATA%\Burnt\notify-state.json` so a budget alert shows once a day.
  Toasts need a Start Menu shortcut to appear, which `install.ps1` creates.
- Update checks hit the GitHub releases API on launch (when `lastUpdateCheck` is over a day
  old) and every 24 h; `dev` and `0.0.0-dev*` builds never check. With `autoUpdate` on, the
  zip is downloaded to `%TEMP%` and `update.Apply` hands the swap to a detached script
  before Burnt quits; otherwise the popover offers an Install button.

## Known gaps

- WebView2's environment callback in `go-webview2` calls `log.Fatal` on failure, which
  would take the process with it. The likeliest cause (an unwritable user-data folder) is
  checked before the window is created; the rest is out of our hands.
- Alt+F4 destroys the popover window. That is detected (`webview.Run()` returns) and the
  next open recreates it.
- ARM64, code signing and MSI/winget are out of scope; see the design spec.
