# Burnt for Windows — design

Date: 2026-09-06. Status: approved for implementation (user requested full Windows parity in one go).

## Goal

Ship Burnt on Windows 10/11 (x64) with the same features as the macOS app: today's spend
as live text in the taskbar tray, a popover dashboard, settings, notifications, launch at
login, auto-update, a one-command install, and a website + README that offer both platforms.

## Key facts discovered

- ccusage 20.0.6's `dist/cli.js` is only a launcher that spawns a **native standalone
  binary** from the optional package `@ccusage/ccusage-win32-x64` (`bin/ccusage.exe`, 3 MB).
  Windows therefore needs **no bundled Node**. The whole install is ~12 MB.
- Windows tray icons are 16x16 (scaled per DPI) and cannot show a text label. The dollar
  figure is rendered **into the icon** as text, redrawn on every refresh, with colour
  following the taskbar theme.
- No CI exists today; releases are manual (`packaging/make-release.sh` + `gh release`).
- The site is a static Next.js export; install commands are hard-coded `brew install`.

## Stack

Go 1.25 (already installed; cross-compiles to windows/amd64 from macOS without cgo).

| Concern | Library |
|---|---|
| Tray icon + context menu | `fyne.io/systray` (pure syscall on Windows) |
| Popover dashboard window | `github.com/jchv/go-webview2` (WebView2, no cgo). Frameless `WS_POPUP` window, positioned at the cursor, hidden on focus loss |
| Icon rendering | `golang.org/x/image` (Go fonts, ICO writer in-house) |
| Registry (theme, Run key) | `golang.org/x/sys/windows/registry` |
| Toasts | `github.com/gen2brain/beeep` |
| Exe icon + manifest | `github.com/tc-hib/go-winres` (generates `.syso`, committed) |

## Layout (all inside the existing repo)

```
windows/
  go.mod                      module github.com/mafex11/Burnt/windows
  cmd/burnt/main.go           flags: --diagnose <out>, --render-icon <text> <out.ico>, --version; else run tray app
  internal/engine/            PORTABLE. Port of Sources/UsageEngine + Formatters + WrappedData. Tested on macOS.
  internal/engine/testdata/   copies of Tests/UsageEngineTests/Fixtures/*.json
  internal/trayicon/          PORTABLE. text -> multi-size ICO; flame frames. Tested.
  internal/settings/          PORTABLE. JSON settings store. Tested.
  internal/update/            PORTABLE check (GitHub releases API) + windows-only apply.
  internal/app/               WINDOWS-ONLY (build tag). Tray, popover, JS bridge, poll loop, notifications, launch-at-login.
  ui/                         Embedded dashboard: index.html, app.js, styles.css, mock.js (browser dev mode).
  winres/                     winres.json + icon PNG; generated rsrc_windows_amd64.syso committed.
install.ps1                   one-shot installer (repo root; served via raw.githubusercontent.com)
.github/workflows/windows.yml test on ubuntu, build+smoke on windows-latest, upload asset on tag
```

## Engine (port of UsageEngine)

Same behaviour as Swift:

- Locator order: `ccusage.exe` next to `burnt.exe` → `ccusage` on PATH → unavailable
  (no npx fallback; Node is not assumed).
- Runner: `ccusage daily --json` and, when projects are requested, `ccusage session --json`.
  Subprocess started with `CREATE_NO_WINDOW` so no console flashes. 30 s timeout.
- Decoder accepts `period` or `date`. Tool classification by model prefix
  (`gpt-`, `o1`, `o3`, `codex` → codex, else claude).
- Aggregator: today, rolling 7-day week, month-to-date, all-time (from `totals`), 14-day
  sparkline, 84-day heatmap, by-tool, by-model (week window), cache savings (Claude rate
  table), avg/day, week trend vs previous 7 days, projected today (fraction ≥ 0.1).
- Project attribution: scan `%USERPROFILE%\.claude\projects\**\*.jsonl` and
  `%USERPROFILE%\.codex\sessions\**\rollout-*.jsonl` for the first `cwd`, join on session id.
- `Engine.LoadSummary(includeProjects)` returns `Summary` with `Status` ok / stale / noData,
  caching the last good result.

### Summary JSON (bridge contract, also used by ui/mock.js)

```json
{
  "status": "ok|stale|noData|error", "staleReason": "", "fetchedAt": "RFC3339",
  "today": {"cost":0,"inputTokens":0,"outputTokens":0,"cacheCreationTokens":0,"cacheReadTokens":0,"totalTokens":0},
  "week": {...same}, "month": {...}, "allTime": {...}, "lastWeek": {...},
  "avgPerDay": 0, "weekTrend": null, "projectedToday": null,
  "sparkline": [{"date":"YYYY-MM-DD","cost":0}],      // 14 entries, oldest first
  "heatmap":   [{"date":"YYYY-MM-DD","cost":0}],      // 84 entries, oldest first
  "byTool":    [{"tool":"claude|codex","cost":0,"tokens":0}],
  "byModel":   [{"model":"claude-opus-5","tool":"claude","cost":0,"tokens":0}],
  "byProject": [{"project":"name","cost":0,"tokens":0}],
  "cacheSavings": 0,
  "wrapped": {"monthCost":0,"allTimeCost":0,"topModels":[{"model":"","cost":0}],"busiestDay":{"date":"","cost":0},"claudeShare":0.0,"cacheSaved":0}
}
```

## Tray icon

- Text modes render ≤ 4 characters, single line, filling the icon: `< $10` → `$3.9`,
  `< $1000` → `$123`, else `$1.2k`; tokens → `340K`, `1.2M`; no data → `—`.
  Full precision lives in the tooltip (`Burnt · $12.34 today · $80.12 this week`) and menu.
- ICO contains 16, 20, 24, 32, 48 px PNG frames, each rendered natively (crisp per DPI).
- Colour: read `HKCU\...\Themes\Personalize\SystemUsesLightTheme`; light → near-black text,
  dark → white. Re-read on each refresh.
- Icon-only mode shows the flame (from `winres/icon.png`); `animateFlame` cycles 6 warped
  frames at ~6 fps only in icon-only mode.

## Tray menu (right-click) and popover (left-click)

Menu: Open Burnt · — · Today: $x (disabled) · Week: $x (disabled) · — · Refresh ·
Settings… · Check for Updates… · — · Quit Burnt.

Popover: 320×auto WebView2 window showing `ui/index.html`, the same dashboard as macOS
(hero, budget bar, stats row, avg/pace, sparkline, heatmap, by tool, by model + cache
savings, by project, stale badge, Wrapped card, settings page) gated by dashboard style.

JS bridge (Go `Bind`): `burnt_getState()`, `burnt_refresh()`, `burnt_setSettings(json)`,
`burnt_setLaunchAtLogin(bool)`, `burnt_getLaunchAtLogin()`, `burnt_checkUpdates()`,
`burnt_installUpdate()`, `burnt_openUrl(url)`, `burnt_copyText(text)`, `burnt_quit()`.
Go pushes `window.burntOnState(stateJson)` after every refresh. `state` =
`{summary, settings, version, update:{status,latest}, loading, launchAtLogin}`.
`ui/mock.js` implements the same globals with fixture data when opened in a browser.

## Settings

`%APPDATA%\Burnt\settings.json`, same keys/defaults as macOS: `menuBarMode` (todayCost),
`dailyBudget` (0), `dashboardStyle` (standard), `notifyBudget/notifyDailySummary/
notifyMilestones` (false), `animateFlame` (true), `autoUpdate` (true), `lastUpdateCheck`.
Launch at login = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\Burnt`.

## Notifications

Same triggers as macOS `Notifier`: budget crossed (once per day), daily summary at first
refresh of a new day, spend milestones ($10/$25/$50/$100/...). Toasts via beeep.

## Auto-update

Check `https://api.github.com/repos/mafex11/Burnt/releases/latest` on launch (if >24 h
since `lastUpdateCheck`) and daily; compare `tag_name` semver to build version; require an
asset named `Burnt-windows-x64.zip`. If `autoUpdate` on: download to `%TEMP%`, extract to
`<installDir>\.update\`, spawn `update.cmd` (waits for our PID, copies files over, relaunches),
quit. Otherwise expose "Update available — vX" + Install button in settings.

## Packaging and release

- `Burnt-windows-x64.zip` = `burnt.exe`, `ccusage.exe` (from the npm tarball of
  `@ccusage/ccusage-win32-x64@<pinned>`), `LICENSE`, `README-windows.txt`.
- Install dir `%LOCALAPPDATA%\Programs\Burnt`. `install.ps1`: TLS 1.2, resolve latest
  release, download zip, stop running Burnt, extract, `Unblock-File`, Start Menu shortcut,
  Run key, ensure WebView2 runtime (install Evergreen bootstrapper silently if missing),
  launch. One-liner:
  `irm https://raw.githubusercontent.com/mafex11/Burnt/main/install.ps1 | iex`
- CI: `.github/workflows/windows.yml` — job `test` (ubuntu: `go vet`, `go test ./...`,
  cross-build); job `build` (windows-latest: build with `-H windowsgui -X main.version`,
  fetch ccusage.exe, run `burnt.exe --version`, `--diagnose`, `--render-icon`, zip; on tag
  `v*` upload asset to the release, creating it if needed).
- Version single source for Windows: git tag → `-ldflags -X main.version`.
- First cross-platform release: v1.3.0 (mac Info.plist/cask bumped, mac zip built locally
  and uploaded alongside the Windows asset).

## Site + README

- Add an OS toggle (macOS / Windows) to Hero and Install; detect OS from `navigator`
  for the default tab. Windows tab shows the `irm … | iex` command, direct-download link
  `releases/latest/download/Burnt-windows-x64.zip`, SmartScreen note. Nav badge
  `macOS 14+ · Windows 10+`. Update metadata copy to "menu bar / system tray".
- README: platform badges, Windows install section (one-liner, manual, build from source),
  requirements per platform.

## Out of scope

ARM64 Windows, code signing, MSI/winget (follow-ups noted in README).

## Testing strategy (no Windows machine available locally)

- Engine, trayicon, settings, update-check: Go unit tests run on macOS with the shared fixtures.
- UI: `ui/index.html` + `mock.js` rendered in a local browser via Playwright for screenshots.
- Windows-only code: compiled with `GOOS=windows go vet/build` locally; executed on the
  `windows-latest` CI runner via headless flags (`--diagnose`, `--render-icon`).
