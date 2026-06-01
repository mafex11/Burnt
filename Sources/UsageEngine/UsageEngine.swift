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
    public func loadSummary() -> EngineResult {
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

            let summary = Aggregator.summary(from: report, referenceDate: now())
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
}
