import SwiftUI
import BurntCore

struct SettingsView: View {
    @ObservedObject var settings: BurntCore.Settings
    @ObservedObject var model: AppModel
    let onBack: () -> Void
    var onShowWrapped: () -> Void = {}

    // Local text for the budget field; 0 shows as empty.
    @State private var budgetText: String = ""

    private var updateStatusText: String {
        switch model.updateState {
        case .idle:             return ""
        case .checking:         return "Checking…"
        case .upToDate:         return "You're up to date"
        case .available(let v): return "Update available — v\(v)"
        case .updating:         return "Updating…"
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Button(action: onBack) { Image(systemName: "chevron.left") }
                    .buttonStyle(.borderless)
                Text("Settings").font(.headline)
                Spacer()
            }

            Picker("Menu bar shows", selection: $settings.menuBarMode) {
                ForEach(MenuBarMode.allCases, id: \.self) { mode in
                    Text(mode.label).tag(mode)
                }
            }

            Toggle("Animate flame", isOn: $settings.animateFlame)

            Picker("Dashboard style", selection: $settings.dashboardStyle) {
                ForEach(DashboardStyle.allCases, id: \.self) { style in
                    Text(style.label).tag(style)
                }
            }

            HStack {
                Text("Daily budget")
                Spacer()
                Text("$")
                TextField("off", text: $budgetText)
                    .frame(width: 70)
                    .multilineTextAlignment(.trailing)
                    .onSubmit { commitBudget() }
                    .onChange(of: budgetText) { _, _ in commitBudget() }
            }

            Toggle("Launch at login", isOn: $settings.launchAtLogin)

            Divider()
            Text("Notifications").font(.caption).foregroundStyle(.secondary)
            Toggle("Budget alerts", isOn: $settings.notifyBudget)
            Toggle("Daily summary", isOn: $settings.notifyDailySummary)
            Toggle("Spend milestones", isOn: $settings.notifyMilestones)

            Divider()
            Text("Updates").font(.caption).foregroundStyle(.secondary)
            Toggle("Automatically update Burnt", isOn: $settings.autoUpdate)
            HStack {
                Button("Check for Updates") { model.checkForUpdates(userInitiated: true) }
                    .buttonStyle(.borderless)
                Spacer()
                Text(updateStatusText).font(.caption).foregroundStyle(.secondary)
            }

            Divider()
            Button("Burnt Wrapped…", action: onShowWrapped).buttonStyle(.borderless)

            Divider()

            Button("Quit Burnt") { NSApplication.shared.terminate(nil) }
                .buttonStyle(.borderless)

            Spacer(minLength: 0)
        }
        .padding()
        .frame(width: 300)
        .onAppear {
            budgetText = settings.dailyBudget > 0 ? String(format: "%.2f", settings.dailyBudget) : ""
        }
    }

    private func commitBudget() {
        let cleaned = budgetText.replacingOccurrences(of: "$", with: "").trimmingCharacters(in: .whitespaces)
        settings.dailyBudget = Double(cleaned) ?? 0
    }
}
