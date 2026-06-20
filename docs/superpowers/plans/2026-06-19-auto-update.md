# Auto-Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Burnt keep itself current by checking its Homebrew tap once a day and (when enabled) shelling out to `brew upgrade --cask burnt`, with a Settings toggle and a manual "Check for Updates" button.

**Architecture:** A pure `UpdateChecker` (in `UsageEngine`) fetches the raw cask file from the tap and compares semantic versions. A `BrewUpdater` (in `BurntCore`) detects whether the app is brew-managed and shells out to `brew`. `AppModel` wires them on a daily wall-clock cadence and exposes state to `SettingsView`. Homebrew remains the single source of truth — Burnt never rewrites its own bytes.

**Tech Stack:** Swift 6, Swift Package Manager, Foundation `URLSession`/`Process`, SwiftUI, XCTest. Build/test require `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`.

## Global Constraints

- Swift tools 6.0, macOS 14+ (`Package.swift`).
- Every `swift build` / `swift test` command MUST be prefixed with `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer` or XCTest is unavailable.
- All git operations run INSIDE `/Users/mafex/code/personal/burnt` (its own repo), never the parent monorepo.
- Commit messages: NO Claude attribution, NO `Co-Authored-By` trailer, NO AI footer.
- Target placement: pure + tested logic → `UsageEngine` (tested by `UsageEngineTests`). App-logic + shell-out → `BurntCore` (tested by `BurntTests`). SwiftUI/`AppModel` glue → `Burnt` executable (not unit-tested).
- Tap version source URL (verbatim): `https://raw.githubusercontent.com/mafex11/homebrew-tap/main/Casks/burnt.rb`.
- Cask token is `burnt`; brew lives at `/opt/homebrew/bin/brew` (Apple Silicon) or `/usr/local/bin/brew` (Intel).
- Settings default: `autoUpdate` defaults to **true** when unset (mirror the `animateFlame` `object(forKey:) == nil ? true : bool` idiom in `Sources/BurntCore/Settings.swift`).

---

### Task 1: `UpdateStatus` + version comparison (pure, UsageEngine)

The smallest testable core: comparing two semantic version strings. No I/O.

**Files:**
- Create: `Sources/UsageEngine/UpdateChecker.swift`
- Test: `Tests/UsageEngineTests/UpdateCheckerTests.swift`

**Interfaces:**
- Produces:
  - `public enum UpdateStatus: Sendable, Equatable { case upToDate; case updateAvailable(String) }`
  - `public enum UpdateChecker` with `public static func compare(current: String, latest: String) -> UpdateStatus`
  - Semantics: split each version on `.`, pad missing components with 0, compare numerically component-by-component. `.updateAvailable(latest)` only when `latest > current`; equal or current-ahead → `.upToDate`. Non-numeric/garbage components parse as 0.

- [ ] **Step 1: Write the failing tests**

```swift
// Tests/UsageEngineTests/UpdateCheckerTests.swift
import XCTest
@testable import UsageEngine

final class UpdateCheckerTests: XCTestCase {
    func testEqualIsUpToDate() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.1", latest: "1.2.1"), .upToDate)
    }
    func testPatchBumpAvailable() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.1", latest: "1.2.2"), .updateAvailable("1.2.2"))
    }
    func testMinorAndMajorBumpAvailable() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.9", latest: "1.3.0"), .updateAvailable("1.3.0"))
        XCTAssertEqual(UpdateChecker.compare(current: "1.9.9", latest: "2.0.0"), .updateAvailable("2.0.0"))
    }
    func testCurrentAheadIsUpToDate() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.3.0", latest: "1.2.9"), .upToDate)
    }
    func testDoubleDigitComponentsCompareNumerically() {
        // string compare would call "1.2.10" < "1.2.9"; numeric must not.
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.9", latest: "1.2.10"), .updateAvailable("1.2.10"))
    }
    func testUnevenComponentCountPadsWithZero() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2", latest: "1.2.0"), .upToDate)
        XCTAssertEqual(UpdateChecker.compare(current: "1.2", latest: "1.2.1"), .updateAvailable("1.2.1"))
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter UpdateCheckerTests`
Expected: FAIL — `cannot find 'UpdateChecker' in scope`.

- [ ] **Step 3: Write minimal implementation**

```swift
// Sources/UsageEngine/UpdateChecker.swift
import Foundation

public enum UpdateStatus: Sendable, Equatable {
    case upToDate
    case updateAvailable(String)   // the newer version string
}

public enum UpdateChecker {
    /// Numeric, component-wise semantic version comparison. Missing components are
    /// treated as 0; non-numeric components parse as 0. `.updateAvailable` only when
    /// `latest` is strictly greater than `current`.
    public static func compare(current: String, latest: String) -> UpdateStatus {
        let c = parts(current), l = parts(latest)
        let n = max(c.count, l.count)
        for i in 0..<n {
            let a = i < c.count ? c[i] : 0
            let b = i < l.count ? l[i] : 0
            if b > a { return .updateAvailable(latest) }
            if b < a { return .upToDate }
        }
        return .upToDate
    }

    private static func parts(_ v: String) -> [Int] {
        v.split(separator: ".").map { Int($0) ?? 0 }
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter UpdateCheckerTests`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
cd /Users/mafex/code/personal/burnt
git add Sources/UsageEngine/UpdateChecker.swift Tests/UsageEngineTests/UpdateCheckerTests.swift
git commit -m "Add semantic version comparison for update checks"
```

---

### Task 2: Parse the cask `version` + fetch latest (UsageEngine)

Parse `version "X.Y.Z"` out of cask text, and wrap it with an injectable network fetch so it stays unit-testable.

**Files:**
- Modify: `Sources/UsageEngine/UpdateChecker.swift`
- Test: `Tests/UsageEngineTests/UpdateCheckerTests.swift`

**Interfaces:**
- Consumes: `UpdateChecker.compare` (Task 1).
- Produces:
  - `public static func parseVersion(fromCask text: String) -> String?` — returns the first `version "X.Y.Z"` match, else nil.
  - `public static let caskURL = URL(string: "https://raw.githubusercontent.com/mafex11/homebrew-tap/main/Casks/burnt.rb")!`
  - `public static func latestVersion(fetch: (URL) throws -> Data) throws -> String` — fetches `caskURL`, parses; throws `UpdateError.unparseable` if no version found.
  - `public enum UpdateError: Error, Equatable { case unparseable }`

- [ ] **Step 1: Write the failing tests**

```swift
// append to Tests/UsageEngineTests/UpdateCheckerTests.swift (inside the class)
func testParsesVersionFromCaskText() {
    let cask = """
    cask "burnt" do
      version "1.2.2"
      sha256 "abc123"
    end
    """
    XCTAssertEqual(UpdateChecker.parseVersion(fromCask: cask), "1.2.2")
}
func testParseReturnsNilWhenNoVersion() {
    XCTAssertNil(UpdateChecker.parseVersion(fromCask: "cask \"burnt\" do\nend"))
}
func testLatestVersionUsesInjectedFetch() throws {
    let cask = "cask \"burnt\" do\n  version \"3.4.5\"\nend"
    let v = try UpdateChecker.latestVersion { _ in Data(cask.utf8) }
    XCTAssertEqual(v, "3.4.5")
}
func testLatestVersionThrowsOnUnparseable() {
    XCTAssertThrowsError(try UpdateChecker.latestVersion { _ in Data("garbage".utf8) }) { err in
        XCTAssertEqual(err as? UpdateChecker.UpdateError, .unparseable)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter UpdateCheckerTests`
Expected: FAIL — `parseVersion` / `latestVersion` / `UpdateError` not found.

- [ ] **Step 3: Write minimal implementation**

Add to `UpdateChecker` in `Sources/UsageEngine/UpdateChecker.swift`:

```swift
    public enum UpdateError: Error, Equatable { case unparseable }

    public static let caskURL = URL(string:
        "https://raw.githubusercontent.com/mafex11/homebrew-tap/main/Casks/burnt.rb")!

    /// First `version "X.Y.Z"` value in cask text, else nil.
    public static func parseVersion(fromCask text: String) -> String? {
        // matches: version "1.2.3"
        guard let r = text.range(of: #"version\s+"([0-9]+(?:\.[0-9]+)*)""#,
                                 options: .regularExpression) else { return nil }
        let match = String(text[r])
        guard let q = match.range(of: #"[0-9]+(?:\.[0-9]+)*"#, options: .regularExpression)
        else { return nil }
        return String(match[q])
    }

    /// Fetch the tap cask and parse its version. `fetch` is injected for tests.
    public static func latestVersion(fetch: (URL) throws -> Data = { try Data(contentsOf: $0) }) throws -> String {
        let data = try fetch(caskURL)
        guard let v = parseVersion(fromCask: String(decoding: data, as: UTF8.self)) else {
            throw UpdateError.unparseable
        }
        return v
    }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter UpdateCheckerTests`
Expected: PASS (10 tests total).

- [ ] **Step 5: Commit**

```bash
cd /Users/mafex/code/personal/burnt
git add Sources/UsageEngine/UpdateChecker.swift Tests/UsageEngineTests/UpdateCheckerTests.swift
git commit -m "Parse cask version and fetch latest from tap"
```

---

### Task 3: `BrewUpdater` — brew-managed detection + upgrade shell-out (BurntCore)

Detect whether this install is brew-managed (so direct-download users degrade gracefully) and run the upgrade. Detection is pure-ish (filesystem check against an injectable path) and unit-tested; the actual `brew` shell-out is integration-verified by hand.

**Files:**
- Create: `Sources/BurntCore/BrewUpdater.swift`
- Test: `Tests/BurntTests/BrewUpdaterTests.swift`

**Interfaces:**
- Produces:
  - `public struct BrewUpdater: Sendable`
  - `public init(caskroomReceipt: URL? = BrewUpdater.defaultReceipt, brewCandidates: [String] = ["/opt/homebrew/bin/brew", "/usr/local/bin/brew"])`
  - `public func isBrewManaged() -> Bool` — true iff `caskroomReceipt` exists on disk.
  - `public func brewPath() -> String?` — first existing executable among `brewCandidates`.
  - `public func upgrade()` — if `brewPath()` resolves, launches `brew update && brew upgrade --cask burnt` detached; no-op otherwise. (Not unit-tested — mutates the real install.)
  - `public static let defaultReceipt: URL? = URL(fileURLWithPath: "/opt/homebrew/Caskroom/burnt/.metadata/INSTALL_RECEIPT.json")`

- [ ] **Step 1: Write the failing tests**

```swift
// Tests/BurntTests/BrewUpdaterTests.swift
import XCTest
@testable import BurntCore

final class BrewUpdaterTests: XCTestCase {
    func testBrewManagedTrueWhenReceiptExists() throws {
        let fm = FileManager.default
        let tmp = fm.temporaryDirectory.appendingPathComponent("brew-\(UUID().uuidString)")
        try fm.createDirectory(at: tmp, withIntermediateDirectories: true)
        let receipt = tmp.appendingPathComponent("INSTALL_RECEIPT.json")
        try "{}".write(to: receipt, atomically: true, encoding: .utf8)
        let u = BrewUpdater(caskroomReceipt: receipt)
        XCTAssertTrue(u.isBrewManaged())
        try? fm.removeItem(at: tmp)
    }
    func testBrewManagedFalseWhenReceiptMissing() {
        let missing = FileManager.default.temporaryDirectory
            .appendingPathComponent("nope-\(UUID().uuidString).json")
        XCTAssertFalse(BrewUpdater(caskroomReceipt: missing).isBrewManaged())
    }
    func testBrewManagedFalseWhenReceiptNil() {
        XCTAssertFalse(BrewUpdater(caskroomReceipt: nil).isBrewManaged())
    }
    func testBrewPathResolvesFirstExistingCandidate() throws {
        let fm = FileManager.default
        let tmp = fm.temporaryDirectory.appendingPathComponent("brewbin-\(UUID().uuidString)")
        try fm.createDirectory(at: tmp, withIntermediateDirectories: true)
        let fake = tmp.appendingPathComponent("brew")
        try "#!/bin/sh\n".write(to: fake, atomically: true, encoding: .utf8)
        try fm.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fake.path)
        let u = BrewUpdater(caskroomReceipt: nil,
                            brewCandidates: ["/no/such/brew", fake.path])
        XCTAssertEqual(u.brewPath(), fake.path)
        try? fm.removeItem(at: tmp)
    }
    func testBrewPathNilWhenNoCandidateExists() {
        let u = BrewUpdater(caskroomReceipt: nil, brewCandidates: ["/no/such/brew"])
        XCTAssertNil(u.brewPath())
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter BrewUpdaterTests`
Expected: FAIL — `cannot find 'BrewUpdater' in scope`.

- [ ] **Step 3: Write minimal implementation**

```swift
// Sources/BurntCore/BrewUpdater.swift
import Foundation

/// Drives updates through Homebrew so brew stays the single source of truth — Burnt
/// never rewrites its own bytes, it asks `brew` to replace the cask. Detection guards
/// against running brew for users who installed by direct download.
public struct BrewUpdater: Sendable {
    private let caskroomReceipt: URL?
    private let brewCandidates: [String]

    public static let defaultReceipt: URL? =
        URL(fileURLWithPath: "/opt/homebrew/Caskroom/burnt/.metadata/INSTALL_RECEIPT.json")

    public init(caskroomReceipt: URL? = BrewUpdater.defaultReceipt,
                brewCandidates: [String] = ["/opt/homebrew/bin/brew", "/usr/local/bin/brew"]) {
        self.caskroomReceipt = caskroomReceipt
        self.brewCandidates = brewCandidates
    }

    /// True iff this install lives under a Homebrew Caskroom (receipt present).
    public func isBrewManaged() -> Bool {
        guard let r = caskroomReceipt else { return false }
        return FileManager.default.fileExists(atPath: r.path)
    }

    /// First existing brew executable, else nil.
    public func brewPath() -> String? {
        brewCandidates.first { FileManager.default.isExecutableFile(atPath: $0) }
    }

    /// Launch `brew update && brew upgrade --cask burnt` detached. No-op if brew is
    /// absent. brew's cask postflight handles quarantine-strip + relaunch.
    public func upgrade() {
        guard let brew = brewPath() else { return }
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/sh")
        p.arguments = ["-c", "\(brew) update && \(brew) upgrade --cask burnt"]
        try? p.run()
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter BrewUpdaterTests`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
cd /Users/mafex/code/personal/burnt
git add Sources/BurntCore/BrewUpdater.swift Tests/BurntTests/BrewUpdaterTests.swift
git commit -m "Add BrewUpdater: brew-managed detection and upgrade shell-out"
```

---

### Task 4: `Settings.autoUpdate` toggle (BurntCore)

Persisted preference, default ON, mirroring the existing `animateFlame` pattern.

**Files:**
- Modify: `Sources/BurntCore/Settings.swift` (add `Key.autoUpdate`, the `@Published` property, and init default)
- Test: `Tests/BurntTests/SettingsTests.swift`

**Interfaces:**
- Produces: `Settings.autoUpdate: Bool` (`@Published`, persisted under key `"autoUpdate"`, default true when unset).

- [ ] **Step 1: Write the failing tests**

```swift
// append to Tests/BurntTests/SettingsTests.swift (inside the existing SettingsTests class).
// Reuse the file's existing `freshDefaults()` helper and `StubLoginItem` double.
func testAutoUpdateDefaultsOnWhenUnset() {
    let s = Settings(defaults: freshDefaults(), loginItem: StubLoginItem())
    XCTAssertTrue(s.autoUpdate)
}
func testAutoUpdatePersists() {
    let d = freshDefaults()
    let s1 = Settings(defaults: d, loginItem: StubLoginItem())
    s1.autoUpdate = false
    let s2 = Settings(defaults: d, loginItem: StubLoginItem())
    XCTAssertFalse(s2.autoUpdate)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter SettingsTests`
Expected: FAIL — `value of type 'Settings' has no member 'autoUpdate'`.

- [ ] **Step 3: Write minimal implementation**

In `Sources/BurntCore/Settings.swift`, add to the `Key` enum:

```swift
        static let autoUpdate = "autoUpdate"
```

In `init`, after the `animateFlame` block, add:

```swift
        // Keep Burnt current via brew; default ON when unset.
        let auto = defaults.object(forKey: Key.autoUpdate) == nil ? true : defaults.bool(forKey: Key.autoUpdate)
        self._autoUpdate = Published(initialValue: auto)
```

Add the property alongside the other `@Published` declarations:

```swift
    @Published public var autoUpdate: Bool {
        didSet { defaults.set(autoUpdate, forKey: Key.autoUpdate) }
    }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test --filter SettingsTests`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/mafex/code/personal/burnt
git add Sources/BurntCore/Settings.swift Tests/BurntTests/SettingsTests.swift
git commit -m "Add autoUpdate setting (default on)"
```

---

### Task 5: Wire update flow into `AppModel` (Burnt executable)

Daily wall-clock check on launch + a shared `checkForUpdates` method the timer and button both call. Not unit-tested (UI glue); verified by build + manual run.

**Files:**
- Modify: `Sources/Burnt/AppModel.swift`

**Interfaces:**
- Consumes: `UpdateChecker.latestVersion` + `.compare` + `currentVersion`; `BrewUpdater`; `Settings.autoUpdate`.
- Produces (on `AppModel`, observed by views):
  - `enum UpdateUIState: Equatable { case idle, checking, upToDate, available(String), updating }`
  - `@Published var updateState: UpdateUIState`
  - `func checkForUpdates(userInitiated: Bool)`
  - Called once from `startAutoRefresh()` (launch) and on a 24h wall-clock cadence.

- [ ] **Step 1: Add the update plumbing to `AppModel`**

Add stored properties near the other privates (after `notifierState`):

```swift
    @Published var updateState: UpdateUIState = .idle
    private let brew = BrewUpdater()
    private var updateTimer: Timer?
    private let lastCheckKey = "lastUpdateCheck"

    enum UpdateUIState: Equatable { case idle, checking, upToDate, available(String), updating }

    /// App version from the bundle (e.g. "1.2.1"); "0" if unreadable.
    private var currentVersion: String {
        (Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String) ?? "0"
    }
```

- [ ] **Step 2: Add `checkForUpdates` and the daily scheduler**

```swift
    /// Shared by the daily timer and the Settings button. Fetches the tap's latest
    /// version off-main, compares, and (when auto-update is on and the app is
    /// brew-managed) applies via brew. Best-effort: any failure degrades to idle.
    func checkForUpdates(userInitiated: Bool) {
        if userInitiated { updateState = .checking }
        let current = currentVersion
        let autoOn = settings.autoUpdate
        let brew = self.brew
        Task.detached {
            let status: UpdateStatus
            do {
                let latest = try UpdateChecker.latestVersion()
                status = UpdateChecker.compare(current: current, latest: latest)
            } catch {
                await MainActor.run { if userInitiated { self.updateState = .idle } }
                return
            }
            await MainActor.run {
                UserDefaults.standard.set(Date(), forKey: self.lastCheckKey)
                switch status {
                case .upToDate:
                    self.updateState = .upToDate
                case .updateAvailable(let v):
                    if autoOn && brew.isBrewManaged() {
                        self.updateState = .updating
                        brew.upgrade()
                    } else {
                        self.updateState = .available(v)
                    }
                }
            }
        }
    }

    /// Check on launch if a day has passed (or never checked), then schedule a daily
    /// wall-clock check. Stored last-check date means a Mac that slept overnight still
    /// checks promptly after wake rather than drifting on pure uptime.
    func startUpdateChecks() {
        let last = UserDefaults.standard.object(forKey: lastCheckKey) as? Date
        if last == nil || Date().timeIntervalSince(last!) >= 86_400 {
            checkForUpdates(userInitiated: false)
        }
        updateTimer?.invalidate()
        updateTimer = Timer.scheduledTimer(withTimeInterval: 86_400, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.checkForUpdates(userInitiated: false) }
        }
    }
```

- [ ] **Step 3: Call `startUpdateChecks()` from `startAutoRefresh()`**

In `startAutoRefresh()`, add after `startFlameAnimation()`:

```swift
        startUpdateChecks()
```

- [ ] **Step 4: Build to verify it compiles**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift build`
Expected: `Build complete!`

- [ ] **Step 5: Commit**

```bash
cd /Users/mafex/code/personal/burnt
git add Sources/Burnt/AppModel.swift
git commit -m "Wire daily update check and brew-upgrade flow into AppModel"
```

---

### Task 6: Settings UI — toggle + "Check for Updates" button (Burnt executable)

Surface the toggle and a manual check button with inline status.

**Files:**
- Modify: `Sources/Burnt/SettingsView.swift`
- Modify: `Sources/Burnt/MenuBarRootView.swift` (pass `model` to `SettingsView` so the button can call `checkForUpdates`)

**Interfaces:**
- Consumes: `Settings.autoUpdate`, `AppModel.updateState`, `AppModel.checkForUpdates(userInitiated:)`.

- [ ] **Step 1: Give `SettingsView` access to the model's update state**

In `Sources/Burnt/SettingsView.swift`, add a model reference and a status-string helper. Add near the top of the struct (after `@ObservedObject var settings`):

```swift
    @ObservedObject var model: AppModel
```

Add this computed helper inside the struct (before `body`):

```swift
    private var updateStatusText: String {
        switch model.updateState {
        case .idle:             return ""
        case .checking:         return "Checking…"
        case .upToDate:         return "You're up to date"
        case .available(let v): return "Update available — v\(v)"
        case .updating:         return "Updating…"
        }
    }
```

- [ ] **Step 2: Add the toggle + button section to the body**

In `Sources/Burnt/SettingsView.swift`, insert this block right before the `Divider()` that precedes `Button("Burnt Wrapped…", …)`:

```swift
            Divider()
            Text("Updates").font(.caption).foregroundStyle(.secondary)
            Toggle("Automatically update Burnt", isOn: $settings.autoUpdate)
            HStack {
                Button("Check for Updates") { model.checkForUpdates(userInitiated: true) }
                    .buttonStyle(.borderless)
                Spacer()
                Text(updateStatusText).font(.caption).foregroundStyle(.secondary)
            }
```

- [ ] **Step 3: Pass `model` when constructing `SettingsView`**

In `Sources/Burnt/MenuBarRootView.swift`, update the `SettingsView(...)` initializer call to include `model: model`:

```swift
                SettingsView(settings: model.settings, model: model,
                             onBack: { showingSettings = false },
                             onShowWrapped: { model.showingWrapped = true })
```

- [ ] **Step 4: Build to verify it compiles**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift build`
Expected: `Build complete!`

- [ ] **Step 5: Run the full test suite (no regressions)**

Run: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test`
Expected: all tests PASS (existing 53 + new from Tasks 1–4).

- [ ] **Step 6: Commit**

```bash
cd /Users/mafex/code/personal/burnt
git add Sources/Burnt/SettingsView.swift Sources/Burnt/MenuBarRootView.swift
git commit -m "Add Updates settings section: auto-update toggle and manual check"
```

---

### Task 7: Manual integration verification (no code)

The real `brew upgrade` path can't be unit-tested (it mutates the install). Verify by hand.

**Files:** none.

- [ ] **Step 1: Confirm version detection against the live tap**

Run:
```bash
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift run Burnt &
# open the popover → Settings → "Check for Updates"
```
Expected: with installed app at the tap's current version, status reads "You're up to date". (To exercise the available path, the tester can temporarily point `caskURL` at a fixture or bump the tap; revert after.)

- [ ] **Step 2: Confirm `brewPath()`/`isBrewManaged()` resolve on this machine**

Run:
```bash
ls /opt/homebrew/bin/brew /opt/homebrew/Caskroom/burnt/.metadata/INSTALL_RECEIPT.json
```
Expected: both exist → on a real install, auto-apply would trigger.

- [ ] **Step 3: Note for release**

The end-to-end auto-apply (brew actually swapping the app) is only observable once a *newer* version is published to the tap. Validate it on the FIRST release after this ships: install the prior version, publish the new cask, confirm Burnt upgrades itself within the daily window / on manual check.

---

## Notes for the release (after plan completes)

This feature ships in the next release (e.g. v1.2.2): bump `packaging/Info.plist` (`CFBundleShortVersionString` + `CFBundleVersion`), rebuild, `make-release.sh`, push, GitHub release, update tap cask, `brew reinstall` to verify — the established pipeline. No cask changes are required for the feature itself (it reads the existing cask `version` line).
