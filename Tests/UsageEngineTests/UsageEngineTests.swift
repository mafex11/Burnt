import XCTest
@testable import UsageEngine

final class UsageEngineSmokeTests: XCTestCase {
    func testLoadSummaryAgainstRealCcusageIfAvailable() throws {
        let state = CcusageLocator().resolve()
        guard case .ready = state else {
            throw XCTSkip("ccusage not available on this machine (no bundle, no PATH, no npx)")
        }
        let engine = UsageEngine()
        switch engine.loadSummary() {
        case .success(let s), .stale(let s, _):
            XCTAssertEqual(s.weekByDay.count, 14)
            XCTAssertGreaterThanOrEqual(s.thisWeek.cost, 0)
        case .noData:
            break // acceptable on a machine with no usage
        case .unavailable:
            XCTFail("locator said ready but engine reported unavailable")
        }
    }
}
