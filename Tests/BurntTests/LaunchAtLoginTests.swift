import XCTest
@testable import BurntCore

final class LaunchAtLoginTests: XCTestCase {
    func testStubTogglesState() throws {
        let stub = StubLoginItem()
        XCTAssertFalse(stub.isEnabled)
        try stub.enable()
        XCTAssertTrue(stub.isEnabled)
        try stub.disable()
        XCTAssertFalse(stub.isEnabled)
    }
}

// A controllable test double used here and by SettingsTests (Task 4).
final class StubLoginItem: LoginItemControlling {
    private(set) var isEnabled = false
    func enable() throws { isEnabled = true }
    func disable() throws { isEnabled = false }
}
