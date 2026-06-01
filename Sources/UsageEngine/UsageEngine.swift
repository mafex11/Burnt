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
    private var lastGood: Summary?

    public init(locator: CcusageLocator = CcusageLocator(), now: @escaping @Sendable () -> Date = { Date() }) {
        self.locator = locator
        self.now = now
    }

    /// Synchronous load — callers run it off the main thread.
    /// - Parameter offline: true for the cheap 60s background poll (cached pricing,
    ///   no network); false for human-triggered refreshes (live LiteLLM pricing).
    public func loadSummary(offline: Bool) -> EngineResult {
        guard case let .ready(invocation) = locator.resolve() else {
            return .unavailable
        }
        let runner = CcusageRunner(invocation: invocation)
        do {
            let report = try runner.fetchDailyReport(offline: offline)
            if report.daily.isEmpty && lastGood == nil {
                return .noData
            }
            let summary = Aggregator.summary(from: report, referenceDate: now())
            lastGood = summary
            return .success(summary)
        } catch {
            if let cached = lastGood {
                return .stale(cached, reason: String(describing: error))
            }
            return .noData
        }
    }
}
