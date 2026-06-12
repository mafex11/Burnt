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
        XCTAssertEqual(w.headlineCost, "$112.40")
        XCTAssertEqual(w.headlineTokens, "47.0M")
        XCTAssertEqual(w.topModelName, "claude-opus-4-8")
        XCTAssertEqual(w.modelBars.count, 2)
        XCTAssertEqual(w.modelBars[0].fraction, 1.0, accuracy: 0.001)
        XCTAssertEqual(w.modelBars[1].fraction, 22.0/90.0, accuracy: 0.001)
    }
}
