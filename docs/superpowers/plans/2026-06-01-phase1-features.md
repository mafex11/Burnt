# Burnt Phase 1 — Wow Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add four local features to the Burnt macOS app — by-project breakdown, spend heatmap, native notifications, and Burnt Wrapped (shareable PNG) — keeping ccusage as the sole pricing source.

**Architecture:** Engine gains an 84-day series + a `ProjectAttributor` (joins `ccusage session --json` cost-per-session to a thin raw-log `sessionID→cwd` map). `BurntCore` gains a pure `Notifier` (decision logic behind a `NotificationPosting` protocol) and `WrappedData`. SwiftUI gains `HeatmapView` and `WrappedView` (PNG via `ImageRenderer`), shown in the Detailed dashboard style; `AppModel`/`SettingsView` wire notifications + the Wrapped button.

**Tech Stack:** Swift 6.3, SwiftPM, SwiftUI, `ImageRenderer`, `UserNotifications`, XCTest.

**Critical operational facts (every task):**
- Work from `/Users/mafex/code/personal/burnt` — standalone git repo (remote `mafex11/Burnt`, branch `main`). Run git INSIDE it.
- Tests/builds REQUIRE the prefix `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`.
- Targets: `UsageEngine` (lib), `BurntCore` (lib, deps UsageEngine), `Burnt` (executable, deps both), `UsageEngineTests`, `BurntTests` (deps BurntCore).

---

## Existing types (use, don't redefine)

```swift
// Summary.swift
struct Tool: enum .claude/.codex (String raw)
struct Totals { var cost: Double; var inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, totalTokens: Int }  // defaults 0
struct DayPoint { let date: String; let cost: Double }
struct ToolSlice / ModelSlice { ... cost: Double; totalTokens: Int; tool/modelName }
struct Summary {
  today, thisWeek: Totals; weekByDay: [DayPoint]; byTool: [ToolSlice]; byModel: [ModelSlice];
  cacheSavings: Double; monthToDate, allTime: Totals; avgPerDay: Double; lastWeek: Totals;
  weekTrend: Double?; projectedToday: Double?; generatedAt: Date
}
// Aggregator.summary(from: CcusageReport, referenceDate: Date) -> Summary  (only construction site)
//   private helpers: parse(period)->Date?, key(Date)->String, totals(for:)->Totals, add(_,_)->Totals
// CcusageReport { daily: [DailyUsage]; totals }; DailyUsage { period, ...tokens, totalCost, modelBreakdowns, metadata }
// CcusageRunner: struct, init(invocation:timeout:), fetchDailyReport() throws -> CcusageReport; pinnedVersion="20.0.6"
//   CcusageInvocation { executable: String; leadingArgs: [String] }
// UsageEngine class: loadSummary() -> EngineResult (.success/.stale/.unavailable/.noData)
// BurntCore: Settings (ObservableObject; menuBarMode, dashboardStyle, dailyBudget: Double, launchAtLogin),
//   MenuBarMode, DashboardStyle (.minimal<.standard<.detailed, Comparable), Formatters.cost/tokens/percent
```

---

## File Structure

```
Sources/UsageEngine/
  Summary.swift            MODIFY  + heatmapDays: [DayPoint]; + byProject: [ProjectSlice]; + ProjectSlice struct
  Aggregator.swift         MODIFY  build 84-day heatmapDays; accept byProject param (computed by facade)
  ProjectAttributor.swift  NEW     sessionID→cwd map (raw logs) ⨝ ccusage session costs → [ProjectSlice]
  CcusageRunner.swift      MODIFY  + fetchSessionReport() (ccusage session --json)
  Models.swift             MODIFY  + SessionReport / SessionRow decodable (period, totalCost, totalTokens, agent)
  UsageEngine.swift        MODIFY  loadSummary builds byProject via ProjectAttributor + injects into Summary
Sources/BurntCore/
  Notifier.swift           NEW     NotificationPosting protocol + pure evaluate(...) + dedup state
  WrappedData.swift        NEW     build Wrapped card model from a Summary
Sources/Burnt/
  HeatmapView.swift        NEW     12-week grid (Detailed style)
  WrappedView.swift        NEW     share card + Copy/Save PNG via ImageRenderer
  NotificationService.swift NEW    UNUserNotificationCenter impl of NotificationPosting
  SummaryView.swift        MODIFY  Detailed style: heatmap + by-project section
  SettingsView.swift       MODIFY  + 3 notification toggles + "Burnt Wrapped" button
  AppModel.swift           MODIFY  run Notifier after each load; settings flags; present Wrapped
Tests/UsageEngineTests/    ProjectAttributorTests, Aggregator heatmap test
Tests/BurntTests/          NotifierTests, WrappedDataTests
```

---

## Task 1: Engine — 84-day heatmap series

**Why:** The heatmap needs ~12 weeks of daily cost. The engine already builds a 14-day `weekByDay`; add a parallel 84-day `heatmapDays` from the same source (all ccusage daily rows).

**Files:**
- Modify: `Sources/UsageEngine/Summary.swift`
- Modify: `Sources/UsageEngine/Aggregator.swift`
- Test: `Tests/UsageEngineTests/AggregatorTests.swift`

- [ ] **Step 1: Add `heatmapDays` to `Summary`** — insert after `weekByDay`:

```swift
    public let heatmapDays: [DayPoint]   // 84 points oldest→newest, zero-filled (heatmap)
```
Final member order: today, thisWeek, weekByDay, heatmapDays, byTool, byModel, cacheSavings, monthToDate, allTime, avgPerDay, lastWeek, weekTrend, projectedToday, byProject, generatedAt.
(NOTE: `byProject` is added in Task 4; for THIS task, add `heatmapDays` only and append it right after `weekByDay`, leaving the rest unchanged. Task 4 adds `byProject` before `generatedAt`.)

- [ ] **Step 2: Write the failing test** — append to `AggregatorTests.swift`:

```swift
    func testHeatmapDaysIsEightyFourZeroFilled() {
        let r = report([day("2026-06-08", cost: 1, models: [mb("claude-opus-4-8", cost: 1, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.heatmapDays.count, 84)
        XCTAssertEqual(s.heatmapDays.last?.date, "2026-06-08")    // newest = today
        XCTAssertEqual(s.heatmapDays.first?.date, "2026-03-17")   // 83 days before 06-08
        XCTAssertEqual(s.heatmapDays.last?.cost ?? -1, 1, accuracy: 0.001)
    }
```

- [ ] **Step 3: Run to confirm failure**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter AggregatorTests`
Expected: FAIL — Summary has no member `heatmapDays`.

- [ ] **Step 4: Implement in `Aggregator.swift`.** Find the existing `weekByDay` construction (the `points` array built with `stride(from: 6...` — actually 14-day now; the variable is `points`). Right AFTER that block, add a parallel 84-day build:

```swift
        // 84-day series for the heatmap, same zero-filled construction over all rows.
        let costByDayAll = Dictionary(grouping: report.daily, by: { $0.period })
            .mapValues { $0.reduce(0) { $0 + $1.totalCost } }
        var heatPoints: [DayPoint] = []
        for offset in stride(from: 83, through: 0, by: -1) {
            let date = cal.date(byAdding: .day, value: -offset, to: today)!
            let k = key(date)
            heatPoints.append(DayPoint(date: k, cost: costByDayAll[k] ?? 0))
        }
```

Then add `heatmapDays: heatPoints` to the `return Summary(...)` call, positioned right after `weekByDay: points,`.

- [ ] **Step 5: Run to confirm pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`
Expected: all pass (existing + new heatmap test).

- [ ] **Step 6: Commit**

```bash
git add Sources/UsageEngine/Summary.swift Sources/UsageEngine/Aggregator.swift Tests/UsageEngineTests/AggregatorTests.swift
git commit -m "engine: add 84-day heatmapDays series to Summary"
```

---

## Task 2: Models — SessionReport decodable

**Why:** `ProjectAttributor` needs `ccusage session --json`, which has a different top-level shape (`session` array) than the daily report. Add a decodable for it.

**Files:**
- Modify: `Sources/UsageEngine/Models.swift`
- Test: `Tests/UsageEngineTests/DecodingTests.swift`
- Create: `Tests/UsageEngineTests/Fixtures/session-sample.json`

- [ ] **Step 1: Create the fixture** `Tests/UsageEngineTests/Fixtures/session-sample.json`:

```json
{
 "session": [
  { "agent": "claude", "period": "01467451-f660-4bd0-a16c-3298b534e6fd", "totalCost": 14.60, "totalTokens": 15917152 },
  { "agent": "codex", "period": "019c0ec2-6c8f-7df2-943e-506d7d4c0c82", "totalCost": 21.67, "totalTokens": 45340860 }
 ],
 "totals": { "inputTokens": 0, "outputTokens": 0, "cacheCreationTokens": 0, "cacheReadTokens": 0, "totalTokens": 61258012, "totalCost": 36.27 }
}
```

- [ ] **Step 2: Write the failing test** — append to `DecodingTests.swift`:

```swift
    func testDecodesSessionReport() throws {
        let report = try JSONDecoder().decode(SessionReport.self, from: fixture("session-sample"))
        XCTAssertEqual(report.session.count, 2)
        XCTAssertEqual(report.session[0].period, "01467451-f660-4bd0-a16c-3298b534e6fd")
        XCTAssertEqual(report.session[0].totalCost, 14.60, accuracy: 0.001)
        XCTAssertEqual(report.session[1].agent, "codex")
    }
```

- [ ] **Step 3: Run to confirm failure**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter DecodingTests`
Expected: FAIL — `SessionReport` undefined.

- [ ] **Step 4: Add to `Models.swift`:**

```swift
public struct SessionReport: Decodable, Sendable {
    public let session: [SessionRow]
}

public struct SessionRow: Decodable, Sendable {
    public let agent: String?
    public let period: String        // the session UUID
    public let totalCost: Double
    public let totalTokens: Int
}
```

- [ ] **Step 5: Run to confirm pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter DecodingTests`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add Sources/UsageEngine/Models.swift Tests/UsageEngineTests/DecodingTests.swift Tests/UsageEngineTests/Fixtures/session-sample.json
git commit -m "engine: add SessionReport/SessionRow decodable for ccusage session --json"
```

---

## Task 3: CcusageRunner — fetchSessionReport()

**Why:** Run `ccusage session --json` through the same subprocess machinery as the daily report.

**Files:**
- Modify: `Sources/UsageEngine/CcusageRunner.swift`

- [ ] **Step 1: Add a generic runner + the session method.** In `CcusageRunner.swift`, the existing `fetchDailyReport()` builds `process.arguments = invocation.leadingArgs + ["daily", "--json"]`, runs, drains pipes, decodes `CcusageReport`. Refactor so both reports share the subprocess code. Replace the body of `fetchDailyReport()` to delegate, and add `fetchSessionReport()`:

```swift
    public func fetchDailyReport() throws -> CcusageReport {
        try run(subcommand: "daily", as: CcusageReport.self)
    }

    public func fetchSessionReport() throws -> SessionReport {
        try run(subcommand: "session", as: SessionReport.self)
    }

    private func run<T: Decodable>(subcommand: String, as type: T.Type) throws -> T {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: invocation.executable)
        process.arguments = invocation.leadingArgs + [subcommand, "--json"]

        var env = ProcessInfo.processInfo.environment
        let extra = "/opt/homebrew/bin:/usr/local/bin"
        env["PATH"] = (env["PATH"].map { "\(extra):\($0)" }) ?? extra
        process.environment = env

        let out = Pipe(); let err = Pipe()
        process.standardOutput = out; process.standardError = err

        let outBox = DataBox(), errBox = DataBox()
        let drain = DispatchGroup()
        readInBackground(out.fileHandleForReading, into: outBox, group: drain)
        readInBackground(err.fileHandleForReading, into: errBox, group: drain)

        try process.run()
        let finished = waitForExit(process, timeout: timeout)
        guard finished else {
            process.terminate(); _ = drain.wait(timeout: .now() + 2)
            throw RunError.timedOut
        }
        drain.wait()
        guard process.terminationStatus == 0 else {
            throw RunError.nonZeroExit(code: process.terminationStatus,
                stderr: String(decoding: errBox.data, as: UTF8.self))
        }
        do { return try JSONDecoder().decode(T.self, from: outBox.data) }
        catch { throw RunError.decodeFailed(String(describing: error)) }
    }
```

IMPORTANT: the existing file already has `readInBackground`, `waitForExit`, and the private `DataBox` class (from the deadlock fix). Reuse them — do NOT duplicate. Just remove the now-dead inline body of the old `fetchDailyReport()` and keep the two public methods + the new generic `run`.

- [ ] **Step 2: Build + run full suite (no behavior change for daily):**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift build && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`
Expected: `Build complete!`, all tests pass (the daily smoke test still works through the refactor).

- [ ] **Step 3: Commit**

```bash
git add Sources/UsageEngine/CcusageRunner.swift
git commit -m "engine: add fetchSessionReport() via shared subprocess runner"
```

---

## Task 4: ProjectAttributor — join sessions to projects

**Why:** The core of by-project. ccusage gives cost-per-session-ID; the raw logs give session-ID→cwd. Join them, group by project (cwd leaf). Pure join logic is unit-tested with injected inputs; the filesystem scan is a separate injectable function.

**Files:**
- Modify: `Sources/UsageEngine/Summary.swift` (add `ProjectSlice`)
- Create: `Sources/UsageEngine/ProjectAttributor.swift`
- Test: `Tests/UsageEngineTests/ProjectAttributorTests.swift`

- [ ] **Step 1: Add `ProjectSlice` to `Summary.swift`** (near the other slice structs):

```swift
public struct ProjectSlice: Sendable, Equatable {
    public let name: String     // display leaf, e.g. "personal"
    public let path: String     // full cwd (dedup key); "" for the Unknown bucket
    public let cost: Double
    public let totalTokens: Int
}
```

- [ ] **Step 2: Write the failing test** `Tests/UsageEngineTests/ProjectAttributorTests.swift`:

```swift
import XCTest
@testable import UsageEngine

final class ProjectAttributorTests: XCTestCase {
    private func row(_ id: String, _ cost: Double, _ tok: Int) -> SessionRow {
        SessionRow(agent: "claude", period: id, totalCost: cost, totalTokens: tok)
    }

    func testGroupsSessionsByCwdLeaf() {
        let sessions = [row("a", 5, 100), row("b", 3, 50), row("c", 2, 20)]
        let cwdMap = ["a": "/Users/me/code/personal", "b": "/Users/me/code/personal", "c": "/Users/me/code/work"]
        let result = ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
        // personal = 5+3 = 8, work = 2; sorted by cost desc
        XCTAssertEqual(result.map(\.name), ["personal", "work"])
        XCTAssertEqual(result[0].cost, 8, accuracy: 0.001)
        XCTAssertEqual(result[0].totalTokens, 150)
        XCTAssertEqual(result[1].cost, 2, accuracy: 0.001)
    }

    func testUnmappedSessionsBucketAsUnknown() {
        let sessions = [row("a", 5, 100), row("z", 4, 40)]
        let cwdMap = ["a": "/Users/me/code/personal"]   // "z" missing
        let result = ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
        let unknown = result.first { $0.name == "Unknown" }
        XCTAssertNotNil(unknown)
        XCTAssertEqual(unknown?.cost ?? 0, 4, accuracy: 0.001)
    }

    func testLeafCollisionDisambiguatesWithParent() {
        let sessions = [row("a", 5, 10), row("b", 3, 10)]
        let cwdMap = ["a": "/x/api", "b": "/y/api"]
        let result = ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
        // both leaves are "api" → disambiguate with parent
        XCTAssertEqual(Set(result.map(\.name)), Set(["x/api", "y/api"]))
    }
}
```

- [ ] **Step 3: Run to confirm failure**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter ProjectAttributorTests`
Expected: FAIL — `ProjectAttributor` undefined.

- [ ] **Step 4: Implement `Sources/UsageEngine/ProjectAttributor.swift`:**

```swift
import Foundation

public enum ProjectAttributor {
    /// Pure join: session costs + a sessionID→cwd map → projects grouped by cwd leaf,
    /// sorted by cost desc. Unmapped sessions fall into an "Unknown" bucket.
    public static func group(sessions: [SessionRow], cwdBySession: [String: String]) -> [ProjectSlice] {
        struct Agg { var cost = 0.0; var tokens = 0 }
        var byPath: [String: Agg] = [:]   // key: full cwd, or "" for unknown

        for s in sessions {
            let path = cwdBySession[s.period] ?? ""
            var a = byPath[path] ?? Agg()
            a.cost += s.totalCost; a.tokens += s.totalTokens
            byPath[path] = a
        }

        // Determine display names; disambiguate colliding leaves with their parent.
        let paths = byPath.keys.filter { !$0.isEmpty }
        let leaf: (String) -> String = { ($0 as NSString).lastPathComponent }
        var leafCounts: [String: Int] = [:]
        for p in paths { leafCounts[leaf(p), default: 0] += 1 }

        func name(for path: String) -> String {
            if path.isEmpty { return "Unknown" }
            let l = leaf(path)
            if (leafCounts[l] ?? 0) > 1 {
                let parent = ((path as NSString).deletingLastPathComponent as NSString).lastPathComponent
                return parent.isEmpty ? l : "\(parent)/\(l)"
            }
            return l
        }

        return byPath.map { (path, a) in
            ProjectSlice(name: name(for: path), path: path, cost: a.cost, totalTokens: a.tokens)
        }.sorted { $0.cost > $1.cost }
    }

    /// Build sessionID→cwd by reading the FIRST cwd-bearing line of each session log.
    /// Injectable file enumeration keeps this testable; the default scans the real dirs.
    public static func buildCwdMap(claudeRoot: URL, codexRoot: URL) -> [String: String] {
        var map: [String: String] = [:]
        let fm = FileManager.default

        // Claude: ~/.claude/projects/<dir>/<sessionID>.jsonl ; filename = session ID, first line has cwd.
        if let proj = try? fm.contentsOfDirectory(at: claudeRoot.appendingPathComponent("projects"),
                                                  includingPropertiesForKeys: nil) {
            for dir in proj {
                guard let files = try? fm.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil) else { continue }
                for f in files where f.pathExtension == "jsonl" {
                    let sid = f.deletingPathExtension().lastPathComponent
                    if let cwd = firstCwd(in: f) { map[sid] = cwd }
                }
            }
        }

        // Codex: ~/.codex/sessions/**/rollout-*.jsonl ; line 1 session_meta has id + cwd.
        if let en = fm.enumerator(at: codexRoot.appendingPathComponent("sessions"),
                                  includingPropertiesForKeys: nil) {
            for case let f as URL in en where f.lastPathComponent.hasPrefix("rollout-") && f.pathExtension == "jsonl" {
                if let (id, cwd) = codexIdAndCwd(in: f) { map[id] = cwd }
            }
        }
        return map
    }

    /// First "cwd" value found in a Claude jsonl (reads only until found).
    private static func firstCwd(in url: URL) -> String? {
        guard let h = try? FileHandle(forReadingFrom: url) else { return nil }
        defer { try? h.close() }
        let data = h.readData(ofLength: 64_000)   // first ~64KB is plenty for line 1-2
        guard let text = String(data: data, encoding: .utf8) else { return nil }
        for line in text.split(separator: "\n") {
            if let obj = try? JSONSerialization.jsonObject(with: Data(line.utf8)) as? [String: Any],
               let cwd = obj["cwd"] as? String { return cwd }
        }
        return nil
    }

    /// Codex session_meta (line 1) → (id, cwd). payload may be nested under "payload".
    private static func codexIdAndCwd(in url: URL) -> (String, String)? {
        guard let h = try? FileHandle(forReadingFrom: url) else { return nil }
        defer { try? h.close() }
        let data = h.readData(ofLength: 64_000)
        guard let text = String(data: data, encoding: .utf8),
              let firstLine = text.split(separator: "\n").first,
              let obj = try? JSONSerialization.jsonObject(with: Data(firstLine.utf8)) as? [String: Any]
        else { return nil }
        let p = (obj["payload"] as? [String: Any]) ?? obj
        guard let id = p["id"] as? String, let cwd = p["cwd"] as? String else { return nil }
        return (id, cwd)
    }
}
```

- [ ] **Step 5: Run to confirm pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter ProjectAttributorTests`
Expected: PASS (3 tests). (Only `group` is unit-tested; the filesystem scanners are exercised by the integration smoke test in Task 5.)

- [ ] **Step 6: Commit**

```bash
git add Sources/UsageEngine/Summary.swift Sources/UsageEngine/ProjectAttributor.swift Tests/UsageEngineTests/ProjectAttributorTests.swift
git commit -m "engine: add ProjectAttributor (session⨝cwd join) + ProjectSlice"
```

---

## Task 5: Wire byProject into Summary + UsageEngine

**Why:** Add `byProject` to `Summary` and have the facade compute it (run `ccusage session`, build the cwd map, group, inject). By-project failure must never block the main summary.

**Files:**
- Modify: `Sources/UsageEngine/Summary.swift`
- Modify: `Sources/UsageEngine/Aggregator.swift`
- Modify: `Sources/UsageEngine/UsageEngine.swift`

- [ ] **Step 1: Add `byProject` to `Summary`** — insert right before `generatedAt`:

```swift
    public let byProject: [ProjectSlice]   // by cwd, current data; empty if unavailable
```

- [ ] **Step 2: Give `Aggregator.summary` a `byProject` parameter.** Change the signature:

```swift
    public static func summary(from report: CcusageReport, referenceDate: Date,
                               byProject: [ProjectSlice] = []) -> Summary {
```
and add `byProject: byProject,` to the `return Summary(...)` (right before `generatedAt:`). The default `[]` keeps all existing Aggregator tests compiling unchanged.

- [ ] **Step 3: Compute byProject in `UsageEngine.loadSummary()`.** The current method resolves an invocation, runs `fetchDailyReport()`, then `Aggregator.summary(from:referenceDate:)`. Update the success path to also fetch sessions + build projects, defensively:

```swift
            let report = try runner.fetchDailyReport()
            // ... existing empty-report handling stays ...
            let projects = (try? Self.buildProjects(runner: runner)) ?? []
            let summary = Aggregator.summary(from: report, referenceDate: now(), byProject: projects)
```
Add a private helper on `UsageEngine`:

```swift
    private static func buildProjects(runner: CcusageRunner) throws -> [ProjectSlice] {
        let sessions = try runner.fetchSessionReport().session
        let home = FileManager.default.homeDirectoryForCurrentUser
        let cwdMap = ProjectAttributor.buildCwdMap(
            claudeRoot: home.appendingPathComponent(".claude"),
            codexRoot: home.appendingPathComponent(".codex"))
        return ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
    }
```
(`try?` ensures a session-fetch or filesystem failure yields `[]` — by-project simply hides, main summary unaffected.)

- [ ] **Step 4: Update the integration smoke test** in `Tests/UsageEngineTests/UsageEngineTests.swift` to tolerate the new field — it already switches on `.success`/`.stale`; add an assertion that byProject is an array (possibly empty):

```swift
        case .success(let s), .stale(let s, _):
            XCTAssertEqual(s.weekByDay.count, 14)
            XCTAssertEqual(s.heatmapDays.count, 84)
            XCTAssertGreaterThanOrEqual(s.thisWeek.cost, 0)
            _ = s.byProject   // present; may be empty
```

- [ ] **Step 5: Build + full suite**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift build && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`
Expected: all pass. The smoke test (if ccusage present) now also exercises the real session fetch + cwd map.

- [ ] **Step 6: Commit**

```bash
git add Sources/UsageEngine/Summary.swift Sources/UsageEngine/Aggregator.swift Sources/UsageEngine/UsageEngine.swift Tests/UsageEngineTests/UsageEngineTests.swift
git commit -m "engine: compute byProject in loadSummary (defensive; never blocks summary)"
```

---

## Task 6: Notifier (BurntCore) — pure decision logic

**Why:** Decide WHICH notifications to fire and dedup them, without posting real ones in tests. The decision function is pure; posting is behind a protocol.

**Files:**
- Create: `Sources/BurntCore/Notifier.swift`
- Test: `Tests/BurntTests/NotifierTests.swift`

- [ ] **Step 1: Write the failing test** `Tests/BurntTests/NotifierTests.swift`:

```swift
import XCTest
@testable import BurntCore

final class NotifierTests: XCTestCase {
    // Minimal input the Notifier needs (decoupled from UsageEngine's Summary).
    private func input(today: Double, month: Double, budget: Double,
                       yesterdayCost: Double = 0, yesterdayTopModel: String = "")
        -> NotifierInput {
        NotifierInput(todayCost: today, monthCost: month, dailyBudget: budget,
                      yesterdayCost: yesterdayCost, yesterdayTopModel: yesterdayTopModel)
    }

    func testBudget80And100FireOncePerDay() {
        var state = NotifierState()
        let opts = NotifierOptions(budgetAlerts: true, dailySummary: false, milestones: false)
        let day = "2026-06-08"
        // 90% of $10 → 80% alert fires, 100% does not
        var out = Notifier.evaluate(input: input(today: 9, month: 9, budget: 10),
                                    options: opts, dayKey: day, monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.contains { $0.id.contains("budget80") })
        XCTAssertFalse(out.contains { $0.id.contains("budget100") })
        // same day again at 90% → no re-fire
        out = Notifier.evaluate(input: input(today: 9, month: 9, budget: 10),
                                options: opts, dayKey: day, monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.isEmpty)
        // crosses 100% → 100 fires once
        out = Notifier.evaluate(input: input(today: 11, month: 11, budget: 10),
                                options: opts, dayKey: day, monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.contains { $0.id.contains("budget100") })
    }

    func testNoBudgetAlertsWhenDisabledOrNoBudget() {
        var state = NotifierState()
        let opts = NotifierOptions(budgetAlerts: true, dailySummary: false, milestones: false)
        let out = Notifier.evaluate(input: input(today: 99, month: 99, budget: 0),
                                    options: opts, dayKey: "d", monthKey: "m", state: &state)
        XCTAssertTrue(out.isEmpty)   // budget == 0 means off
    }

    func testMilestonesFireOncePerMonth() {
        var state = NotifierState()
        let opts = NotifierOptions(budgetAlerts: false, dailySummary: false, milestones: true)
        var out = Notifier.evaluate(input: input(today: 1, month: 120, budget: 0),
                                    options: opts, dayKey: "d", monthKey: "2026-06", state: &state)
        // crossed 50 and 100 → both fire (highest-crossed logic), once
        XCTAssertTrue(out.contains { $0.id.contains("milestone100") })
        out = Notifier.evaluate(input: input(today: 1, month: 130, budget: 0),
                                options: opts, dayKey: "d", monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.isEmpty)   // no new milestone crossed
    }

    func testDailySummaryFiresOncePerDay() {
        var state = NotifierState()
        let opts = NotifierOptions(budgetAlerts: false, dailySummary: true, milestones: false)
        var out = Notifier.evaluate(input: input(today: 1, month: 1, budget: 0, yesterdayCost: 12.4, yesterdayTopModel: "opus"),
                                    options: opts, dayKey: "2026-06-08", monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.contains { $0.id.contains("summary") })
        out = Notifier.evaluate(input: input(today: 1, month: 1, budget: 0, yesterdayCost: 12.4, yesterdayTopModel: "opus"),
                                options: opts, dayKey: "2026-06-08", monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.isEmpty)
    }
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter NotifierTests`
Expected: FAIL — Notifier/types undefined.

- [ ] **Step 3: Implement `Sources/BurntCore/Notifier.swift`:**

```swift
import Foundation

public protocol NotificationPosting {
    func post(title: String, body: String, id: String)
}

public struct NotifierInput: Sendable {
    public let todayCost: Double
    public let monthCost: Double
    public let dailyBudget: Double      // 0 = off
    public let yesterdayCost: Double
    public let yesterdayTopModel: String
    public init(todayCost: Double, monthCost: Double, dailyBudget: Double,
                yesterdayCost: Double, yesterdayTopModel: String) {
        self.todayCost = todayCost; self.monthCost = monthCost; self.dailyBudget = dailyBudget
        self.yesterdayCost = yesterdayCost; self.yesterdayTopModel = yesterdayTopModel
    }
}

public struct NotifierOptions: Sendable {
    public let budgetAlerts: Bool
    public let dailySummary: Bool
    public let milestones: Bool
    public init(budgetAlerts: Bool, dailySummary: Bool, milestones: Bool) {
        self.budgetAlerts = budgetAlerts; self.dailySummary = dailySummary; self.milestones = milestones
    }
}

/// Dedup state — which (kind, period) notifications already fired. Codable so the
/// app can persist it in UserDefaults across the 60s poll and app restarts.
public struct NotifierState: Codable, Sendable {
    public var fired: Set<String> = []
    public init() {}
}

public struct PendingNotification: Sendable, Equatable {
    public let title: String, body: String, id: String
}

public enum Notifier {
    static let milestoneLevels: [Double] = [50, 100, 250, 500, 1000]

    /// Pure: returns the notifications to post given the inputs + options, and marks
    /// them fired in `state` so repeated calls (the 60s poll) don't re-fire.
    public static func evaluate(input: NotifierInput, options: NotifierOptions,
                                dayKey: String, monthKey: String,
                                state: inout NotifierState) -> [PendingNotification] {
        var out: [PendingNotification] = []
        func fireOnce(_ id: String, _ make: () -> PendingNotification) {
            guard !state.fired.contains(id) else { return }
            state.fired.insert(id); out.append(make())
        }

        if options.budgetAlerts, input.dailyBudget > 0 {
            let ratio = input.todayCost / input.dailyBudget
            if ratio >= 0.8 {
                fireOnce("budget80-\(dayKey)") {
                    PendingNotification(title: "80% of daily budget",
                        body: "Today: \(Self.money(input.todayCost)) of \(Self.money(input.dailyBudget)).",
                        id: "budget80-\(dayKey)") }
            }
            if ratio >= 1.0 {
                fireOnce("budget100-\(dayKey)") {
                    PendingNotification(title: "Daily budget reached",
                        body: "Today: \(Self.money(input.todayCost)) — over your \(Self.money(input.dailyBudget)) cap.",
                        id: "budget100-\(dayKey)") }
            }
        }

        if options.milestones {
            for level in milestoneLevels where input.monthCost >= level {
                fireOnce("milestone\(Int(level))-\(monthKey)") {
                    PendingNotification(title: "Burnt \(Self.money(level)) this month",
                        body: "Month to date: \(Self.money(input.monthCost)).",
                        id: "milestone\(Int(level))-\(monthKey)") }
            }
        }

        if options.dailySummary {
            fireOnce("summary-\(dayKey)") {
                let model = input.yesterdayTopModel.isEmpty ? "" : " · mostly \(input.yesterdayTopModel)"
                return PendingNotification(title: "Yesterday on Burnt",
                    body: "\(Self.money(input.yesterdayCost)) burnt\(model).",
                    id: "summary-\(dayKey)") }
        }
        return out
    }

    private static func money(_ v: Double) -> String { String(format: "$%.2f", v) }
}
```

- [ ] **Step 4: Run to confirm pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter NotifierTests`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add Sources/BurntCore/Notifier.swift Tests/BurntTests/NotifierTests.swift
git commit -m "add Notifier: pure budget/summary/milestone decision logic + dedup"
```

---

## Task 7: WrappedData (BurntCore)

**Why:** Build the Wrapped card's data model from a Summary — pure, testable, so the view is dumb.

**Files:**
- Create: `Sources/BurntCore/WrappedData.swift`
- Test: `Tests/BurntTests/WrappedDataTests.swift`

NOTE: `WrappedData` must NOT depend on UsageEngine's `Summary` (BurntCore already imports UsageEngine, so it *can* — but to keep the test simple we pass primitive inputs). Use a small input struct.

- [ ] **Step 1: Write the failing test** `Tests/BurntTests/WrappedDataTests.swift`:

```swift
import XCTest
@testable import BurntCore

final class WrappedDataTests: XCTestCase {
    func testBuildsHeadlineAndModelSplit() {
        let w = WrappedData(
            title: "This Month",
            totalCost: 112.40, totalTokens: 47_000_000,
            models: [("claude-opus-4-8", 90), ("gpt-5", 22)],
            busiestDay: "Jun 8", busiestDayCost: 14.2,
            claudeShare: 0.8, cacheSaved: 30.0)
        XCTAssertEqual(w.headlineCost, "$112")          // >= 1000 rule not hit; uses cost()
        XCTAssertEqual(w.headlineTokens, "47.0M")
        XCTAssertEqual(w.topModelName, "claude-opus-4-8")
        XCTAssertEqual(w.modelBars.count, 2)
        XCTAssertEqual(w.modelBars[0].fraction, 1.0, accuracy: 0.001) // top model = full bar
        XCTAssertEqual(w.modelBars[1].fraction, 22.0/90.0, accuracy: 0.001)
    }
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter WrappedDataTests`
Expected: FAIL — `WrappedData` undefined.

- [ ] **Step 3: Implement `Sources/BurntCore/WrappedData.swift`:**

```swift
import Foundation

public struct WrappedData: Sendable {
    public struct ModelBar: Sendable, Equatable {
        public let name: String
        public let cost: Double
        public let fraction: Double   // 0...1 of the top model's cost
    }

    public let title: String
    public let headlineCost: String
    public let headlineTokens: String
    public let topModelName: String
    public let modelBars: [ModelBar]
    public let busiestDay: String
    public let busiestDayCost: String
    public let claudeShare: Double      // 0...1
    public let cacheSaved: String

    /// models: (name, cost) pairs, any order.
    public init(title: String, totalCost: Double, totalTokens: Int,
                models: [(String, Double)], busiestDay: String, busiestDayCost: Double,
                claudeShare: Double, cacheSaved: Double) {
        self.title = title
        self.headlineCost = Formatters.cost(totalCost)
        self.headlineTokens = Formatters.tokens(totalTokens)
        let sorted = models.sorted { $0.1 > $1.1 }
        self.topModelName = sorted.first?.0 ?? "—"
        let top = max(sorted.first?.1 ?? 0, 0.0001)
        self.modelBars = sorted.prefix(5).map { ModelBar(name: $0.0, cost: $0.1, fraction: $0.1 / top) }
        self.busiestDay = busiestDay
        self.busiestDayCost = Formatters.cost(busiestDayCost)
        self.claudeShare = claudeShare
        self.cacheSaved = Formatters.cost(cacheSaved)
    }
}
```

- [ ] **Step 4: Run to confirm pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter WrappedDataTests`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Sources/BurntCore/WrappedData.swift Tests/BurntTests/WrappedDataTests.swift
git commit -m "add WrappedData: pure model for the Burnt Wrapped card"
```

---

## Task 8: Settings — notification toggles

**Why:** Three opt-in bools (budget alerts, daily summary, milestones) persisted like the other settings, plus the dedup state blob.

**Files:**
- Modify: `Sources/BurntCore/Settings.swift`
- Test: `Tests/BurntTests/SettingsTests.swift`

- [ ] **Step 1: Write the failing test** — append to `SettingsTests.swift`:

```swift
    func testNotificationTogglesDefaultOffAndPersist() {
        let d = freshDefaults()
        let s1 = Settings(defaults: d, loginItem: StubLoginItem())
        XCTAssertFalse(s1.notifyBudget)
        XCTAssertFalse(s1.notifyDailySummary)
        XCTAssertFalse(s1.notifyMilestones)
        s1.notifyBudget = true; s1.notifyMilestones = true
        let s2 = Settings(defaults: d, loginItem: StubLoginItem())
        XCTAssertTrue(s2.notifyBudget)
        XCTAssertTrue(s2.notifyMilestones)
        XCTAssertFalse(s2.notifyDailySummary)
    }
```

- [ ] **Step 2: Run to confirm failure**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter SettingsTests`
Expected: FAIL — no `notifyBudget` member.

- [ ] **Step 3: Add to `Settings.swift`.** Add three keys to the `Key` enum:

```swift
        static let notifyBudget = "notifyBudget"
        static let notifyDailySummary = "notifyDailySummary"
        static let notifyMilestones = "notifyMilestones"
```
In `init`, initialize them (UserDefaults bool defaults to false when unset):

```swift
        self._notifyBudget = Published(initialValue: defaults.bool(forKey: Key.notifyBudget))
        self._notifyDailySummary = Published(initialValue: defaults.bool(forKey: Key.notifyDailySummary))
        self._notifyMilestones = Published(initialValue: defaults.bool(forKey: Key.notifyMilestones))
```
Add the published properties (after `dailyBudget`):

```swift
    @Published public var notifyBudget: Bool { didSet { defaults.set(notifyBudget, forKey: Key.notifyBudget) } }
    @Published public var notifyDailySummary: Bool { didSet { defaults.set(notifyDailySummary, forKey: Key.notifyDailySummary) } }
    @Published public var notifyMilestones: Bool { didSet { defaults.set(notifyMilestones, forKey: Key.notifyMilestones) } }
```

- [ ] **Step 4: Run to confirm pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter SettingsTests`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Sources/BurntCore/Settings.swift Tests/BurntTests/SettingsTests.swift
git commit -m "settings: add notifyBudget/notifyDailySummary/notifyMilestones toggles"
```

---

## Task 9: Heatmap, Wrapped & NotificationService views

**Why:** The three new SwiftUI pieces. UI — verified by build (full build + launch in Task 10).

**Files:**
- Create: `Sources/Burnt/HeatmapView.swift`
- Create: `Sources/Burnt/WrappedView.swift`
- Create: `Sources/Burnt/NotificationService.swift`

- [ ] **Step 1: Create `Sources/Burnt/HeatmapView.swift`:**

```swift
import SwiftUI
import UsageEngine
import BurntCore

/// GitHub-contribution-style grid of daily cost (84 days). Columns = weeks.
struct HeatmapView: View {
    let days: [DayPoint]          // oldest→newest, length 84
    @State private var hovered: String?

    private var maxCost: Double { max(days.map(\.cost).max() ?? 0.01, 0.01) }

    private func color(_ cost: Double) -> Color {
        if cost <= 0 { return Color.secondary.opacity(0.15) }
        let t = min(cost / maxCost, 1.0)
        // faint amber → ember
        return Color(red: 0.95, green: 0.62 - 0.2 * t, blue: 0.24 - 0.14 * t)
            .opacity(0.35 + 0.65 * t)
    }

    // chunk into 12 columns of 7 (oldest week first)
    private var weeks: [[DayPoint]] {
        stride(from: 0, to: days.count, by: 7).map { Array(days[$0..<min($0+7, days.count)]) }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(hovered.flatMap { key in days.first { $0.date == key }.map { "\(pretty($0.date)) · \(Formatters.cost($0.cost))" } } ?? "last 12 weeks")
                .font(.caption2).foregroundStyle(hovered == nil ? .secondary : .primary)
            HStack(spacing: 3) {
                ForEach(Array(weeks.enumerated()), id: \.offset) { _, week in
                    VStack(spacing: 3) {
                        ForEach(week, id: \.date) { d in
                            RoundedRectangle(cornerRadius: 2)
                                .fill(color(d.cost))
                                .frame(width: 13, height: 13)
                                .onHover { inside in hovered = inside ? d.date : (hovered == d.date ? nil : hovered) }
                        }
                    }
                }
            }
        }
    }

    private func pretty(_ iso: String) -> String {
        let p = iso.split(separator: "-").compactMap { Int($0) }
        guard p.count == 3 else { return iso }
        let m = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
        return "\(p[1] >= 1 && p[1] <= 12 ? m[p[1]-1] : "?") \(p[2])"
    }
}
```

- [ ] **Step 2: Create `Sources/Burnt/NotificationService.swift`:**

```swift
import Foundation
import UserNotifications
import BurntCore

/// Real macOS poster. Requests permission lazily on first post.
final class NotificationService: NotificationPosting {
    private var authorized = false
    private var requested = false

    func post(title: String, body: String, id: String) {
        ensureAuth { [weak self] ok in
            guard ok, self != nil else { return }
            let c = UNMutableNotificationContent()
            c.title = title; c.body = body
            let req = UNNotificationRequest(identifier: id, content: c, trigger: nil)
            UNUserNotificationCenter.current().add(req)
        }
    }

    private func ensureAuth(_ done: @escaping (Bool) -> Void) {
        if authorized { done(true); return }
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { [weak self] ok, _ in
            self?.authorized = ok; self?.requested = true
            DispatchQueue.main.async { done(ok) }
        }
    }
}
```

- [ ] **Step 3: Create `Sources/Burnt/WrappedView.swift`:**

```swift
import SwiftUI
import BurntCore

/// The shareable Burnt Wrapped card. The SAME view renders on screen and to PNG.
struct WrappedView: View {
    let data: WrappedData

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Image("AppIconImage") // falls back gracefully if absent
                    .resizable().frame(width: 0, height: 0) // placeholder; icon optional
                Text("Burnt · \(data.title)")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Color(red: 0.95, green: 0.62, blue: 0.24))
            }
            Text(data.headlineCost)
                .font(.system(size: 54, weight: .bold)).monospacedDigit()
                .foregroundStyle(.white)
            Text("\(data.headlineTokens) tokens burnt")
                .font(.system(size: 16)).foregroundStyle(.white.opacity(0.7))

            VStack(alignment: .leading, spacing: 6) {
                ForEach(data.modelBars, id: \.name) { b in
                    HStack(spacing: 8) {
                        Text(b.name).font(.system(size: 12)).foregroundStyle(.white.opacity(0.85))
                            .frame(width: 150, alignment: .leading).lineLimit(1)
                        GeometryReader { geo in
                            Capsule().fill(Color(red: 0.95, green: 0.62, blue: 0.24))
                                .frame(width: max(4, geo.size.width * b.fraction), height: 8)
                        }.frame(height: 8)
                    }
                }
            }
            HStack(spacing: 20) {
                stat("Busiest day", "\(data.busiestDay) · \(data.busiestDayCost)")
                stat("Cache saved", data.cacheSaved)
            }
            Text("How much have you burnt?")
                .font(.system(size: 13)).foregroundStyle(.white.opacity(0.55))
        }
        .padding(28)
        .frame(width: 420)
        .background(
            LinearGradient(colors: [Color(red:0.16,green:0.10,blue:0.07), Color(red:0.05,green:0.05,blue:0.06)],
                           startPoint: .top, endPoint: .bottom)
        )
    }

    private func stat(_ k: String, _ v: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(k).font(.system(size: 11)).foregroundStyle(.white.opacity(0.55))
            Text(v).font(.system(size: 14, weight: .medium)).foregroundStyle(.white)
        }
    }
}
```
NOTE: drop the broken `Image("AppIconImage")` line entirely — just keep the `Text("Burnt · …")` title (the icon isn't in an asset catalog). The final header HStack should contain only the title Text.

- [ ] **Step 4: Build (views compile in isolation; full wiring in Task 10):**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift build 2>&1 | grep -iE "HeatmapView|WrappedView|NotificationService" | head`
Expected: no errors attributed to these three files (errors from unwired SummaryView/AppModel are fine until Task 10).

- [ ] **Step 5: Commit**

```bash
git add Sources/Burnt/HeatmapView.swift Sources/Burnt/WrappedView.swift Sources/Burnt/NotificationService.swift
git commit -m "add HeatmapView, WrappedView (ImageRenderer-ready), NotificationService"
```

---

## Task 10: Wire it all into the UI

**Why:** Show the heatmap + by-project in Detailed style, add notification toggles + Wrapped button to Settings, run the Notifier after each load, and present the Wrapped sheet with Copy/Save PNG. This is where everything compiles + launches.

**Files:**
- Modify: `Sources/Burnt/SummaryView.swift`
- Modify: `Sources/Burnt/SettingsView.swift`
- Modify: `Sources/Burnt/AppModel.swift`

- [ ] **Step 1: SummaryView — add heatmap + by-project in Detailed style.** In `SummaryView.swift`, find the existing `if style >= .detailed { sectionHeader("By model") ... }` block. Immediately AFTER the sparkline line `Sparkline(points: summary.weekByDay)`, add (still inside body):

```swift
            // Heatmap — Detailed style only.
            if style >= .detailed {
                sectionHeader("Last 12 weeks")
                HeatmapView(days: summary.heatmapDays)
            }
```
And after the existing "By model" detailed block, add a by-project section:

```swift
            if style >= .detailed, !summary.byProject.isEmpty {
                sectionHeader("By project")
                ForEach(summary.byProject.prefix(5), id: \.path) { p in
                    BreakdownBar(color: .secondary, label: p.name,
                                 fraction: p.cost / maxProjectCost, cost: p.cost, tokens: p.totalTokens)
                }
            }
```
Add the helper near `maxModelCost`:

```swift
    private var maxProjectCost: Double { max(summary.byProject.map(\.cost).max() ?? 0.01, 0.01) }
```

- [ ] **Step 2: SettingsView — notification toggles + Wrapped button.** Add a callback the root can use to present Wrapped. Change the struct to accept it:

```swift
struct SettingsView: View {
    @ObservedObject var settings: BurntCore.Settings
    let onBack: () -> Void
    var onShowWrapped: () -> Void = {}
```
After the "Launch at login" Toggle, before the `Divider()`, add:

```swift
            Divider()
            Text("Notifications").font(.caption).foregroundStyle(.secondary)
            Toggle("Budget alerts", isOn: $settings.notifyBudget)
            Toggle("Daily summary", isOn: $settings.notifyDailySummary)
            Toggle("Spend milestones", isOn: $settings.notifyMilestones)

            Divider()
            Button("Burnt Wrapped…", action: onShowWrapped).buttonStyle(.borderless)
```

- [ ] **Step 3: AppModel — Notifier wiring + Wrapped presentation.** Add to `AppModel`:

```swift
    @Published var showingWrapped = false

    private let notifier = NotificationService()
    private var notifierState = NotifierState()   // (could persist; in-memory dedup is fine within a run)
```
After the engine load sets `self.result` in `load()`, evaluate notifications on the main actor. Replace the `MainActor.run { ... }` body with one that also runs the notifier:

```swift
            await MainActor.run {
                self.result = r
                self.isLoading = false
                self.runNotifications()
            }
```
Add the method (uses the current Summary + settings):

```swift
    private func runNotifications() {
        guard case let .success(s) = result else {
            if case let .stale(s, _) = result { fire(for: s) }; return
        }
        fire(for: s)
    }

    private func fire(for s: Summary) {
        let opts = NotifierOptions(budgetAlerts: settings.notifyBudget,
                                   dailySummary: settings.notifyDailySummary,
                                   milestones: settings.notifyMilestones)
        guard opts.budgetAlerts || opts.dailySummary || opts.milestones else { return }
        // yesterday = second-to-last heatmap day; top model = byModel.first
        let yesterday = s.heatmapDays.dropLast().last
        let input = NotifierInput(todayCost: s.today.cost, monthCost: s.monthToDate.cost,
                                  dailyBudget: settings.dailyBudget,
                                  yesterdayCost: yesterday?.cost ?? 0,
                                  yesterdayTopModel: s.byModel.first?.modelName ?? "")
        let cal = Calendar(identifier: .gregorian)
        let c = cal.dateComponents([.year,.month,.day], from: s.generatedAt)
        let dayKey = String(format: "%04d-%02d-%02d", c.year!, c.month!, c.day!)
        let monthKey = String(format: "%04d-%02d", c.year!, c.month!)
        let pending = Notifier.evaluate(input: input, options: opts,
                                        dayKey: dayKey, monthKey: monthKey, state: &notifierState)
        for n in pending { notifier.post(title: n.title, body: n.body, id: n.id) }
    }

    /// Builds the Wrapped card. `allTime == false` → this month; `true` → all-time.
    func wrappedData(allTime: Bool = false) -> WrappedData? {
        let s: Summary
        switch result { case .success(let x), .stale(let x, _): s = x; default: return nil }
        let busiest = s.heatmapDays.max { $0.cost < $1.cost }
        let claudeCost = s.byTool.first { $0.tool == .claude }?.cost ?? 0
        let totalToolCost = s.byTool.reduce(0) { $0 + $1.cost }
        let totals = allTime ? s.allTime : s.monthToDate
        return WrappedData(
            title: allTime ? "All-Time" : "This Month",
            totalCost: totals.cost, totalTokens: totals.totalTokens,
            models: s.byModel.map { ($0.modelName, $0.cost) },
            busiestDay: busiest.map { prettyDate($0.date) } ?? "—",
            busiestDayCost: busiest?.cost ?? 0,
            claudeShare: totalToolCost > 0 ? claudeCost / totalToolCost : 0,
            cacheSaved: s.cacheSavings)
    }

    private func prettyDate(_ iso: String) -> String {
        let p = iso.split(separator: "-").compactMap { Int($0) }; guard p.count == 3 else { return iso }
        let m = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
        return "\(p[1] >= 1 && p[1] <= 12 ? m[p[1]-1] : "?") \(p[2])"
    }
```
Add `import BurntCore` if not already present (it is). The `Summary`/`NotifierInput` etc. are available via `import UsageEngine` + `import BurntCore`.

- [ ] **Step 4: Present Wrapped from the root view.** In `MenuBarRootView.swift`, pass `onShowWrapped` into `SettingsView` and add a sheet. Where `SettingsView(settings:onBack:)` is constructed, change to:

```swift
                SettingsView(settings: model.settings, onBack: { showingSettings = false },
                             onShowWrapped: { model.showingWrapped = true })
```
And add a `.sheet` modifier on the root `Group` (after `.frame(width: 300)`):

```swift
        .sheet(isPresented: $model.showingWrapped) {
            WrappedSheet(model: model) { model.showingWrapped = false }
        }
```
Add a small `WrappedSheet` wrapper (in `WrappedView.swift`) that adds Copy/Save buttons + the ImageRenderer export:

```swift
struct WrappedSheet: View {
    @ObservedObject var model: AppModel
    let onClose: () -> Void
    @State private var allTime = false

    private var data: WrappedData? { model.wrappedData(allTime: allTime) }

    var body: some View {
        VStack(spacing: 14) {
            Picker("", selection: $allTime) {
                Text("This Month").tag(false)
                Text("All-Time").tag(true)
            }.pickerStyle(.segmented).frame(width: 240)

            if let data {
                WrappedView(data: data)
                HStack {
                    Button("Copy Image") { export(data, toClipboard: true) }
                    Button("Save PNG…") { export(data, toClipboard: false) }
                    Spacer()
                    Button("Close", action: onClose)
                }.padding(.horizontal)
            } else {
                Text("No data yet").foregroundStyle(.secondary)
                Button("Close", action: onClose)
            }
        }.padding()
    }

    @MainActor private func render(_ data: WrappedData) -> NSImage? {
        let r = ImageRenderer(content: WrappedView(data: data))
        r.scale = 2
        return r.nsImage
    }

    @MainActor private func export(_ data: WrappedData, toClipboard: Bool) {
        guard let img = render(data), let tiff = img.tiffRepresentation,
              let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:]) else { return }
        if toClipboard {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setData(png, forType: .png)
        } else {
            let panel = NSSavePanel(); panel.nameFieldStringValue = "burnt-wrapped.png"
            panel.allowedContentTypes = [.png]
            if panel.runModal() == .OK, let url = panel.url { try? png.write(to: url) }
        }
    }
}
```
Add `import AppKit` to `WrappedView.swift` if needed (SwiftUI on macOS usually brings it, but `NSPasteboard`/`NSSavePanel` need it explicitly).

- [ ] **Step 5: Full build + all tests**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift build && DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`
Expected: `Build complete!`, all tests pass.

- [ ] **Step 6: Build the app bundle + launch + verify**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer ./packaging/make-app.sh && open Burnt.app`
Expected: switch dashboard style to **Detailed** → heatmap grid + "By project" section appear; Settings shows the 3 notification toggles + "Burnt Wrapped…"; clicking Wrapped opens the card with Copy/Save; enabling a budget + notifications posts a macOS notification when thresholds cross.

- [ ] **Step 7: Commit**

```bash
git add Sources/Burnt/SummaryView.swift Sources/Burnt/SettingsView.swift Sources/Burnt/AppModel.swift Sources/Burnt/MenuBarRootView.swift Sources/Burnt/WrappedView.swift
git commit -m "wire Phase 1: heatmap + by-project (Detailed), notification toggles, Burnt Wrapped sheet"
```

---

## Final Verification

- [ ] `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test` → all pass (engine + BurntTests).
- [ ] Detailed dashboard shows: 14-day sparkline, 12-week heatmap (hover works), By tool, By model, By project, cache line.
- [ ] Settings: 3 notification toggles (off by default) + "Burnt Wrapped…" button; Quit still present.
- [ ] Burnt Wrapped sheet renders the card; the month/all-time toggle switches variants; Copy Image puts a PNG on the clipboard; Save PNG writes a file.
- [ ] With a budget set + budget alerts on, crossing 80%/100% posts one macOS notification each.
- [ ] By-project numbers reconcile (sum ≈ total); unmatched sessions appear under "Unknown".

---

## Self-Review

**Spec coverage:**
- §3 by-project (ccusage session ⨝ cwd map, leaf grouping, Unknown bucket, dedup-by-fullpath) → Tasks 2,3,4,5 ✓
- §4 notifications (3 kinds, opt-in, deduped, protocol-posted) → Tasks 6,8,9,10 ✓
- §5 heatmap (84-day series, Detailed only, hover) → Tasks 1,9,10 ✓
- §6 Wrapped (month variant, model split, busiest day, Copy/Save via ImageRenderer) → Tasks 7,9,10 ✓
- §7 error handling (by-project never blocks summary; perms-denied no-crash; Unknown bucket) → Tasks 5 (`try?`), 9 (auth), 4 ✓
- §8 tests (ProjectAttributor join, Notifier dedup/thresholds, WrappedData, heatmap series) → Tasks 4,6,7,1 ✓

**Placeholder scan:** none. The `Image("AppIconImage")` line in Task 9 Step 3 is explicitly called out to be removed (note under the code) — not a lingering placeholder. Both Wrapped variants (month + all-time) ship: `wrappedData(allTime:)` builds either, and `WrappedSheet` has a segmented month/all-time picker — matches spec §6.

**Type consistency:** `ProjectSlice` (Task 4) used in Summary (5), SummaryView (10). `SessionRow`/`SessionReport` (Task 2) used in CcusageRunner (3), ProjectAttributor tests (4), UsageEngine (5). `Notifier`/`NotifierInput`/`NotifierOptions`/`NotifierState`/`PendingNotification`/`NotificationPosting` (Task 6) used in NotificationService (9) + AppModel (10). `WrappedData` (Task 7) used in WrappedView (9) + AppModel (10). `heatmapDays` (Task 1) used in HeatmapView (9), AppModel yesterday/busiest (10), smoke test (5). `Formatters.cost/tokens` reused. `BreakdownBar` (existing) reused for by-project rows with `.secondary` color.

**One scope flag for the user:** the spec's Wrapped calls for **month + all-time** variants; this plan fully wires the **month** card and notes all-time as a one-toggle follow-up (Task 10 self-review). Confirm whether all-time must ship in this pass.
