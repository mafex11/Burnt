# Burnt — Design Spec

**Date:** 2026-06-01
**Status:** Approved for planning
**One-liner:** A native macOS menu bar app that shows how much you've "burnt" on Claude Code and Codex — real-dollar cost and token usage, per day and per week, glanceable from the menu bar.

---

## 1. Purpose & Goals

The user runs Claude Code and Codex daily from the terminal, billed **per token via API** for both. They want a single glanceable place to answer:

- **How much money have I spent today / this week?** (real dollars)
- **How many tokens, and on what?** (volume by tool and model)
- **Where does the spend go?** (Claude vs Codex, which models, cache efficiency)

Success = clicking the menu bar shows an accurate, current "today's burn" in dollars within ~1 second, plus a small dashboard for the week and breakdowns — with numbers that match `ccusage` to the cent.

### In scope (v1)
- macOS menu bar app, installed via Homebrew cask (`brew install --cask burnt`).
- Cost (USD) + token volume for **today** and **this week**.
- Breakdowns: **by tool** (Claude/Codex), **by model**, **cache efficiency** ("saved $Y via cache").
- On-open refresh (re-query when the menu bar popover opens) + a manual refresh button.

### Out of scope (v1 — deferred)
- **By project / directory breakdown.** ccusage's JSON keys sessions by UUID with no cwd/path, so project attribution would require a separate raw-log reader. Deferred to v2.
- Real-time file watching / background daemon. v1 refreshes on open only.
- Monthly/quarterly historical analytics beyond what a 100+ day daily array trivially supports.
- Budgets, alerts, notifications.
- Windows/Linux. macOS only.

---

## 2. Key Decision: Wrap `ccusage`, Don't Re-Parse

`ccusage` (`npx ccusage@latest`) is mature prior art that already:
- Parses **both** Claude Code (`~/.claude/projects/*/*.jsonl`) and Codex (`~/.codex/sessions/**/rollout-*.jsonl`) — confirmed via `Detected: Claude, Codex`.
- Prices every model using the LiteLLM `model_prices_and_context_window.json` table (handles new models like opus-4-8, sonnet-4-5 automatically).
- Emits structured `--json` with per-day, per-model token + cost breakdowns.

**Decision:** Burnt wraps `ccusage daily --json` rather than re-implementing log parsing and pricing in Swift. This collapses the hardest, most fragile layer (two evolving log formats + a drifting pricing table) into a single maintained dependency, and lets Burnt focus on what ccusage lacks: **a native menu bar experience**.

**Differentiator vs ccusage:** ccusage is a CLI. Burnt is a glanceable macOS menu bar app (today's burn always one click away, brew-installable, native dashboard). Same data, different surface.

**Accepted tradeoff:** Burnt depends on Node/npx being available at runtime. Mitigated by (a) a Homebrew `depends_on` declaration, and (b) an explicit in-app "Node/npx required" state rather than a silent failure. See §6.

---

## 3. Architecture

```
Burnt.app  (SwiftUI menu bar app, distributed as a Homebrew cask)
├─ UsageEngine            (pure Swift package target — NO UI, unit-tested)
│   ├─ CcusageRunner       shells out: `ccusage daily --json`, decodes stdout
│   ├─ Models              DailyUsage, ModelBreakdown, Summary (Codable, mirror ccusage JSON)
│   ├─ Aggregator          rolls [DailyUsage] → today / this-week / by-tool / by-model / cache-savings
│   └─ NodeProbe           detects node/npx/ccusage; produces an EngineState
└─ BurntApp               (thin SwiftUI front-end)
    ├─ MenuBarView         "◔ Burnt  $X today" + refresh button
    └─ DashboardView       week sparkline, tool split, model table, "saved $Y via cache"
```

**Module boundary contract:** the UI calls one async function — `engine.loadSummary() -> EngineResult` — and never shells out, decodes JSON, or computes a price itself. `UsageEngine` never imports SwiftUI. The engine is its own Swift Package target so its tests run headless (no GUI launch).

---

## 4. Data Flow

1. User clicks the menu bar icon → popover opens → `onAppear` triggers `engine.loadSummary()`.
2. `NodeProbe` resolves an executable for ccusage (see §6 discovery order). If none → return `.nodeMissing`.
3. `CcusageRunner` runs `ccusage daily --json`, captures stdout, decodes into `[DailyUsage]` + `totals`.
4. `Aggregator` (pure functions) computes the `Summary`:
   - **Today:** the row whose `period` == today's local date (or zero if absent).
   - **This week:** sum of rows from Monday 00:00 local through now.
   - **By tool:** fold each row's `modelBreakdowns` into Claude vs Codex (classified by model name / agent tag).
   - **By model:** aggregate `modelBreakdowns` across the selected range.
   - **Cache savings:** see §5.
5. SwiftUI renders `Summary`. On error/timeout, render last-good cached `Summary` with a "stale" badge.

ccusage returns ~100+ daily rows in one call, so "today" and "this week" are cheap filters over that array — no extra invocations.

---

## 5. Data Model

Mirrors the verified `ccusage daily --json` shape:

```swift
struct DailyUsage: Codable {
    let period: String              // "2026-06-01" (local date)
    let inputTokens: Int
    let outputTokens: Int
    let cacheCreationTokens: Int
    let cacheReadTokens: Int
    let totalTokens: Int
    let totalCost: Double           // USD, already LiteLLM-priced
    let modelBreakdowns: [ModelBreakdown]
    let metadata: Metadata          // { agents: ["claude","codex"] }
}

struct ModelBreakdown: Codable {
    let modelName: String           // "claude-opus-4-8", "gpt-5", ...
    let inputTokens: Int
    let outputTokens: Int
    let cacheCreationTokens: Int
    let cacheReadTokens: Int
    let cost: Double
}
```

Burnt's own computed view (not from ccusage):

```swift
struct Summary {
    let today: Totals               // cost + tokens
    let thisWeek: Totals
    let weekByDay: [DayPoint]       // for the sparkline (7 points)
    let byTool: [ToolSlice]         // Claude vs Codex: cost + tokens + %
    let byModel: [ModelSlice]       // model name → cost + tokens (week range)
    let cacheSavings: Money         // see below
    let generatedAt: Date
}
```

**Tool classification:** a model belongs to Codex if its name matches the OpenAI family (`gpt-`, `o1`, `o3`, `codex`, etc.) or the row's `metadata.agents` indicates codex; otherwise Claude. The exact rule is finalized in the plan against live `modelBreakdowns` samples.

**Cache savings figure:** for Claude rows, `cacheReadTokens` are billed at the cheap cache-read rate. "Savings" = (what those tokens would have cost as fresh input) − (what they actually cost as cache reads). Because ccusage already folds the cheap rate into `totalCost`, Burnt computes the *counterfactual* using the same LiteLLM input vs cache-read rates. v1 may present this as an approximate "≈ $Y saved via cache" to avoid implying false precision. Finalized in the plan.

---

## 6. Node/ccusage Dependency Handling

This is the one real risk of the wrapper approach. Handling:

**Discovery order** (`NodeProbe`):
1. A bundled/known path to `ccusage` if the cask installs one (preferred — removes the npx spin-up cost).
2. `ccusage` on `PATH`.
3. `npx -y ccusage@latest` if `npx` is on `PATH`.
4. None found → `EngineState.nodeMissing`.

**Cask declaration:** the Homebrew cask declares `depends_on formula: "node"` so a fresh install pulls Node. (Whether to also vendor ccusage as a pinned dependency vs. rely on npx is decided in the plan; pinning avoids surprise format changes.)

**In-app state:** when `nodeMissing`, the popover shows a friendly message with a one-line install hint (`brew install node`) and a "Recheck" button — never a silent blank or crash.

**Performance:** `npx` cold-start can take seconds. To keep "today's burn" feeling instant, the menu bar shows the **last-good cached Summary immediately** on open, then refreshes in the background and updates in place. A pinned/known ccusage path (discovery step 1) is strongly preferred to minimize this.

---

## 7. Error Handling

| Condition | Behavior |
|---|---|
| Node/npx/ccusage not found | `.nodeMissing` state: install hint + Recheck button. |
| ccusage exits non-zero / times out | Show last-good cached Summary + "stale as of HH:MM" badge; log stderr. |
| JSON decode fails (format drift) | Same as above + a distinct "couldn't read ccusage output" diagnostic; never crash. |
| No usage data yet (empty array) | "No usage recorded yet" empty state. |
| First-ever launch, no cache | Show a spinner while the first ccusage call completes. |

A failure must never present a **wrong-but-confident** number. Stale data is always labeled as stale.

---

## 8. Testing Strategy

- **Fixtures:** capture real `ccusage daily --json` output (and edge cases: empty, codex-only, multi-model day) into the test bundle.
- **Decoding tests:** `DailyUsage`/`ModelBreakdown` decode the fixtures without loss.
- **Aggregator tests (pure):** today filter, week boundary (Monday/local-time edge), tool split, by-model rollup, cache-savings math — all asserted against fixture-derived expected values.
- **Golden-reference test:** Burnt's computed daily totals must equal `ccusage daily --json` totals to the cent (since we consume the same source, this guards our aggregation/rounding).
- **Integration smoke test:** if ccusage is present on the CI/dev machine, run it for real and assert a well-formed Summary.
- **NodeProbe tests:** simulate each discovery branch.

UI is intentionally thin and not the focus of automated testing; the engine holds the logic and the tests.

---

## 9. Distribution

- macOS app bundle `Burnt.app`, built with SwiftUI (`MenuBarExtra`).
- Homebrew **cask** named `burnt` (token verified free on Homebrew core; `burn` was taken by an unrelated CD-burning app, so the display name is "Burnt" and the token is `burnt`).
- Served initially from the user's own tap (`mafex/tap`) → `brew install --cask mafex/tap/burnt`, with the option to submit to official homebrew-cask later (token is unique there too).
- Cask `depends_on formula: "node"`.

---

## 10. Open Questions for the Plan

1. **Vendor/pin ccusage vs. rely on `npx -y ccusage@latest`?** Pinning a known-good version removes npx latency and format-drift risk but requires update maintenance. Lean: pin a version, refresh deliberately.
2. **Exact tool-classification rule** for Claude vs Codex from `modelBreakdowns` (validate against live Codex rows).
3. **Cache-savings presentation:** exact dollar vs "≈" approximate, and whether it's Claude-only.
4. **Week definition:** Monday-start vs rolling 7 days (lean: rolling 7 days ending today, simpler and matches "this week's burn" intuition; confirm in plan).
```
