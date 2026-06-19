import Foundation

public enum EngineResult: Sendable {
    case success(Summary)
    case stale(Summary, reason: String)   // last-good cache + why fresh fetch failed
    case unavailable                       // ccusage not found (unreachable in a real install)
    case noData
}

public final class UsageEngine: @unchecked Sendable {
    private let locator: CcusageLocator
    private let now: @Sendable () -> Date

    // `lastGood` is read/written from concurrent refresh tasks; guard every access
    // with this lock to avoid a data race.
    private let lock = NSLock()
    private var lastGood: Summary?

    public init(locator: CcusageLocator = CcusageLocator(), now: @escaping @Sendable () -> Date = { Date() }) {
        self.locator = locator
        self.now = now
    }

    /// Loads usage with live pricing. Synchronous — callers run it off the main thread.
    ///
    /// `includeProjects` controls the expensive per-project attribution: it runs a
    /// second `ccusage session` subprocess AND walks every Claude/Codex session log on
    /// disk. That data only feeds the Detailed-mode "By project" list, so the menu bar
    /// poll passes `false` and only pays for it when the popover is open in Detailed.
    /// When skipped, the previous build's projects are reused so the list doesn't flicker.
    public func loadSummary(includeProjects: Bool = true) -> EngineResult {
        guard case let .ready(invocation) = locator.resolve() else {
            return .unavailable
        }
        let runner = CcusageRunner(invocation: invocation)
        do {
            let report = try runner.fetchDailyReport()

            // An empty report means no usage. Only trust it for a genuine cold start
            // (no cached data). If we already have good data, a sudden empty result is
            // suspicious (transient read issue) — keep the cache as stale rather than
            // flashing $0.00.
            if report.daily.isEmpty {
                if let cached = cachedSummary() {
                    return .stale(cached, reason: "ccusage returned no data")
                }
                return .noData
            }

            // Only pay for project attribution when asked (Detailed popover). Otherwise
            // reuse the last build's projects so the list stays populated between fetches.
            let projects: [ProjectSlice]
            if includeProjects {
                projects = (try? Self.buildProjects(runner: runner)) ?? lastProjects()
            } else {
                projects = lastProjects()
            }
            let summary = Aggregator.summary(from: report, referenceDate: now(), byProject: projects)
            setCachedSummary(summary)
            return .success(summary)
        } catch {
            if let cached = cachedSummary() {
                return .stale(cached, reason: String(describing: error))
            }
            return .noData
        }
    }

    private func cachedSummary() -> Summary? {
        lock.lock(); defer { lock.unlock() }
        return lastGood
    }

    private func setCachedSummary(_ summary: Summary) {
        lock.lock(); defer { lock.unlock() }
        lastGood = summary
    }

    /// The previous build's projects, so a light (no-project) refresh keeps the
    /// "By project" list populated rather than briefly emptying it.
    private func lastProjects() -> [ProjectSlice] {
        lock.lock(); defer { lock.unlock() }
        return lastGood?.byProject ?? []
    }

    private static func buildProjects(runner: CcusageRunner) throws -> [ProjectSlice] {
        let sessions = try runner.fetchSessionReport().session
        let home = FileManager.default.homeDirectoryForCurrentUser
        let cwdMap = ProjectAttributor.buildCwdMap(
            claudeRoot: home.appendingPathComponent(".claude"),
            codexRoot: home.appendingPathComponent(".codex"))
        return ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
    }
}
