import XCTest
@testable import UsageEngine

final class ProjectAttributorTests: XCTestCase {
    private func row(_ id: String, _ cost: Double, _ tok: Int) -> SessionRow {
        SessionRow(agent: "claude", period: id, totalCost: cost, totalTokens: tok)
    }

    func testGroupsSessionsByCwdLeaf() {
        let sessions = [row("a", 5, 100), row("b", 3, 50), row("c", 2, 20)]
        let cwdMap = ["a": "/Users/me/code/personal", "b": "/Users/me/code/personal", "c": "/Users/me/code/work"]
        let result = ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
        XCTAssertEqual(result.map(\.name), ["personal", "work"])
        XCTAssertEqual(result[0].cost, 8, accuracy: 0.001)
        XCTAssertEqual(result[0].totalTokens, 150)
        XCTAssertEqual(result[1].cost, 2, accuracy: 0.001)
    }

    func testUnmappedSessionsBucketAsUnknown() {
        let sessions = [row("a", 5, 100), row("z", 4, 40)]
        let cwdMap = ["a": "/Users/me/code/personal"]
        let result = ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
        let unknown = result.first { $0.name == "Unknown" }
        XCTAssertNotNil(unknown)
        XCTAssertEqual(unknown?.cost ?? 0, 4, accuracy: 0.001)
    }

    func testLeafCollisionDisambiguatesWithParent() {
        let sessions = [row("a", 5, 10), row("b", 3, 10)]
        let cwdMap = ["a": "/x/api", "b": "/y/api"]
        let result = ProjectAttributor.group(sessions: sessions, cwdBySession: cwdMap)
        XCTAssertEqual(Set(result.map(\.name)), Set(["x/api", "y/api"]))
    }
}
