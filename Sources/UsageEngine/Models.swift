import Foundation

public struct CcusageReport: Decodable, Sendable {
    public let daily: [DailyUsage]
    public let totals: Totals

    public struct Totals: Decodable, Sendable {
        public let inputTokens: Int
        public let outputTokens: Int
        public let cacheCreationTokens: Int
        public let cacheReadTokens: Int
        public let totalTokens: Int
        public let totalCost: Double
    }
}

public struct DailyUsage: Decodable, Sendable {
    public let period: String              // "2026-06-01" local date
    public let inputTokens: Int
    public let outputTokens: Int
    public let cacheCreationTokens: Int
    public let cacheReadTokens: Int
    public let totalTokens: Int
    public let totalCost: Double
    public let modelBreakdowns: [ModelBreakdown]
    public let metadata: Metadata?

    public struct Metadata: Decodable, Sendable {
        public let agents: [String]?
    }

    public init(period: String, inputTokens: Int, outputTokens: Int,
                cacheCreationTokens: Int, cacheReadTokens: Int, totalTokens: Int,
                totalCost: Double, modelBreakdowns: [ModelBreakdown], metadata: Metadata?) {
        self.period = period
        self.inputTokens = inputTokens
        self.outputTokens = outputTokens
        self.cacheCreationTokens = cacheCreationTokens
        self.cacheReadTokens = cacheReadTokens
        self.totalTokens = totalTokens
        self.totalCost = totalCost
        self.modelBreakdowns = modelBreakdowns
        self.metadata = metadata
    }

    private enum CodingKeys: String, CodingKey {
        case period, date   // ccusage uses "period" (newer) or "date" (older, e.g. 17.1.3)
        case inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens
        case totalTokens, totalCost, modelBreakdowns, metadata
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        // Accept either "period" (newer ccusage) or "date" (older), so a version
        // bump in either direction never silently breaks decoding.
        if let p = try c.decodeIfPresent(String.self, forKey: .period) {
            period = p
        } else {
            period = try c.decode(String.self, forKey: .date)
        }
        inputTokens = try c.decode(Int.self, forKey: .inputTokens)
        outputTokens = try c.decode(Int.self, forKey: .outputTokens)
        cacheCreationTokens = try c.decode(Int.self, forKey: .cacheCreationTokens)
        cacheReadTokens = try c.decode(Int.self, forKey: .cacheReadTokens)
        totalTokens = try c.decode(Int.self, forKey: .totalTokens)
        totalCost = try c.decode(Double.self, forKey: .totalCost)
        modelBreakdowns = try c.decode([ModelBreakdown].self, forKey: .modelBreakdowns)
        metadata = try c.decodeIfPresent(Metadata.self, forKey: .metadata)
    }
}

public struct ModelBreakdown: Decodable, Sendable {
    public let modelName: String
    public let inputTokens: Int
    public let outputTokens: Int
    public let cacheCreationTokens: Int
    public let cacheReadTokens: Int
    public let cost: Double
}
