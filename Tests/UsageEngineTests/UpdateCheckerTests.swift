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
    func testParsesVersionFromCaskText() {
        let cask = """
        cask "burnt" do
          version "1.2.2"
          sha256 "abc123"
        end
        """
        XCTAssertEqual(UpdateChecker.parseVersion(fromCask: cask), "1.2.2")
    }
    func testParseReturnsNilWhenNoVersion() {
        XCTAssertNil(UpdateChecker.parseVersion(fromCask: "cask \"burnt\" do\nend"))
    }
    func testLatestVersionUsesInjectedFetch() throws {
        let cask = "cask \"burnt\" do\n  version \"3.4.5\"\nend"
        let v = try UpdateChecker.latestVersion { _ in Data(cask.utf8) }
        XCTAssertEqual(v, "3.4.5")
    }
    func testLatestVersionThrowsOnUnparseable() {
        XCTAssertThrowsError(try UpdateChecker.latestVersion { _ in Data("garbage".utf8) }) { err in
            XCTAssertEqual(err as? UpdateChecker.UpdateError, .unparseable)
        }
    }
}
