import Foundation
import SwiftUI
import UsageEngine
import BurntCore

@MainActor
final class AppModel: ObservableObject {
    @Published var result: EngineResult = .noData
    @Published var isLoading = false
    @Published var showingWrapped = false

    let settings: BurntCore.Settings
    private let engine = UsageEngine()
    private var pollTimer: Timer?
    private let notifier = NotificationService()
    private var notifierState = NotifierState()

    init(settings: BurntCore.Settings = BurntCore.Settings()) {
        self.settings = settings
    }

    /// Refresh now (live pricing), used on popover open and the manual button.
    func refresh() { load() }

    /// Refresh immediately, then poll every 60s so the menu bar number stays live.
    func startAutoRefresh() {
        refresh()
        pollTimer?.invalidate()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh() }
        }
    }

    private func load() {
        isLoading = true
        Task.detached { [engine] in
            let r = engine.loadSummary()
            await MainActor.run {
                self.result = r
                self.isLoading = false
                self.runNotifications()
            }
        }
    }

    private func runNotifications() {
        let s: Summary
        switch result { case .success(let x), .stale(let x, _): s = x; default: return }
        let opts = NotifierOptions(budgetAlerts: settings.notifyBudget,
                                   dailySummary: settings.notifyDailySummary,
                                   milestones: settings.notifyMilestones)
        guard opts.budgetAlerts || opts.dailySummary || opts.milestones else { return }
        let yesterday = s.heatmapDays.dropLast().last
        let input = NotifierInput(todayCost: s.today.cost, monthCost: s.monthToDate.cost,
                                  dailyBudget: settings.dailyBudget,
                                  yesterdayCost: yesterday?.cost ?? 0,
                                  yesterdayTopModel: s.byModel.first?.modelName ?? "")
        let cal = Calendar(identifier: .gregorian)
        let c = cal.dateComponents([.year,.month,.day], from: s.generatedAt)
        let dayKey = String(format: "%04d-%02d-%02d", c.year!, c.month!, c.day!)
        let monthKey = String(format: "%04d-%02d", c.year!, c.month!)
        let pending = Notifier.evaluate(input: input, options: opts,
                                        dayKey: dayKey, monthKey: monthKey, state: &notifierState)
        for n in pending { notifier.post(title: n.title, body: n.body, id: n.id) }
    }

    /// Builds the Wrapped card. allTime=false → this month; true → all-time.
    func wrappedData(allTime: Bool = false) -> WrappedData? {
        let s: Summary
        switch result { case .success(let x), .stale(let x, _): s = x; default: return nil }
        let busiest = s.heatmapDays.max { $0.cost < $1.cost }
        let claudeCost = s.byTool.first { $0.tool == .claude }?.cost ?? 0
        let totalToolCost = s.byTool.reduce(0) { $0 + $1.cost }
        let totals = allTime ? s.allTime : s.monthToDate
        return WrappedData(
            title: allTime ? "All-Time" : "This Month",
            totalCost: totals.cost, totalTokens: totals.totalTokens,
            models: s.byModel.map { ($0.modelName, $0.cost) },
            busiestDay: busiest.map { prettyDate($0.date) } ?? "—",
            busiestDayCost: busiest?.cost ?? 0,
            claudeShare: totalToolCost > 0 ? claudeCost / totalToolCost : 0,
            cacheSaved: s.cacheSavings)
    }

    private func prettyDate(_ iso: String) -> String {
        let p = iso.split(separator: "-").compactMap { Int($0) }; guard p.count == 3 else { return iso }
        let m = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
        return "\(p[1] >= 1 && p[1] <= 12 ? m[p[1]-1] : "?") \(p[2])"
    }

    /// The headline string for the menu bar label, per the chosen display mode.
    /// Empty string means "icon only" (no text beside the flame).
    var menuBarText: String {
        if settings.menuBarMode == .iconOnly { return "" }
        guard let s = currentSummary else { return "—" }
        switch settings.menuBarMode {
        case .todayCost:   return Formatters.cost(s.today.cost)
        case .todayTokens: return Formatters.tokens(s.today.totalTokens)
        case .weekCost:    return Formatters.cost(s.thisWeek.cost)
        case .iconOnly:    return ""
        }
    }

    private var currentSummary: Summary? {
        switch result {
        case .success(let s), .stale(let s, _): return s
        case .unavailable, .noData: return nil
        }
    }
}
