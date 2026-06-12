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

public struct ProjectSlice: Sendable, Equatable {
    public let name: String     // display leaf, e.g. "personal"
    public let path: String     // full cwd (dedup key); "" for the Unknown bucket
    public let cost: Double
    public let totalTokens: Int
}

public struct Summary: Sendable, Equatable {
    public let today: Totals
    public let thisWeek: Totals          // rolling 7 days ending on referenceDate
    public let weekByDay: [DayPoint]     // 7 points oldest→newest, zero-filled
    public let heatmapDays: [DayPoint]   // 84 points oldest→newest, zero-filled (heatmap)
    public let byTool: [ToolSlice]       // week range
    public let byModel: [ModelSlice]     // week range, sorted by cost desc
    public let cacheSavings: Double      // week range, Claude-only, approximate
    public let monthToDate: Totals       // sum of daily rows in referenceDate's calendar month
    public let allTime: Totals           // from CcusageReport.totals
    public let avgPerDay: Double         // thisWeek.cost / 7
    public let lastWeek: Totals          // 7 days before thisWeek (offsets -13..-7)
    public let weekTrend: Double?        // (thisWeek-lastWeek)/lastWeek; nil if lastWeek.cost == 0
    public let projectedToday: Double?   // today.cost / fractionOfDayElapsed; nil if too early
    public let byProject: [ProjectSlice]   // by cwd, current data; empty if unavailable
    public let generatedAt: Date
}
