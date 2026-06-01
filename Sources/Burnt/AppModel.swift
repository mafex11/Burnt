import Foundation
import SwiftUI
import UsageEngine
import BurntCore

@MainActor
final class AppModel: ObservableObject {
    @Published var result: EngineResult = .noData
    @Published var isLoading = false

    let settings: BurntCore.Settings
    private let engine = UsageEngine()
    private var pollTimer: Timer?

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
            }
        }
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
