import XCTest
@testable import UsageEngine

final class CcusageLocatorTests: XCTestCase {
    func testPrefersBundledBinary() {
        let loc = CcusageLocator(
            bundledPath: { "/Apps/Burnt.app/Contents/Resources/ccusage" },
            lookup: { _ in "/opt/homebrew/bin/ccusage" }   // present, but bundled wins
        )
        guard case let .ready(inv) = loc.resolve() else { return XCTFail("expected ready") }
        XCTAssertEqual(inv.executable, "/Apps/Burnt.app/Contents/Resources/ccusage")
        XCTAssertEqual(inv.leadingArgs, [])
    }

    func testFallsBackToPathCcusage() {
        let loc = CcusageLocator(
            bundledPath: { nil },
            lookup: { name in name == "ccusage" ? "/usr/local/bin/ccusage" : nil }
        )
        guard case let .ready(inv) = loc.resolve() else { return XCTFail("expected ready") }
        XCTAssertEqual(inv.executable, "/usr/local/bin/ccusage")
        XCTAssertEqual(inv.leadingArgs, [])
    }

    func testFallsBackToNpx() {
        let loc = CcusageLocator(
            bundledPath: { nil },
            lookup: { name in name == "npx" ? "/opt/homebrew/bin/npx" : nil }
        )
        guard case let .ready(inv) = loc.resolve() else { return XCTFail("expected ready") }
        XCTAssertEqual(inv.executable, "/opt/homebrew/bin/npx")
        XCTAssertEqual(inv.leadingArgs, ["-y", "ccusage@\(CcusageRunner.pinnedVersion)"])
    }

    func testUnavailableWhenNothingFound() {
        let loc = CcusageLocator(bundledPath: { nil }, lookup: { _ in nil })
        guard case .unavailable = loc.resolve() else { return XCTFail("expected unavailable") }
    }
}
