import Foundation
import SwiftUI
import UsageEngine

@MainActor
final class AppModel: ObservableObject {
    @Published var result: EngineResult = .noData
    @Published var isLoading = false

    private let engine = UsageEngine()
    private var pollTimer: Timer?

    /// Live, human-triggered refresh: fetches current LiteLLM prices (online).
    /// Call from popover open and the manual refresh button.
    func refresh() { load(offline: false) }

    /// Cheap background refresh: cached pricing, no network. Call from the 60s timer.
    func pollRefresh() { load(offline: true) }

    /// Refresh immediately (live), then poll every 60s offline so the menu bar
    /// number stays live without a network call every minute.
    func startAutoRefresh() {
        refresh()
        pollTimer?.invalidate()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.pollRefresh() }
        }
    }

    private func load(offline: Bool) {
        isLoading = true
        Task.detached { [engine] in
            let r = engine.loadSummary(offline: offline)
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
