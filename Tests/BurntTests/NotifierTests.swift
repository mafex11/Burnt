import XCTest
@testable import BurntCore

final class NotifierTests: XCTestCase {
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
        var out = Notifier.evaluate(input: input(today: 9, month: 9, budget: 10),
                                    options: opts, dayKey: day, monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.contains { $0.id.contains("budget80") })
        XCTAssertFalse(out.contains { $0.id.contains("budget100") })
        out = Notifier.evaluate(input: input(today: 9, month: 9, budget: 10),
                                options: opts, dayKey: day, monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.isEmpty)
        out = Notifier.evaluate(input: input(today: 11, month: 11, budget: 10),
                                options: opts, dayKey: day, monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.contains { $0.id.contains("budget100") })
    }

    func testNoBudgetAlertsWhenDisabledOrNoBudget() {
        var state = NotifierState()
        let opts = NotifierOptions(budgetAlerts: true, dailySummary: false, milestones: false)
        let out = Notifier.evaluate(input: input(today: 99, month: 99, budget: 0),
                                    options: opts, dayKey: "d", monthKey: "m", state: &state)
        XCTAssertTrue(out.isEmpty)
    }

    func testMilestonesFireOncePerMonth() {
        var state = NotifierState()
        let opts = NotifierOptions(budgetAlerts: false, dailySummary: false, milestones: true)
        var out = Notifier.evaluate(input: input(today: 1, month: 120, budget: 0),
                                    options: opts, dayKey: "d", monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.contains { $0.id.contains("milestone100") })
        out = Notifier.evaluate(input: input(today: 1, month: 130, budget: 0),
                                options: opts, dayKey: "d", monthKey: "2026-06", state: &state)
        XCTAssertTrue(out.isEmpty)
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
