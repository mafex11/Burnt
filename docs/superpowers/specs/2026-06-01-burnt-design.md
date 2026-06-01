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

**Billing-model assumption (v1):** Burnt targets **API (pay-per-token)** billing, where the dollar figure is real money spent. The underlying token data (volume, by-tool, by-model, cache) is identical regardless of plan, so the app *runs* fine for Pro/Max/Team subscription accounts — but for a subscriber the `$` is the **API-equivalent value** of their usage (what it *would* cost on the API), not a bill. v1 labels the figure as spend and documents this caveat; a subscription "value" mode is a v2 path (see §11).

### In scope (v1)
- macOS menu bar app, installed via Homebrew cask (`brew install --cask burnt`).
- Cost (USD) + token volume for **today** and **this week**.
- Breakdowns: **by tool** (Claude/Codex), **by model**, **cache efficiency** ("saved $Y via cache").
- **Two refresh modes (cost-vs-freshness split):**
  - **Background poll every 60s** uses ccusage `--offline` (cached pricing) — keeps the menu bar number live without a network call every minute. Cheap, battery-friendly, works offline.
  - **Human-triggered refresh** (popover open + manual refresh button) fetches **live** LiteLLM prices (online). The numbers you actually read are accurate to the minute.

### Out of scope (v1 — deferred)
- **By project / directory breakdown.** ccusage's JSON keys sessions by UUID with no cwd/path, so project attribution would require a separate raw-log reader. Deferred to v2.
- File-system watching (FSEvents) for sub-second updates. The 60s poll is sufficient for v1; true live-watching is deferred.
- Launch-at-login. Considered but deferred — user starts the app; revisit if desired.
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

**Delivery of ccusage (UX decision):** Burnt **bundles a pinned, self-contained `ccusage` binary inside `Burnt.app`** rather than depending on Node/npx at runtime. This removes the ~150MB Node install, eliminates the multi-second `npx` cold-start, and works fully offline. Node/npx remains only a *graceful fallback* if the bundled binary is somehow unavailable. See §6.

---

## 3. Architecture

```
Burnt.app  (SwiftUI menu bar app, distributed as a Homebrew cask)
├─ UsageEngine            (pure Swift package target — NO UI, unit-tested)
│   ├─ CcusageRunner       shells out: `ccusage daily --json`, decodes stdout
│   ├─ Models              DailyUsage, ModelBreakdown, Summary (Codable, mirror ccusage JSON)
│   ├─ Aggregator          rolls [DailyUsage] → today / this-week / by-tool / by-model / cache-savings
│   └─ CcusageLocator      bundled-binary-first discovery; produces an EngineState
└─ BurntApp               (thin SwiftUI front-end)
    ├─ MenuBarView         "◔ Burnt  $X today" + refresh button
    └─ DashboardView       week sparkline, tool split, model table, "saved $Y via cache"
```

**Module boundary contract:** the UI calls one async function — `engine.loadSummary() -> EngineResult` — and never shells out, decodes JSON, or computes a price itself. `UsageEngine` never imports SwiftUI. The engine is its own Swift Package target so its tests run headless (no GUI launch).

---

## 4. Data Flow

1. User clicks the menu bar icon → popover opens → `onAppear` triggers `engine.loadSummary()`.
2. `CcusageLocator` resolves an executable for ccusage (see §6 discovery order — bundled binary first). If none → return `.unavailable`.
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

## 6. ccusage Delivery & Discovery

ccusage is **bundled inside the app**, so there is no runtime Node dependency in the normal path. A self-contained `ccusage` binary (produced at build time — see plan) ships at `Burnt.app/Contents/Resources/ccusage`.

**Discovery order** (`CcusageLocator`):
1. **Bundled binary** at `Bundle.main` Resources — the normal, preferred path. Instant, offline, no Node.
2. `ccusage` on `PATH` (developer convenience / fallback).
3. `npx -y ccusage@<pinnedVersion>` if `npx` is on `PATH` (last-resort fallback).
4. None found → `EngineState.unavailable`.

**Why bundle:** removes the ~150MB Node install, eliminates the multi-second `npx` cold-start on every reboot, and makes Burnt work offline. The bundled binary is the keystone UX decision — steps 2–3 exist only so a dev machine without the bundle still works.

**Cask declaration:** because ccusage is bundled, the cask does **not** force-install Node (`depends_on formula: "node"` is dropped). The only `depends_on` is the macOS version.

**In-app state:** the `.unavailable` state should be effectively unreachable in a real install (the binary is bundled). If it ever occurs, the popover shows a friendly diagnostic + "Recheck" button — never a silent blank or crash.

**Pricing source & freshness:** ccusage prices models from the LiteLLM table, which it **fetches from the network by default** and can read from a **bundled cache via `--offline`**. Burnt exploits both:
- The **60s background poll** runs `ccusage daily --json --offline` — cached pricing, no network, fast.
- **Popover-open and manual refresh** run `ccusage daily --json` (online) — live pricing for the numbers the user is actively reading.

The last-good `Summary` is shown immediately while any refresh runs, so the number never blanks. If an online fetch fails (no network), the engine surfaces the last-good `Summary` as `.stale` rather than erroring — the cached/offline numbers remain visible.

---

## 7. Error Handling

| Condition | Behavior |
|---|---|
| ccusage binary not found (should be unreachable — it's bundled) | `.unavailable` state: diagnostic + Recheck button. |
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
- **Integration smoke test:** if ccusage is present (bundled or on PATH), run it for real and assert a well-formed Summary.
- **CcusageLocator tests:** simulate each discovery branch (bundled binary, PATH, npx fallback, none).

UI is intentionally thin and not the focus of automated testing; the engine holds the logic and the tests.

---

## 9. Distribution

- macOS app bundle `Burnt.app`, built with SwiftUI (`MenuBarExtra`).
- Homebrew **cask** named `burnt` (token verified free on Homebrew core; `burn` was taken by an unrelated CD-burning app, so the display name is "Burnt" and the token is `burnt`).
- Served initially from the user's own tap (`mafex/tap`) → `brew install --cask mafex/tap/burnt`, with the option to submit to official homebrew-cask later (token is unique there too).
- **ccusage is bundled** in the app, so the cask does **not** `depends_on node`. Only `depends_on macos: ">= :sonoma"`.

### Code signing (UX decision)
- v1 ships **ad-hoc signed** (no paid Apple Developer ID). On first launch macOS Gatekeeper shows "cannot be opened because Apple cannot check it…". The README documents the one-time **right-click → Open** (or System Settings → Privacy & Security → Open Anyway) step.
- Notarization with a paid Developer ID ($99/yr) for a zero-warning install is explicitly deferred; revisit if distributing widely.

### First-run user flow (target experience)
1. `brew install --cask mafex/tap/burnt`
2. Launch Burnt; first time, right-click → Open (documented).
3. `◔ $X.XX` appears in the menu bar and **updates itself every 60s** (offline/cached pricing — no Node, no network, no spinner).
4. Click for the week sparkline + tool/model breakdown + cache-savings line — opening the popover does a live price fetch so what you read is current.

---

## 10. Resolved Decisions (formerly open questions)

1. **ccusage delivery:** **bundle** a pinned, self-contained binary in the app (not `npx` at runtime). See §6. Removes Node dependency + cold-start.
2. **Tool-classification rule:** by model-name prefix — `claude-*` → Claude; `gpt-*`/`o1`/`o3`/`codex*` → Codex; unknown → Claude (conservative default). Verified against live model names.
3. **Cache-savings presentation:** approximate "≈ $Y saved via cache", **Claude-only**, from LiteLLM input-vs-cache-read rate delta.
4. **Week definition:** **rolling 7 days** ending today (inclusive).
5. **Refresh:** background poll every **60s** (offline/cached pricing) + on-open & manual button (live online pricing).
6. **Code signing:** ad-hoc + documented right-click→Open for v1; notarization deferred.
7. **Billing model:** **API (pay-per-token)** for v1; the `$` is real spend. Subscription accounts work (token data is plan-agnostic) but their `$` is API-equivalent value, documented as a caveat. Subscription "value" mode deferred to v2.

---

## 11. Future Paths (v2+)

- **Subscription "value" mode:** a setting toggling the `$` framing between "spent" (API) and "≈ value / would-have-cost" (Pro/Max/Team), plus an optional "your plan costs $X/mo — usage value $Y" comparison. Same numbers, honest labels.
- **By project / directory breakdown** (requires a thin raw-log reader for cwd; see §1 out-of-scope).
- **Notarized, double-click install** (paid Apple Developer ID).
- **Live file-watching** (FSEvents) for sub-second updates instead of the 60s poll.
- **Launch-at-login** toggle.
```
