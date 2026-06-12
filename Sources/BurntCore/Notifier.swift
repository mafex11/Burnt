import Foundation

public protocol NotificationPosting {
    func post(title: String, body: String, id: String)
}

public struct NotifierInput: Sendable {
    public let todayCost: Double
    public let monthCost: Double
    public let dailyBudget: Double
    public let yesterdayCost: Double
    public let yesterdayTopModel: String
    public init(todayCost: Double, monthCost: Double, dailyBudget: Double,
                yesterdayCost: Double, yesterdayTopModel: String) {
        self.todayCost = todayCost; self.monthCost = monthCost; self.dailyBudget = dailyBudget
        self.yesterdayCost = yesterdayCost; self.yesterdayTopModel = yesterdayTopModel
    }
}

public struct NotifierOptions: Sendable {
    public let budgetAlerts: Bool
    public let dailySummary: Bool
    public let milestones: Bool
    public init(budgetAlerts: Bool, dailySummary: Bool, milestones: Bool) {
        self.budgetAlerts = budgetAlerts; self.dailySummary = dailySummary; self.milestones = milestones
    }
}

public struct NotifierState: Codable, Sendable {
    public var fired: Set<String> = []
    public init() {}
}

public struct PendingNotification: Sendable, Equatable {
    public let title: String, body: String, id: String
}

public enum Notifier {
    static let milestoneLevels: [Double] = [50, 100, 250, 500, 1000]

    public static func evaluate(input: NotifierInput, options: NotifierOptions,
                                dayKey: String, monthKey: String,
                                state: inout NotifierState) -> [PendingNotification] {
        var out: [PendingNotification] = []
        func fireOnce(_ id: String, _ make: () -> PendingNotification) {
            guard !state.fired.contains(id) else { return }
            state.fired.insert(id); out.append(make())
        }

        if options.budgetAlerts, input.dailyBudget > 0 {
            let ratio = input.todayCost / input.dailyBudget
            if ratio >= 0.8 {
                fireOnce("budget80-\(dayKey)") {
                    PendingNotification(title: "80% of daily budget",
                        body: "Today: \(money(input.todayCost)) of \(money(input.dailyBudget)).",
                        id: "budget80-\(dayKey)") }
            }
            if ratio >= 1.0 {
                fireOnce("budget100-\(dayKey)") {
                    PendingNotification(title: "Daily budget reached",
                        body: "Today: \(money(input.todayCost)) — over your \(money(input.dailyBudget)) cap.",
                        id: "budget100-\(dayKey)") }
            }
        }

        if options.milestones {
            for level in milestoneLevels where input.monthCost >= level {
                fireOnce("milestone\(Int(level))-\(monthKey)") {
                    PendingNotification(title: "Burnt \(money(level)) this month",
                        body: "Month to date: \(money(input.monthCost)).",
                        id: "milestone\(Int(level))-\(monthKey)") }
            }
        }

        if options.dailySummary {
            fireOnce("summary-\(dayKey)") {
                let model = input.yesterdayTopModel.isEmpty ? "" : " · mostly \(input.yesterdayTopModel)"
                return PendingNotification(title: "Yesterday on Burnt",
                    body: "\(money(input.yesterdayCost)) burnt\(model).",
                    id: "summary-\(dayKey)") }
        }
        return out
    }

    private static func money(_ v: Double) -> String { String(format: "$%.2f", v) }
}
