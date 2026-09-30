import XCTest
@testable import BurntCore

final class SpendWatcherTests: XCTestCase {
    func testFirstReadingOnlySetsBaseline() {
        var w = SpendWatcher()
        XCTAssertFalse(w.observe(cost: 12.40))
    }

    func testRiseTriggersFlare() {
        var w = SpendWatcher()
        _ = w.observe(cost: 12.40)
        XCTAssertTrue(w.observe(cost: 12.55))
    }

    func testUnchangedCostDoesNotFlare() {
        var w = SpendWatcher()
        _ = w.observe(cost: 12.40)
        XCTAssertFalse(w.observe(cost: 12.40))
    }

    func testSubCentNoiseDoesNotFlare() {
        var w = SpendWatcher()
        _ = w.observe(cost: 12.400)
        XCTAssertFalse(w.observe(cost: 12.403))
    }

    /// Today's cost resets at midnight; the drop must not flare, and spend after
    /// the reset is measured from the new, lower baseline.
    func testDropResetsBaseline() {
        var w = SpendWatcher()
        _ = w.observe(cost: 40.00)
        XCTAssertFalse(w.observe(cost: 0.00))
        XCTAssertTrue(w.observe(cost: 0.20))
    }
}
