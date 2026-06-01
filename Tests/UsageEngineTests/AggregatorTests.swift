import XCTest
@testable import UsageEngine

final class AggregatorTests: XCTestCase {
    private func report(_ days: [DailyUsage]) -> CcusageReport {
        CcusageReport(daily: days, totals: .init(inputTokens: 0, outputTokens: 0,
            cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0, totalCost: 0))
    }

    private func day(_ period: String, cost: Double, models: [ModelBreakdown]) -> DailyUsage {
        DailyUsage(period: period, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0,
            cacheReadTokens: 0, totalTokens: 0, totalCost: cost, modelBreakdowns: models, metadata: nil)
    }

    private func mb(_ name: String, cost: Double, total: Int, cacheRead: Int = 0) -> ModelBreakdown {
        ModelBreakdown(modelName: name, inputTokens: 0, outputTokens: 0,
            cacheCreationTokens: 0, cacheReadTokens: cacheRead, cost: cost)
    }

    // referenceDate = 2026-06-08
    private let ref = ISO8601DateFormatter.dateOnly("2026-06-08")

    func testTodayPicksMatchingDateOnly() {
        let r = report([
            day("2026-06-08", cost: 5, models: [mb("claude-opus-4-8", cost: 5, total: 100)]),
            day("2026-06-07", cost: 9, models: [mb("claude-opus-4-8", cost: 9, total: 200)]),
        ])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.today.cost, 5, accuracy: 0.001)
    }

    func testWeekIsRolling7DaysInclusive() {
        // 2026-06-02 is exactly 6 days before 06-08 → included. 06-01 → excluded.
        let r = report([
            day("2026-06-08", cost: 1, models: [mb("claude-opus-4-8", cost: 1, total: 10)]),
            day("2026-06-02", cost: 2, models: [mb("claude-opus-4-8", cost: 2, total: 10)]),
            day("2026-06-01", cost: 4, models: [mb("claude-opus-4-8", cost: 4, total: 10)]),
        ])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.thisWeek.cost, 3, accuracy: 0.001)   // 1 + 2, not 4
    }

    func testByToolSplitsClaudeAndCodex() {
        let r = report([
            day("2026-06-08", cost: 7, models: [
                mb("claude-opus-4-8", cost: 5, total: 100),
                mb("gpt-5.4", cost: 2, total: 50),
            ]),
        ])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        let claude = s.byTool.first { $0.tool == .claude }!
        let codex = s.byTool.first { $0.tool == .codex }!
        XCTAssertEqual(claude.cost, 5, accuracy: 0.001)
        XCTAssertEqual(codex.cost, 2, accuracy: 0.001)
    }

    func testWeekByDayIsZeroFilledSevenPoints() {
        let r = report([day("2026-06-08", cost: 1, models: [mb("claude-opus-4-8", cost: 1, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.weekByDay.count, 7)
        XCTAssertEqual(s.weekByDay.last?.date, "2026-06-08")
        XCTAssertEqual(s.weekByDay.first?.date, "2026-06-02")
        XCTAssertEqual(s.weekByDay.first?.cost, 0)            // zero-filled gap day
    }

    func testCacheSavingsAggregatesClaudeOnly() {
        let r = report([
            day("2026-06-08", cost: 1, models: [
                mb("claude-opus-4-8", cost: 1, total: 10, cacheRead: 1_000_000),
                mb("gpt-5.4", cost: 1, total: 10, cacheRead: 1_000_000),
            ]),
        ])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.cacheSavings, 13.50, accuracy: 0.01) // claude only
    }
}

extension ISO8601DateFormatter {
    static func dateOnly(_ s: String) -> Date {
        var c = DateComponents()
        let parts = s.split(separator: "-").map { Int($0)! }
        c.year = parts[0]; c.month = parts[1]; c.day = parts[2]
        return Calendar(identifier: .gregorian).date(from: c)!
    }
}
