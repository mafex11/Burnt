import XCTest
@testable import BurntCore

final class SettingsTests: XCTestCase {
    private func freshDefaults() -> UserDefaults {
        let suite = "burnt.tests.\(UUID().uuidString)"
        return UserDefaults(suiteName: suite)!
    }

    func testDefaults() {
        let s = Settings(defaults: freshDefaults(), loginItem: StubLoginItem())
        XCTAssertEqual(s.menuBarMode, .todayCost)
        XCTAssertEqual(s.dailyBudget, 0)
        XCTAssertFalse(s.launchAtLogin)
        XCTAssertEqual(s.dashboardStyle, .standard)   // defaults to Standard
    }

    func testPersistsDashboardStyle() {
        let d = freshDefaults()
        let s1 = Settings(defaults: d, loginItem: StubLoginItem())
        s1.dashboardStyle = .minimal
        let s2 = Settings(defaults: d, loginItem: StubLoginItem())
        XCTAssertEqual(s2.dashboardStyle, .minimal)
    }

    func testDashboardStyleOrdering() {
        XCTAssertTrue(DashboardStyle.minimal < .standard)
        XCTAssertTrue(DashboardStyle.standard < .detailed)
        XCTAssertTrue(DashboardStyle.detailed >= .standard)
    }

    func testPersistsMenuBarModeAndBudget() {
        let d = freshDefaults()
        let s1 = Settings(defaults: d, loginItem: StubLoginItem())
        s1.menuBarMode = .weekCost
        s1.dailyBudget = 12.5
        let s2 = Settings(defaults: d, loginItem: StubLoginItem())
        XCTAssertEqual(s2.menuBarMode, .weekCost)
        XCTAssertEqual(s2.dailyBudget, 12.5, accuracy: 0.001)
    }

    func testLaunchAtLoginDelegatesToLoginItem() {
        let stub = StubLoginItem()
        let s = Settings(defaults: freshDefaults(), loginItem: stub)
        s.launchAtLogin = true
        XCTAssertTrue(stub.isEnabled)
        s.launchAtLogin = false
        XCTAssertFalse(stub.isEnabled)
    }
}
