import Foundation
import SwiftUI
import UsageEngine

@MainActor
final class AppModel: ObservableObject {
    @Published var result: EngineResult = .noData
    @Published var isLoading = false

    private let engine = UsageEngine()
    private var pollTimer: Timer?

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

    /// The headline string for the menu bar label.
    var menuBarText: String {
        switch result {
        case .success(let s), .stale(let s, _):
            return String(format: "$%.2f", s.today.cost)
        case .unavailable, .noData:
            return "—"
        }
    }
}
