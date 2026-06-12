import Foundation

public struct WrappedData: Sendable {
    public struct ModelBar: Sendable, Equatable {
        public let name: String
        public let cost: Double
        public let fraction: Double   // 0...1 of the top model's cost
    }

    public let title: String
    public let headlineCost: String
    public let headlineTokens: String
    public let topModelName: String
    public let modelBars: [ModelBar]
    public let busiestDay: String
    public let busiestDayCost: String
    public let claudeShare: Double      // 0...1
    public let cacheSaved: String

    /// models: (name, cost) pairs, any order.
    public init(title: String, totalCost: Double, totalTokens: Int,
                models: [(String, Double)], busiestDay: String, busiestDayCost: Double,
                claudeShare: Double, cacheSaved: Double) {
        self.title = title
        self.headlineCost = Formatters.cost(totalCost)
        self.headlineTokens = Formatters.tokens(totalTokens)
        let sorted = models.sorted { $0.1 > $1.1 }
        self.topModelName = sorted.first?.0 ?? "—"
        let top = max(sorted.first?.1 ?? 0, 0.0001)
        self.modelBars = sorted.prefix(5).map { ModelBar(name: $0.0, cost: $0.1, fraction: $0.1 / top) }
        self.busiestDay = busiestDay
        self.busiestDayCost = Formatters.cost(busiestDayCost)
        self.claudeShare = claudeShare
        self.cacheSaved = Formatters.cost(cacheSaved)
    }
}
