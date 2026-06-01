import XCTest
@testable import UsageEngine

final class ToolClassifierTests: XCTestCase {
    func testClaudeModels() {
        for name in ["claude-opus-4-8", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001"] {
            XCTAssertEqual(ToolClassifier.tool(forModel: name), .claude, name)
        }
    }

    func testCodexModels() {
        for name in ["gpt-5.4", "gpt-5.2-codex", "o3", "o1-mini", "codex-mini"] {
            XCTAssertEqual(ToolClassifier.tool(forModel: name), .codex, name)
        }
    }

    func testUnknownDefaultsToClaude() {
        // Conservative default: unknown model names are treated as Claude (Anthropic is primary).
        XCTAssertEqual(ToolClassifier.tool(forModel: "mystery-model-7"), .claude)
    }
}
