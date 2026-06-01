import XCTest
@testable import UsageEngine

final class UsageEngineSmokeTests: XCTestCase {
    func testLoadSummaryAgainstRealCcusageIfAvailable() throws {
        let state = CcusageLocator().resolve()
        guard case .ready = state else {
            throw XCTSkip("ccusage not available on this machine (no bundle, no PATH, no npx)")
        }
        let engine = UsageEngine()
        // Use offline for the test so it never depends on network availability in CI.
        switch engine.loadSummary(offline: true) {
        case .success(let s), .stale(let s, _):
            XCTAssertEqual(s.weekByDay.count, 7)
            XCTAssertGreaterThanOrEqual(s.thisWeek.cost, 0)
        case .noData:
            break // acceptable on a machine with no usage
        case .unavailable:
            XCTFail("locator said ready but engine reported unavailable")
        }
    }
}
