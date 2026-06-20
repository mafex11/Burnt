import SwiftUI
import UsageEngine
import BurntCore

struct MenuBarRootView: View {
    @ObservedObject var model: AppModel
    @State private var showingSettings = false

    var body: some View {
        Group {
            if showingSettings {
                SettingsView(settings: model.settings, model: model,
                             onBack: { showingSettings = false },
                             onShowWrapped: { model.showingWrapped = true })
            } else {
                content
            }
        }
        .frame(width: 300)
        .sheet(isPresented: $model.showingWrapped) {
            WrappedSheet(model: model) { model.showingWrapped = false }
        }
        .onAppear {
            // LSUIElement apps don't auto-activate; bring the popover to front so it's
            // key and hover events don't bleed to windows beneath.
            NSApp.activate(ignoringOtherApps: true)
            model.setPopoverOpen(true)   // full refresh while visible
        }
        .onDisappear { model.setPopoverOpen(false) }   // poll goes light again
        .onChange(of: model.settings.animateFlame) { _, _ in
            model.startFlameAnimation()   // start/stop the flame when the toggle flips
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
