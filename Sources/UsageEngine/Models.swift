import Foundation

public struct CcusageReport: Codable, Sendable {
    public let daily: [DailyUsage]
    public let totals: Totals

    public struct Totals: Codable, Sendable {
        public let inputTokens: Int
        public let outputTokens: Int
        public let cacheCreationTokens: Int
        public let cacheReadTokens: Int
        public let totalTokens: Int
        public let totalCost: Double
    }
}

public struct DailyUsage: Codable, Sendable {
    public let period: String              // "2026-06-01" local date
    public let inputTokens: Int
    public let outputTokens: Int
    public let cacheCreationTokens: Int
    public let cacheReadTokens: Int
    public let totalTokens: Int
    public let totalCost: Double
    public let modelBreakdowns: [ModelBreakdown]
    public let metadata: Metadata?

    public struct Metadata: Codable, Sendable {
        public let agents: [String]?
    }
}

public struct ModelBreakdown: Codable, Sendable {
    public let modelName: String
    public let inputTokens: Int
    public let outputTokens: Int
    public let cacheCreationTokens: Int
    public let cacheReadTokens: Int
    public let cost: Double
}
