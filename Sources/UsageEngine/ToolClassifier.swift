import Foundation

public enum ToolClassifier {
    /// Classify a model name to its originating tool by name prefix.
    /// Codex (OpenAI) families: gpt-*, o1*, o3*, codex*. Everything else → Claude.
    public static func tool(forModel modelName: String) -> Tool {
        let name = modelName.lowercased()
        let codexPrefixes = ["gpt-", "o1", "o3", "codex"]
        if codexPrefixes.contains(where: { name.hasPrefix($0) }) {
            return .codex
        }
        return .claude
    }
}
