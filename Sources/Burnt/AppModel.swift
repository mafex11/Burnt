import Foundation
import SwiftUI
import UsageEngine
import BurntCore

@MainActor
final class AppModel: ObservableObject {
    @Published var result: EngineResult = .noData
    @Published var isLoading = false
    @Published var showingWrapped = false
    @Published var flameFrame = 0          // current pixel-flame animation frame

    let settings: BurntCore.Settings
    private let engine = UsageEngine()
    private var pollTimer: Timer?
    private var flameTimer: Timer?
    private let notifier = NotificationService()
    private var notifierState = NotifierState()

    @Published var updateState: UpdateUIState = .idle
    private let brew = BrewUpdater()
    private var updateTimer: Timer?
    private let lastCheckKey = "lastUpdateCheck"

    enum UpdateUIState: Equatable { case idle, checking, upToDate, available(String), updating }

    /// App version from the bundle (e.g. "1.2.1"); "0" if unreadable.
    private var currentVersion: String {
        (Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String) ?? "0"
    }

    init(settings: BurntCore.Settings = BurntCore.Settings()) {
        self.settings = settings
    }

    /// True while the popover is on screen. The background poll uses this to decide
    /// whether the heavy per-project attribution is worth computing.
    private var popoverOpen = false

    /// Full refresh including per-project attribution — used on popover open and the
    /// manual Refresh button, where the Detailed "By project" list may be visible.
    func refresh() { load(includeProjects: true) }

    /// Called when the popover appears/disappears so the poll can stay light while it's
    /// closed. Opening triggers an immediate full refresh so projects are current.
    func setPopoverOpen(_ open: Bool) {
        popoverOpen = open
        if open { refresh() }
    }

    /// Refresh immediately, then poll every 60s so the menu bar number stays live.
    /// The poll is LIGHT (no project attribution) unless the popover is open in
    /// Detailed mode — that's the only place project data is shown, and it spares a
    /// second subprocess plus a walk of every session log on every tick.
    func startAutoRefresh() {
        refresh()
        pollTimer?.invalidate()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            Task { @MainActor in
                guard let self else { return }
                let needProjects = self.popoverOpen && self.settings.dashboardStyle == .detailed
                self.load(includeProjects: needProjects)
            }
        }
        startFlameAnimation()
        startUpdateChecks()
    }

    /// Cycle the pixel flame at ~6fps while enabled. Cheap; advances a published
    /// frame index the menu bar label observes. Off → hold a single frame.
    func startFlameAnimation() {
        flameTimer?.invalidate()
        guard settings.animateFlame else { flameFrame = 0; return }
        flameTimer = Timer.scheduledTimer(withTimeInterval: 0.16, repeats: true) { [weak self] _ in
            Task { @MainActor in
                guard let self else { return }
                self.flameFrame = (self.flameFrame + 1) % PixelFlame.frameCount
            }
        }
    }

    /// Shared by the daily timer and the Settings button. Fetches the tap's latest
    /// version off-main, compares, and (when auto-update is on and the app is
    /// brew-managed) applies via brew. Best-effort: any failure degrades to idle.
    func checkForUpdates(userInitiated: Bool) {
        if userInitiated { updateState = .checking }
        let current = currentVersion
        let autoOn = settings.autoUpdate
        let brew = self.brew
        Task.detached {
            let status: UpdateStatus
            do {
                let latest = try UpdateChecker.latestVersion()
                status = UpdateChecker.compare(current: current, latest: latest)
            } catch {
                await MainActor.run { if userInitiated { self.updateState = .idle } }
                return
            }
            let shouldUpgrade = await MainActor.run { () -> Bool in
                UserDefaults.standard.set(Date(), forKey: self.lastCheckKey)
                switch status {
                case .upToDate:
                    self.updateState = .upToDate
                    return false
                case .updateAvailable(let v):
                    if autoOn && brew.isBrewManaged() {
                        self.updateState = .updating
                        return true
                    } else {
                        self.updateState = .available(v)
                        return false
                    }
                }
            }
            if shouldUpgrade { brew.upgrade() }
        }
    }

    /// Check on launch if a day has passed (or never checked), then schedule a daily
    /// wall-clock check. Stored last-check date means a Mac that slept overnight still
    /// checks promptly after wake rather than drifting on pure uptime.
    func startUpdateChecks() {
        let last = UserDefaults.standard.object(forKey: lastCheckKey) as? Date
        if last == nil || Date().timeIntervalSince(last!) >= 86_400 {
            checkForUpdates(userInitiated: false)
        }
        updateTimer?.invalidate()
        updateTimer = Timer.scheduledTimer(withTimeInterval: 86_400, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.checkForUpdates(userInitiated: false) }
        }
    }

    private func load(includeProjects: Bool) {
        isLoading = true
        Task.detached { [engine] in
            let r = engine.loadSummary(includeProjects: includeProjects)
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
