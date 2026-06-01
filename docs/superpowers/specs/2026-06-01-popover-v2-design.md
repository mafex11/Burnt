# Burnt Popover v2 + Settings — Design Spec

**Date:** 2026-06-01
**Status:** Approved for planning
**One-liner:** Upgrade Burnt's menu bar popover with richer numbers, a polished tiered layout, color-coded breakdowns, and an inline settings panel (menu-bar display mode, daily budget bar, launch-at-login).

---

## 1. Purpose & Goals

Burnt v1 ships a working but visually flat popover: raw today/week figures, a small sparkline, and plain tool/model rows. This iteration makes it feel premium and surfaces more insight from data we already fetch, plus adds the settings users expect from a menu bar app.

Three goals:
1. **Prettier** — clear visual hierarchy, color-coded Claude/Codex, section headers, compact token formatting, sparkline hover tooltips.
2. **More numbers** — month-to-date, all-time, average $/day, week-over-week trend, today's projected pace.
3. **Settings** — choose what the menu bar label shows, set a daily budget (visual progress bar), and toggle launch-at-login.

**Success:** opening the popover gives an at-a-glance, well-organized picture; a gear toggles to settings; the menu bar respects the chosen display mode and a budget bar reflects daily spend — all with no new ccusage subprocess calls.

### In scope
- Engine: new derived `Summary` fields (all from the existing single ccusage call).
- Popover: tiered layout (hero → secondary stats → sparkline → tool → model), color system, % bars, compact tokens, sparkline hover.
- Settings: inline gear panel with menu-bar mode, daily budget, launch-at-login; persisted to `UserDefaults`.
- Budget: visual progress bar under the hero number (amber ≥ 80%, red > 100%).

### Out of scope (deferred)
- Notifications / alerts.
- Menu-bar label tint when over budget (visual bar only this round).
- Project / directory breakdown (separate future item; needs a raw-log reader).
- A second `ccusage monthly --json` call (month derived from daily rows instead).
- Clickable / expandable rows (tiered layout shows everything at once).

---

## 2. Engine Changes (`UsageEngine`)

All new numbers derive from the **single** `ccusage daily --json` call already made — no new subprocess work. `Aggregator.summary(from:referenceDate:)` already receives the reference date; the pace projection needs the current time-of-day, so the reference date is used for that too (keeps it pure/testable).

New fields on `Summary`:

```swift
public let monthToDate: Totals    // sum of daily rows in referenceDate's calendar month
public let allTime: Totals        // mapped from CcusageReport.totals (currently unused)
public let avgPerDay: Double      // thisWeek.cost / 7
public let lastWeek: Totals       // the 7 days BEFORE thisWeek (days -13..-7), for trend
public let weekTrend: Double?     // (thisWeek.cost - lastWeek.cost) / lastWeek.cost; nil if lastWeek == 0
public let projectedToday: Double? // today.cost / fractionOfDayElapsed; nil before threshold
```

Details:
- **monthToDate:** filter daily rows whose period is in the same year+month as `referenceDate`, sum into a `Totals`.
- **allTime:** `CcusageReport.totals` already has the token+cost fields; map into a `Totals`. (Note: ccusage `totals` has no per-day breakdown — that's fine, it's a single number.)
- **avgPerDay:** `thisWeek.cost / 7.0`.
- **lastWeek:** rolling window of the 7 days immediately preceding the current week window (offsets -13 through -7 from today, inclusive).
- **weekTrend:** percentage change; `nil` when `lastWeek.cost == 0` (avoid divide-by-zero / meaningless ∞). UI shows ▲ (green) when > 0, ▼ (red) when < 0, nothing when nil.
- **projectedToday:** `today.cost / fraction`, where `fraction = secondsElapsedToday / 86400`. Returns `nil` when `fraction < 0.1` (before ~2:24am-equivalent threshold, i.e. too early to extrapolate). Labeled "~" in UI to signal estimate.

`allTime` uses `Totals` for cost + tokens; the per-period `Totals` struct already fits.

---

## 3. Settings Store

A `Settings` type backed by `UserDefaults` (standard macOS prefs, persists at `~/Library/Preferences/dev.mafex.burnt.plist` — already covered by the cask's `zap`).

```swift
enum MenuBarMode: String, CaseIterable { case todayCost, todayTokens, weekCost }

final class Settings: ObservableObject {
    @AppStorage("menuBarMode") var menuBarMode: MenuBarMode = .todayCost
    @AppStorage("dailyBudget") var dailyBudget: Double = 0   // 0 = off
    @AppStorage("launchAtLogin") var launchAtLogin: Bool = false { didSet { applyLaunchAtLogin() } }
}
```

- `dailyBudget == 0` means "off" (no budget bar). UI presents an empty field for off.
- `menuBarMode` drives `AppModel.menuBarText`:
  - `.todayCost` → `$X.XX` (current behavior)
  - `.todayTokens` → compact today tokens, e.g. `1.2M`
  - `.weekCost` → week `$X.XX`
- Changing `launchAtLogin` calls into the launch helper (below).

### Launch-at-login

Isolated behind a small protocol so it's stubbable in tests and the system call lives in one place:

```swift
protocol LoginItemControlling { var isEnabled: Bool { get }; func enable() throws; func disable() throws }
```

Production impl uses `SMAppService.mainApp` (macOS 13+; we target 14): `register()` / `unregister()` / status check. No helper bundle, no `SMLoginItemSetEnabled`.

---

## 4. Popover Layout

Tiered summary view, top to bottom:

```
$4.21                          ⚙        ← hero: mode-aware big number; gear top-right
today   ▲ 12% vs last week              ← trend (green ▲ / red ▼); omitted if weekTrend nil
████████░░░░  42% of $10                ← budget bar (only if dailyBudget > 0)
──────────────────────────────────
Week      Month     All-time            ← secondary stats row (labels)
$28.90    $112.40   $7,468              ← values, monospaced
avg $4.13/day · pace ~$9.80 today       ← derived line; pace omitted if projectedToday nil
▁▃▅▇▅▃▂                                 ← sparkline (hover → "Jun 1 · $4.21")
── By tool ───────────────────────       ← section header
● Claude   ▓▓▓▓▓▓░  $3.10   1.2M         ← color dot + %-of-total bar + cost + compact tokens
● Codex    ▓▓░░░░░  $1.11   340K
── By model ──────────────────────
● claude-opus-4-8     $2.80   900K       ← top 5, dot colored by owning tool
● gpt-5               $1.11   340K
≈ $12.40 saved via cache                 ← green; only if cacheSavings > 0.01
──────────────────────────────────
stale · 2:14 PM              ↻   Quit     ← footer (stale badge only when stale)
```

**Color system:**
- Claude = amber `#F2A03D`, Codex = green `#3DB868`.
- Used consistently for: tool/model dots, the %-of-total bars, and the sparkline tint.
- A model row's dot color = `ToolClassifier.tool(forModel:)` of that model.

**Budget bar:** width = `min(today.cost / dailyBudget, 1.0)`; fill color green < 80%, amber 80–100%, red > 100% (bar caps at full width but label shows true %, e.g. "124% of $10").

**Token formatting:** compact — `1_234_567 → "1.2M"`, `340_000 → "340K"` (a small `formatTokens` helper).

**Sparkline hover:** each bar carries `.help("Jun 1 · $4.21")` (date + exact cost). If `.help` proves insufficient, an overlay tooltip on hover.

### Gear settings panel

Tapping ⚙ flips the popover to settings (← returns):

```
← Settings
  Menu bar shows:   [ Today $  ▾ ]      ← Picker bound to menuBarMode
  Daily budget:     [ $ 10.00  ]         ← TextField; empty/0 = off
  Launch at login:  [ ✔ ]                ← Toggle bound to launchAtLogin
```

State toggled via a `@State showingSettings` in the root popover view.

### File organization

`MenuBarView.swift` is split for focus (each file one responsibility):
- `MenuBarRootView.swift` — switches engine state; hosts summary ⇄ settings toggle.
- `SummaryView.swift` — the tiered summary layout.
- `SettingsView.swift` — the gear panel.
- `Components.swift` (existing) — gains `StatCell`, `BreakdownBar` (dot + %bar + cost + tokens), `BudgetBar`, `TrendArrow`; keep `Sparkline`, `StaleBadge`.
- `Formatters.swift` — `formatTokens`, `formatCost`, percentage helpers.

---

## 5. Error Handling

- Engine states (`success`/`stale`/`unavailable`/`noData`) unchanged; the richer summary only renders in `success`/`stale`.
- `weekTrend` / `projectedToday` are optionals → UI simply omits those lines when nil (no placeholder text).
- `dailyBudget == 0` → budget bar hidden entirely.
- Launch-at-login failure (rare `SMAppService` throw) → revert the toggle and log; never crash. The toggle reflects true `isEnabled` after the attempt.

---

## 6. Testing Strategy

- **Aggregator (extend existing pure tests):** month-to-date summing across a known fixture; all-time passthrough; avgPerDay; lastWeek window boundaries (-13..-7 inclusive); weekTrend math incl. the `lastWeek == 0 → nil` case; projectedToday with a fixed reference datetime (deterministic) incl. the early-morning `nil` threshold.
- **Settings:** round-trip via a `UserDefaults(suiteName:)` test instance — write each field, read back.
- **LaunchAtLogin:** test against a stub `LoginItemControlling`; assert `Settings.launchAtLogin` toggling calls enable/disable. Do NOT touch the real `SMAppService` in tests.
- **Formatters:** `formatTokens` (1.2M / 340K / 999 / 0), `formatCost`, percentage edges.
- **Views:** verified by build + launch (consistent with v1); no automated UI tests.

---

## 7. Open Questions

None — all resolved during brainstorming:
- Settings surface → inline gear panel.
- Layout → tiered (hero + stats + breakdowns).
- Budget → visual progress bar only (no menu-bar tint, no notifications).
- Month total → derived from daily rows (no second ccusage call).
