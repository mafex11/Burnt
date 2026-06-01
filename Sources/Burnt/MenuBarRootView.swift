import SwiftUI
import UsageEngine
import BurntCore

struct MenuBarRootView: View {
    @ObservedObject var model: AppModel
    @State private var showingSettings = false

    var body: some View {
        Group {
            if showingSettings {
                SettingsView(settings: model.settings) { showingSettings = false }
            } else {
                content
            }
        }
        .frame(width: 300)
        .onAppear {
            // LSUIElement apps don't auto-activate; bring the popover to front so it's
            // key and hover events don't bleed to windows beneath.
            NSApp.activate(ignoringOtherApps: true)
            model.refresh()
        }
    }

    @ViewBuilder
    private var content: some View {
        switch model.result {
        case .unavailable:
            UnavailableView(onRecheck: model.refresh)
        case .noData:
            VStack(spacing: 8) {
                Text("No usage recorded yet").foregroundStyle(.secondary)
                Button("Refresh", action: model.refresh)
            }.padding()
        case .success(let s):
            summary(s, stale: nil)
        case .stale(let s, _):
            summary(s, stale: s.generatedAt)
        }
    }

    private func summary(_ s: Summary, stale: Date?) -> some View {
        SummaryView(summary: s, stale: stale, settings: model.settings,
                    onGear: { showingSettings = true },
                    onRefresh: model.refresh,
                    isLoading: model.isLoading)
    }
}
