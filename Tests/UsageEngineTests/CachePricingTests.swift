import XCTest
@testable import UsageEngine

final class CachePricingTests: XCTestCase {
    func testSavingsIsInputMinusCacheReadRate() {
        // 1,000,000 cache-read tokens on opus: input $15/Mtok, cache-read $1.50/Mtok.
        // Savings = (15.00 - 1.50) * 1.0 = $13.50
        let saved = CachePricing.estimatedSavings(cacheReadTokens: 1_000_000, model: "claude-opus-4-8")
        XCTAssertEqual(saved, 13.50, accuracy: 0.01)
    }

    func testNonClaudeModelSavesNothing() {
        XCTAssertEqual(CachePricing.estimatedSavings(cacheReadTokens: 1_000_000, model: "gpt-5.4"), 0)
    }

    func testZeroTokensSavesNothing() {
        XCTAssertEqual(CachePricing.estimatedSavings(cacheReadTokens: 0, model: "claude-opus-4-8"), 0)
    }
}
