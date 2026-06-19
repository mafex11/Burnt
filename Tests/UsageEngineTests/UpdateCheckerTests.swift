import XCTest
@testable import UsageEngine

final class UpdateCheckerTests: XCTestCase {
    func testEqualIsUpToDate() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.1", latest: "1.2.1"), .upToDate)
    }
    func testPatchBumpAvailable() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.1", latest: "1.2.2"), .updateAvailable("1.2.2"))
    }
    func testMinorAndMajorBumpAvailable() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.9", latest: "1.3.0"), .updateAvailable("1.3.0"))
        XCTAssertEqual(UpdateChecker.compare(current: "1.9.9", latest: "2.0.0"), .updateAvailable("2.0.0"))
    }
    func testCurrentAheadIsUpToDate() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.3.0", latest: "1.2.9"), .upToDate)
    }
    func testDoubleDigitComponentsCompareNumerically() {
        // string compare would call "1.2.10" < "1.2.9"; numeric must not.
        XCTAssertEqual(UpdateChecker.compare(current: "1.2.9", latest: "1.2.10"), .updateAvailable("1.2.10"))
    }
    func testUnevenComponentCountPadsWithZero() {
        XCTAssertEqual(UpdateChecker.compare(current: "1.2", latest: "1.2.0"), .upToDate)
        XCTAssertEqual(UpdateChecker.compare(current: "1.2", latest: "1.2.1"), .updateAvailable("1.2.1"))
    }
}
