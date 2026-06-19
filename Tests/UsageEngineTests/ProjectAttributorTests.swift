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

    /// A session log's cwd is immutable once written, so once we've parsed a file
    /// the result is cached by (path, mtime). Proof: read once, blank the file's
    /// CONTENTS while preserving its mtime, read again — the cached cwd survives,
    /// showing we didn't re-parse. (If we re-read, the blanked file would yield nil.)
    func testBuildCwdMapCachesUnchangedFileByMtime() throws {
        let fm = FileManager.default
        let tmp = fm.temporaryDirectory.appendingPathComponent("burnt-cache-\(UUID().uuidString)")
        let projDir = tmp.appendingPathComponent(".claude/projects/d")
        try fm.createDirectory(at: projDir, withIntermediateDirectories: true)
        let codexRoot = tmp.appendingPathComponent(".codex")
        try fm.createDirectory(at: codexRoot.appendingPathComponent("sessions"), withIntermediateDirectories: true)

        let sid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
        let file = projDir.appendingPathComponent("\(sid).jsonl")
        try "{\"cwd\":\"/Users/me/code/cached\"}\n".write(to: file, atomically: true, encoding: .utf8)
        // Pin a whole-second mtime so it round-trips exactly (sub-second precision is
        // lost on disk and would otherwise make the cached mtime compare unequal).
        let mtime = Date(timeIntervalSince1970: 1_700_000_000)
        try fm.setAttributes([.modificationDate: mtime], ofItemAtPath: file.path)

        let claudeRoot = tmp.appendingPathComponent(".claude")
        let first = ProjectAttributor.buildCwdMap(claudeRoot: claudeRoot, codexRoot: codexRoot)
        XCTAssertEqual(first[sid], "/Users/me/code/cached")

        // Blank the contents but restore the same mtime → looks "unchanged".
        try "{}\n".write(to: file, atomically: true, encoding: .utf8)
        try fm.setAttributes([.modificationDate: mtime], ofItemAtPath: file.path)

        let second = ProjectAttributor.buildCwdMap(claudeRoot: claudeRoot, codexRoot: codexRoot)
        XCTAssertEqual(second[sid], "/Users/me/code/cached",
                       "unchanged mtime should serve the cached cwd without re-parsing")
        try? fm.removeItem(at: tmp)
    }
}
