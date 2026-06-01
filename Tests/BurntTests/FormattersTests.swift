import XCTest
@testable import BurntCore

final class FormattersTests: XCTestCase {
    func testTokensCompact() {
        XCTAssertEqual(Formatters.tokens(0), "0")
        XCTAssertEqual(Formatters.tokens(999), "999")
        XCTAssertEqual(Formatters.tokens(1_000), "1.0K")
        XCTAssertEqual(Formatters.tokens(340_000), "340K")
        XCTAssertEqual(Formatters.tokens(1_234_567), "1.2M")
        XCTAssertEqual(Formatters.tokens(12_000_000_000), "12.0B")
    }

    func testCost() {
        XCTAssertEqual(Formatters.cost(4.2), "$4.20")
        XCTAssertEqual(Formatters.cost(7468.3), "$7,468")
        XCTAssertEqual(Formatters.cost(0), "$0.00")
    }

    func testPercent() {
        XCTAssertEqual(Formatters.percent(0.12), "12%")
        XCTAssertEqual(Formatters.percent(-0.5), "50%")
        XCTAssertEqual(Formatters.percent(1.24), "124%")
    }
}
