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

    func testBuildCwdMapReadsCwdBeyond64KBFirstLine() throws {
        let fm = FileManager.default
        let tmp = fm.temporaryDirectory.appendingPathComponent("burnt-test-\(UUID().uuidString)")
        let projDir = tmp.appendingPathComponent(".claude/projects/somedir")
        try fm.createDirectory(at: projDir, withIntermediateDirectories: true)
        // session file: line 1 is a giant cwd-less object, line 2 has the cwd.
        let sid = "11111111-2222-3333-4444-555555555555"
        let huge = String(repeating: "x", count: 80_000)
        let line1 = "{\"type\":\"user\",\"blob\":\"\(huge)\"}"
        let line2 = "{\"type\":\"assistant\",\"cwd\":\"/Users/me/code/myproj\"}"
        try "\(line1)\n\(line2)\n".write(to: projDir.appendingPathComponent("\(sid).jsonl"),
                                         atomically: true, encoding: .utf8)
        // empty codex dir so the enumerator no-ops
        let codexRoot = tmp.appendingPathComponent(".codex")
        try fm.createDirectory(at: codexRoot.appendingPathComponent("sessions"), withIntermediateDirectories: true)

        let map = ProjectAttributor.buildCwdMap(
            claudeRoot: tmp.appendingPathComponent(".claude"),
            codexRoot: codexRoot)
        XCTAssertEqual(map[sid], "/Users/me/code/myproj")
        try? fm.removeItem(at: tmp)
    }
}
