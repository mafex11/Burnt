import XCTest
@testable import UsageEngine

final class DecodingTests: XCTestCase {
    private func fixture(_ name: String) throws -> Data {
        let url = Bundle.module.url(forResource: name, withExtension: "json", subdirectory: "Fixtures")!
        return try Data(contentsOf: url)
    }

    func testDecodesNormalReport() throws {
        let report = try JSONDecoder().decode(CcusageReport.self, from: fixture("daily-normal"))
        XCTAssertFalse(report.daily.isEmpty)
        let day = report.daily.first!
        XCTAssertFalse(day.period.isEmpty)
        XCTAssertFalse(day.modelBreakdowns.isEmpty)
        XCTAssertGreaterThanOrEqual(report.totals.totalCost, 0)
    }

    func testDecodesEmptyReport() throws {
        let report = try JSONDecoder().decode(CcusageReport.self, from: fixture("daily-empty"))
        XCTAssertTrue(report.daily.isEmpty)
    }

    // Regression: ccusage 17.1.3 (the pinned/bundled version) emits "date" instead
    // of "period". The decoder must accept both so the real bundled binary works.
    func testDecodesPinnedVersionWithDateField() throws {
        let report = try JSONDecoder().decode(CcusageReport.self, from: fixture("daily-pinned-1713"))
        XCTAssertFalse(report.daily.isEmpty)
        let day = report.daily.first!
        XCTAssertFalse(day.period.isEmpty, "the 'date' field must map into period")
        XCTAssertFalse(day.modelBreakdowns.isEmpty)
    }
}
