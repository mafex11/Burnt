# `windows/ui` — the popover dashboard

The Windows popover is a frameless 320 px-wide WebView2 window that loads
`index.html` from Go's embedded FS. This directory is plain HTML/CSS/JS:

| File | Purpose |
|---|---|
| `index.html` | Shell. Loads `mock.js` first, then `app.js`. |
| `styles.css` | All styling. Light/dark via `prefers-color-scheme` + a `data-theme` override. |
| `app.js` | Everything: formatters, three pages (dashboard / settings / wrapped), bridge calls. |
| `mock.js` | Browser-only fixture bridge. No-op when the Go host is present. |

**No build step and no network access.** No bundler, no npm, no CDN, no webfonts —
type is `"Segoe UI Variable Text", "Segoe UI Variable", "Segoe UI", system-ui`,
icons are inline SVG, everything else is CSS. Nothing in here may ever fetch a
remote resource: the popover has to render instantly and offline.

## Previewing in a browser

```sh
cd windows/ui
python3 -m http.server 8731
# then open http://localhost:8731/index.html
```

Open it at a 320 px-wide viewport. `mock.js` notices there is no Go host and
installs fixture `burnt_*` globals derived from
`Tests/UsageEngineTests/Fixtures/daily-normal.json` (all-time totals verbatim,
mixed Claude + Codex models) plus 84 days of deterministic pseudo-random daily
cost, so screenshots are reproducible.

Preview URL params (all handled by `mock.js`, except `theme`/`page` which
`app.js` reads — both are absent in production, so they are inert there):

| Param | Values | Default |
|---|---|---|
| `style` | `minimal` \| `standard` \| `detailed` | `standard` |
| `status` | `ok` \| `stale` \| `noData` \| `error` | `ok` |
| `theme` | `light` \| `dark` | follows the OS |
| `page` | `settings` \| `wrapped` | dashboard |
| `update` | `idle` \| `checking` \| `upToDate` \| `available` \| `updating` \| `error` | `idle` |
| `budget` | dollars (`0` = off) | ~60 % of today's fixture spend |

The document title mirrors the last `burnt_resize()` value (`Burnt — 575px`),
which is a quick way to check the height Go would be told to use.

## Bridge contract

Go binds these as global functions on `window`. Every one returns a Promise
(`webview2` `Bind` always does); `app.js` wraps them so a missing binding
degrades to a no-op rather than throwing.

| Function | Argument | Returns |
|---|---|---|
| `burnt_getState()` | — | state JSON **string** |
| `burnt_refresh()` | — | ignored; Go should push new state |
| `burnt_setSettings(json)` | settings JSON **string** | ignored |
| `burnt_setLaunchAtLogin(on)` | JS **boolean** | ignored |
| `burnt_getLaunchAtLogin()` | — | `"true"` / `"false"` or a bool (both accepted) |
| `burnt_checkUpdates()` | — | ignored; Go should push new state |
| `burnt_installUpdate()` | — | ignored |
| `burnt_openUrl(url)` | string | ignored |
| `burnt_copyText(text)` | string | ignored |
| `burnt_quit()` | — | ignored |
| `burnt_resize(heightPx)` | JS **number** | ignored |

Go → JS push: call `window.burntOnState(stateJson)` with the state as a **string**

Go → JS page hook: `window.burntShowPage('dashboard'|'settings'|'wrapped')` switches the visible page (used by the tray menu's "Settings…" item).
after every refresh, settings write, or update-state change. `app.js` defines it
and re-renders. (It also accepts an already-parsed object, which makes the mock
simpler.)

### `state`

```json
{
  "summary":  { /* Summary JSON, see the design spec */ },
  "settings": { "menuBarMode": "todayCost", "dailyBudget": 0,
                "dashboardStyle": "standard", "notifyBudget": false,
                "notifyDailySummary": false, "notifyMilestones": false,
                "animateFlame": true, "autoUpdate": true,
                "lastUpdateCheck": "RFC3339" },
  "version": "1.3.0",
  "update":  { "status": "idle|checking|upToDate|available|updating|error",
               "latest": "1.4.0", "error": "" },
  "loading": false,
  "launchAtLogin": false
}
```

`summary.status` drives the whole dashboard: `ok` and `stale` render the full
thing (`stale` adds a `stale · <relative time>` badge), `noData` renders the
empty state, `error` renders `summary.staleReason` as an error line.

The UI is tolerant about two field names for the same data so it works against
either the spec's names or a direct transcription of the Swift model:
`sparkline`/`weekByDay`, `heatmap`/`heatmapDays`, `model`/`modelName`,
`project`/`name`, `tokens`/`totalTokens`.

### Window sizing

`app.js` calls `burnt_resize(h)` after **every** render (inside a
`requestAnimationFrame`, so layout has settled). Notes for the Go side:

- `h` is already clamped to **720**. Content taller than that gets a thin
  in-page scrollbar (`html.scrolls`); everything below 720 must be shown in full,
  with no scrollbar.
- The same height may be reported repeatedly (e.g. three times across one
  refresh). **Go should no-op when the height is unchanged** rather than calling
  `SetWindowPos` each time, otherwise the popover visibly twitches.
- The height is measured from `#root`, not from
  `document.documentElement.scrollHeight`, because the latter is floored at the
  viewport height — a dashboard that got *shorter* (e.g. Detailed → Minimal)
  would keep reporting the old, larger height and the window would never shrink.
  `#root` is the only child of `<body>` and `<body>` has no padding or margin, so
  its box height is the content height, and it equals
  `documentElement.scrollHeight` whenever the content genuinely overflows.
- The window must be **320 px wide**. CSS uses `width: 100%` / `max-width: 320px`
  rather than a hard 320 so that when the scrollbar appears it takes width from
  the layout instead of clipping the right edge of the card.

### Settings writes

Every control writes through immediately — there is no Save button:

- selects and toggles call `burnt_setSettings(JSON.stringify(settings))` with the
  full settings object, then re-render;
- **Launch at login** is *not* part of settings JSON; it calls
  `burnt_setLaunchAtLogin(bool)` and mirrors `state.launchAtLogin`;
- the **Daily budget** field commits on every keystroke but deliberately does
  **not** re-render (that would destroy the caret). `$`, commas and whitespace
  are stripped; anything unparseable or negative becomes `0` (= off).

## Parity with the macOS popover

Content and gating mirror `Sources/Burnt/SummaryView.swift`. `dashboardStyle`
levels are additive (`minimal` < `standard` < `detailed`):

| Section | Level |
|---|---|
| Hero (today's cost, `today`, week trend arrow, gear, refresh) | all |
| Budget bar (only when `settings.dailyBudget > 0`) | all |
| Week / Month / All-time stats | all |
| `avg $X/day · pace ~$Y today` | detailed |
| 14-day sparkline | all |
| "Last 12 weeks" 84-day heatmap + `$0 → $1k+` legend | standard+ |
| "By tool" bars | standard+ |
| "By model" (top 5) + `≈ $X saved via cache` (when > $0.01) | detailed |
| "By project" (top 5, hidden when empty) | detailed |
| Stale badge | when `status == "stale"` |
| Footer: "Burnt Wrapped…" + version | all |

Ported verbatim from Swift:

- `Formatters.cost` — `$%.2f` under $1000, whole dollars with thousands
  separators at or above it (`$7,468`).
- `Formatters.tokens` — `340K` / `1.2M` / `12B`; one decimal below 100, none above.
- `Formatters.percent` — magnitude only, whole percent. Direction comes from the
  arrow's colour (up = more spend = red, down = green).
- The heatmap ramp from `HeatmapView.swift` — **absolute** $50 bands capped at
  $1000, interpolated across 8 sRGB stops (teal → green → lime → yellow → amber
  → orange → red → deep ember), so a $161 day and a $400 day read as clearly
  different shades. Zero-cost days use a flat neutral tint. Keep this absolute:
  a max-relative or quantile scale collapses every big day onto one colour.

Windows-specific differences from macOS:

- Settings says **"Tray shows"**, not "Menu bar shows".
- The **Animate flame** toggle carries a hint that it only applies in icon-only
  mode (Windows renders the dollar figure *into* the 16×16 tray icon, so there is
  no flame to animate in the text modes).
- The **Updates** block has an explicit "Install" button (calling
  `burnt_installUpdate`) instead of macOS's silent `brew upgrade`.
- **Burnt Wrapped** offers "Copy as text" via `burnt_copyText`; the macOS
  "Copy Image"/"Save PNG…" render path does not exist here.

## Conventions

- ES5-flavoured, IIFE-scoped, no modules — WebView2 loads these as classic
  scripts from an embedded FS and there is no transpiler in the loop.
- Every interpolated value goes through `esc()`. Model and project names come
  from local log files, but they still reach the DOM as HTML.
- All click/change/input/hover handling is delegated from `#root`, so a render is
  a single `innerHTML` assignment with no listener bookkeeping.
- Hover readouts (sparkline, heatmap) mutate the caption element's text directly
  instead of re-rendering — a re-render would drop the hover.
