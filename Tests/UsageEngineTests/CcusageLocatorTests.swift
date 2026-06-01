import XCTest
@testable import UsageEngine

final class CcusageLocatorTests: XCTestCase {
    func testPrefersBundledInvocation() {
        let bundled = CcusageInvocation(executable: "/Apps/Burnt.app/Contents/Resources/node",
                                        leadingArgs: ["/Apps/Burnt.app/Contents/Resources/node_modules/ccusage/dist/cli.js"])
        let loc = CcusageLocator(
            bundledInvocation: { bundled },
            lookup: { _ in "/opt/homebrew/bin/ccusage" }   // present, but bundled wins
        )
        guard case let .ready(inv) = loc.resolve() else { return XCTFail("expected ready") }
        XCTAssertEqual(inv, bundled)
    }

    func testFallsBackToPathCcusage() {
        let loc = CcusageLocator(
            bundledInvocation: { nil },
            lookup: { name in name == "ccusage" ? "/usr/local/bin/ccusage" : nil }
        )
        guard case let .ready(inv) = loc.resolve() else { return XCTFail("expected ready") }
        XCTAssertEqual(inv.executable, "/usr/local/bin/ccusage")
        XCTAssertEqual(inv.leadingArgs, [])
    }

    func testFallsBackToNpx() {
        let loc = CcusageLocator(
            bundledInvocation: { nil },
            lookup: { name in name == "npx" ? "/opt/homebrew/bin/npx" : nil }
        )
        guard case let .ready(inv) = loc.resolve() else { return XCTFail("expected ready") }
        XCTAssertEqual(inv.executable, "/opt/homebrew/bin/npx")
        XCTAssertEqual(inv.leadingArgs, ["-y", "ccusage@\(CcusageRunner.pinnedVersion)"])
    }

    func testUnavailableWhenNothingFound() {
        let loc = CcusageLocator(bundledInvocation: { nil }, lookup: { _ in nil })
        guard case .unavailable = loc.resolve() else { return XCTFail("expected unavailable") }
    }
}
