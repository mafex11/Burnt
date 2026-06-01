import Foundation

public enum Tool: String, Codable, Sendable, CaseIterable {
    case claude
    case codex
}

public struct Totals: Sendable, Equatable {
    public var cost: Double = 0
    public var inputTokens: Int = 0
    public var outputTokens: Int = 0
    public var cacheCreationTokens: Int = 0
    public var cacheReadTokens: Int = 0
    public var totalTokens: Int = 0
}

public struct DayPoint: Sendable, Equatable {
    public let date: String   // "2026-06-01"
    public let cost: Double
}

public struct ToolSlice: Sendable, Equatable {
    public let tool: Tool
    public let cost: Double
    public let totalTokens: Int
}

public struct ModelSlice: Sendable, Equatable {
    public let modelName: String
    public let tool: Tool
    public let cost: Double
    public let totalTokens: Int
}

public struct Summary: Sendable, Equatable {
    public let today: Totals
    public let thisWeek: Totals          // rolling 7 days ending on referenceDate
    public let weekByDay: [DayPoint]     // 7 points oldest→newest, zero-filled
    public let byTool: [ToolSlice]       // week range
    public let byModel: [ModelSlice]     // week range, sorted by cost desc
    public let cacheSavings: Double      // week range, Claude-only, approximate
    public let generatedAt: Date
}
