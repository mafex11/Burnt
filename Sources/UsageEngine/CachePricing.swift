import Foundation

/// Approximate cache-savings estimator (Claude-only).
/// Rates are representative LiteLLM USD-per-million-token values, hardcoded
/// because this figure is an illustrative "you saved ≈$Y" number, not billing.
public enum CachePricing {
    /// (inputPerMTok, cacheReadPerMTok) by model-name prefix. First match wins.
    private static let rateTable: [(prefix: String, input: Double, cacheRead: Double)] = [
        ("claude-opus",   15.0, 1.50),
        ("claude-sonnet",  3.0, 0.30),
        ("claude-haiku",   1.0, 0.10),
    ]

    public static func estimatedSavings(cacheReadTokens: Int, model: String) -> Double {
        guard ToolClassifier.tool(forModel: model) == .claude, cacheReadTokens > 0 else { return 0 }
        let name = model.lowercased()
        guard let rate = rateTable.first(where: { name.hasPrefix($0.prefix) }) else { return 0 }
        let perToken = (rate.input - rate.cacheRead) / 1_000_000.0
        return Double(cacheReadTokens) * perToken
    }
}
