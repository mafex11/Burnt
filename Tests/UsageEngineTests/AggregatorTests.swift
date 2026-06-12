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

    func testWeekByDayIsZeroFilledFourteenPoints() {
        let r = report([day("2026-06-08", cost: 1, models: [mb("claude-opus-4-8", cost: 1, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.weekByDay.count, 14)                 // 14-day sparkline series
        XCTAssertEqual(s.weekByDay.last?.date, "2026-06-08")  // newest = today
        XCTAssertEqual(s.weekByDay.first?.date, "2026-05-26") // oldest = 13 days prior
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

    func testMonthToDateSumsCalendarMonth() {
        let r = report([
            day("2026-06-08", cost: 3, models: [mb("claude-opus-4-8", cost: 3, total: 10)]),
            day("2026-06-02", cost: 4, models: [mb("claude-opus-4-8", cost: 4, total: 10)]),
            day("2026-05-30", cost: 9, models: [mb("claude-opus-4-8", cost: 9, total: 10)]),
        ])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.monthToDate.cost, 7, accuracy: 0.001)
    }

    func testAllTimeFromReportTotals() {
        let r = CcusageReport(daily: [], totals: .init(inputTokens: 1, outputTokens: 2,
            cacheCreationTokens: 3, cacheReadTokens: 4, totalTokens: 10, totalCost: 99.5))
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.allTime.cost, 99.5, accuracy: 0.001)
        XCTAssertEqual(s.allTime.totalTokens, 10)
    }

    func testAvgPerDayIsWeekOverSeven() {
        let r = report([day("2026-06-08", cost: 7, models: [mb("claude-opus-4-8", cost: 7, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.avgPerDay, 1.0, accuracy: 0.001)
    }

    func testLastWeekWindowAndTrend() {
        let r = report([
            day("2026-06-08", cost: 2, models: [mb("claude-opus-4-8", cost: 2, total: 10)]),
            day("2026-06-01", cost: 1, models: [mb("claude-opus-4-8", cost: 1, total: 10)]),
        ])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.lastWeek.cost, 1, accuracy: 0.001)
        XCTAssertEqual(s.weekTrend ?? -999, 1.0, accuracy: 0.001)
    }

    func testWeekTrendNilWhenNoLastWeek() {
        let r = report([day("2026-06-08", cost: 2, models: [mb("claude-opus-4-8", cost: 2, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertNil(s.weekTrend)
    }

    func testProjectedTodayNilEarlyMorning() {
        let early = Calendar(identifier: .gregorian).date(bySettingHour: 0, minute: 30, second: 0, of: ref)!
        let r = report([day("2026-06-08", cost: 1, models: [mb("claude-opus-4-8", cost: 1, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: early)
        XCTAssertNil(s.projectedToday)
    }

    func testProjectedTodayExtrapolatesMidday() {
        let noon = Calendar(identifier: .gregorian).date(bySettingHour: 12, minute: 0, second: 0, of: ref)!
        let r = report([day("2026-06-08", cost: 5, models: [mb("claude-opus-4-8", cost: 5, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: noon)
        XCTAssertEqual(s.projectedToday ?? -1, 10.0, accuracy: 0.1)
    }

    func testHeatmapDaysIsEightyFourZeroFilled() {
        let r = report([day("2026-06-08", cost: 1, models: [mb("claude-opus-4-8", cost: 1, total: 10)])])
        let s = Aggregator.summary(from: r, referenceDate: ref)
        XCTAssertEqual(s.heatmapDays.count, 84)
        XCTAssertEqual(s.heatmapDays.last?.date, "2026-06-08")
        XCTAssertEqual(s.heatmapDays.first?.date, "2026-03-17")
        XCTAssertEqual(s.heatmapDays.last?.cost ?? -1, 1, accuracy: 0.001)
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
