# Burnt Phase 1 — Wow Features Design Spec

**Date:** 2026-06-01
**Status:** Approved for planning
**One-liner:** Four features that make Burnt impressive and sticky — Burnt Wrapped (shareable card), spend heatmap, native notifications, and by-project breakdown — all local, all in the existing macOS app.

---

## 1. Purpose & Goals

Burnt v1.1.1 is a polished glance tool. Phase 1 adds the things that make a dev tool get *shared* and *kept*:

- **Burnt Wrapped** — a one-click shareable PNG of your AI spend (growth lever: people post these).
- **Spend heatmap** — a GitHub-contribution-style grid (instant "whoa").
- **Notifications** — budget/summary/milestone alerts (turns a glance into a habit).
- **By-project** — cost per repo/directory (power-user respect; the deferred v2 item).

**Constraints:** stays 100% local (no backend/accounts — those are a future phase). Keeps **ccusage as the single pricing source of truth** — we never price tokens ourselves. Builds on the existing `UsageEngine` / `BurntCore` / SwiftUI structure without disturbing what works.

**Success:** a user opens Burnt, sees a beautiful heatmap, gets a useful daily-summary notification, drills into which repo is eating their budget, and one-clicks a Wrapped card they want to post.

### In scope
- `Summary.byProject` + a `ProjectAttributor` (ccusage session ⨝ raw-log cwd map).
- `Notifier` (budget threshold, daily summary, spend milestones) — opt-in, deduped.
- `HeatmapView` — shown in the **Detailed** dashboard style only.
- `WrappedView` — month + all-time card, Copy/Save PNG via SwiftUI `ImageRenderer`.

### Out of scope (later phases)
- Web dashboard, cloud sync, accounts, backend.
- Windows / cross-platform native app.
- Subscription **quota** tracking (Anthropic/OpenAI expose no public "% of plan used" API; subscription `$` stays "API-equivalent value", as today).
- Auto-posting Wrapped to social (manual copy/save only).

---

## 2. Architecture

Layered onto the current structure; the engine stays the tested core.

```
Sources/UsageEngine/
  Aggregator.swift        MODIFY  add byProject rollup; extend daily series to ~84 days for heatmap
  ProjectAttributor.swift NEW     ccusage session --json ⨝ raw-log {sessionID → cwd}
  Summary.swift           MODIFY  + byProject: [ProjectSlice]; + heatmapDays: [DayPoint]
Sources/BurntCore/
  Notifier.swift          NEW     pure decision logic (what/when to fire) + NotificationPosting protocol
  WrappedData.swift       NEW     builds the Wrapped card's data model from a Summary
Sources/Burnt/
  HeatmapView.swift       NEW     GitHub-style grid (Detailed style only)
  WrappedView.swift       NEW     the share card; Copy/Save PNG via ImageRenderer
  SummaryView.swift       MODIFY  show HeatmapView + byProject section in Detailed style
  SettingsView.swift      MODIFY  notification toggles (3) + "Burnt Wrapped" button
  AppModel.swift          MODIFY  run Notifier after each load; expose Wrapped presentation
Tests/UsageEngineTests/   ProjectAttributorTests (join logic)
Tests/BurntTests/         NotifierTests (dedup/threshold logic via stub poster), WrappedDataTests
```

Two visual features (heatmap, Wrapped) use SwiftUI's native `ImageRenderer` — no Chrome/HTML pipeline inside the app. `WrappedView` is the same SwiftUI view for on-screen display and PNG export.

---

## 3. By-Project (the one new data pipeline)

`ProjectAttributor` (in UsageEngine), run during the same load as the main summary:

1. **Cost per session:** run `ccusage session --json` → `[{period: sessionUUID, totalCost, totalTokens, agent, modelBreakdowns}]`. Already priced by ccusage. `period` is the session ID.
2. **Session → cwd map** (thin scan, not full parse, mtime-cached):
   - **Claude:** `~/.claude/projects/<dir>/<sessionID>.jsonl` — filename is the session ID; read the first line containing `cwd`.
   - **Codex:** `~/.codex/sessions/**/rollout-*.jsonl` — read line 1 (`session_meta`) for `id` + `cwd`.
3. **Join + group:** `sessionCost[id]` joined with `sessionCwd[id]`; group by **project = last path component of cwd**, summing cost + tokens. Key on full path; display the leaf; disambiguate with parent only on leaf collision.
4. **Emit** `byProject: [ProjectSlice]` for the current range, sorted by cost desc.

```swift
public struct ProjectSlice: Sendable, Equatable {
    public let name: String        // display leaf, e.g. "personal"
    public let path: String        // full cwd, the dedup key
    public let cost: Double
    public let totalTokens: Int
}
```

**Honesty rules (baked in):**
- A session is attributed to its first/primary cwd (multi-dir sessions are rare; noted, not split).
- Sessions ccusage reports but whose log can't be located (or vice versa) → bucketed as a `"Unknown"` project, so by-project totals reconcile with the headline totals.
- Performance: only the **first line** of each session file is read; mtime-cached. Hundreds of sessions = milliseconds.

**Why this design:** keying the join on session ID means we never re-price — ccusage says "session X cost $Y", we say "session X ran in project Z." By-project always reconciles with ccusage's totals, and we take on zero pricing-drift risk.

---

## 4. Notifications

`Notifier` in BurntCore — pure decision logic (what to fire, when), with the actual posting behind a protocol so tests never post real notifications.

```swift
public protocol NotificationPosting { func post(title: String, body: String, id: String) }
```

Three opt-in kinds (all **off by default**; first enable triggers the macOS permission prompt):

1. **Budget threshold** — when today's spend crosses **80%** and **100%** of the daily budget (only if a budget > 0 is set). Each level fires at most once per calendar day.
2. **Daily summary** — at the first refresh of a new local day: *"Yesterday: $12.40 · 80% opus."* Once per day.
3. **Spend milestones** — when month-to-date crosses **$50 / $100 / $250 / $500 / $1000**, each once per calendar month.

**Dedup state** persists in UserDefaults keyed by `(kind, period)` so the 60s poll never re-fires the same alert. `Notifier.evaluate(summary:now:state:) -> [Notification]` is pure and the unit-tested core; `AppModel` calls it after each load and routes results to the real poster.

---

## 5. Heatmap

`HeatmapView`, rendered **only in the Detailed dashboard style** (keeps Minimal/Standard compact while honoring "inline in the popover").

- GitHub-contribution-style grid: ~12 weeks (84 days) of daily cost, columns = weeks, rows = weekdays.
- Color scale: empty = faint gray; non-zero scaled faint→`#F2A03D`→`#d6420f` (ember) by cost quartile/relative max.
- Hover a cell → caption "Jun 8 · $4.21" (same hover pattern as the sparkline, reliable in a popover).
- Data: `Summary.heatmapDays: [DayPoint]` — the daily series extended from 14 to 84 days (same zero-filled construction in `Aggregator`, just a longer window; sourced from all ccusage daily rows).

---

## 6. Burnt Wrapped

`WrappedView` (SwiftUI) + a **"Burnt Wrapped"** button in Settings.

- Two variants via a toggle: **This Month** and **All-Time**.
- Card content: headline spend, total tokens (compact), model split (horizontal bars), busiest day, Claude/Codex split, and a cache-savings flex line — in the amber/ember brand style with the app icon.
- `WrappedData` (BurntCore) builds the card's data model from a `Summary` (busiest day = max cost in the range; model split from `byModel`; etc.) — pure + unit-tested.
- **Export:** `ImageRenderer(content: WrappedView(...))` at 2x → **Copy to clipboard** and **Save PNG…** buttons. Same view renders on screen and to the image.
- No auto-posting.

---

## 7. Error Handling

- `ProjectAttributor`: if `ccusage session --json` fails, by-project is simply absent (the section hides) — never blocks the main summary. Unmatched sessions → "Unknown" bucket.
- `Notifier`: if permission denied, toggles still flip but nothing posts (and we don't nag). Posting failures are logged, never crash.
- `ImageRenderer` failure (rare) → show an error state on the Wrapped sheet, don't crash.
- Heatmap with sparse/no data → empty grid (all faint cells), not a blank space.

---

## 8. Testing Strategy

- **ProjectAttributorTests** (UsageEngineTests): join logic over fixtures — session cost + a `{id→cwd}` map → expected `[ProjectSlice]`; leaf-collision disambiguation; Unknown bucketing; totals reconcile.
- **NotifierTests** (BurntTests): `evaluate(...)` fires the right notifications at the right thresholds; dedup state prevents re-fire across repeated calls; nothing fires when toggles off / no budget. Uses a stub `NotificationPosting`.
- **WrappedDataTests** (BurntTests): busiest-day, model-split, totals derived correctly from a fixture Summary.
- **Aggregator**: extend tests for the 84-day heatmap series + byProject presence.
- Views (Heatmap, Wrapped) verified by build + launch (consistent with prior UI work).

---

## 9. Open Questions

None — resolved in brainstorming:
- By-project data → ccusage `session --json` ⨝ thin raw-log cwd map (verified: session JSON has no cwd; raw logs do).
- Wrapped → month + all-time, Copy + Save PNG, no auto-post.
- Notifications → all three kinds, opt-in, deduped.
- Heatmap → inline but gated to the **Detailed** dashboard style (compromise between "inline" and keeping Minimal/Standard compact).
