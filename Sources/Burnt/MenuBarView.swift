import SwiftUI
import UsageEngine

struct MenuBarView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        Group {
            switch model.result {
            case .unavailable:
                UnavailableView(onRecheck: model.refresh)
            case .noData:
                VStack(spacing: 8) {
                    Text("No usage recorded yet").foregroundStyle(.secondary)
                    Button("Refresh", action: model.refresh)
                }.padding()
            case .success(let s):
                summaryView(s, stale: nil)
            case .stale(let s, _):
                summaryView(s, stale: s.generatedAt)
            }
        }
        .frame(width: 300)
        .onAppear { model.refresh() }   // refresh on every popover open; 60s timer keeps it live in the background
    }

    @ViewBuilder
    private func summaryView(_ s: Summary, stale: Date?) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                VStack(alignment: .leading) {
                    Text("Today").font(.caption).foregroundStyle(.secondary)
                    Text(String(format: "$%.2f", s.today.cost)).font(.system(size: 28, weight: .semibold)).monospacedDigit()
                }
                Spacer()
                VStack(alignment: .trailing) {
                    Text("This week").font(.caption).foregroundStyle(.secondary)
                    Text(String(format: "$%.2f", s.thisWeek.cost)).font(.title3).monospacedDigit()
                }
            }
            Sparkline(points: s.weekByDay)
            Divider()
            ForEach(s.byTool, id: \.tool) { t in
                BreakdownRow(label: t.tool.rawValue.capitalized, cost: t.cost, tokens: t.totalTokens)
            }
            Divider()
            ForEach(s.byModel.prefix(5), id: \.modelName) { m in
                BreakdownRow(label: m.modelName, cost: m.cost, tokens: m.totalTokens)
            }
            if s.cacheSavings > 0.01 {
                Text(String(format: "≈ $%.2f saved via cache", s.cacheSavings))
                    .font(.caption).foregroundStyle(.green)
            }
            HStack {
                if let stale { StaleBadge(generatedAt: stale) }
                Spacer()
                Button(action: model.refresh) {
                    Image(systemName: model.isLoading ? "arrow.triangle.2.circlepath" : "arrow.clockwise")
                }.buttonStyle(.borderless)
                Button("Quit") { NSApplication.shared.terminate(nil) }.buttonStyle(.borderless)
            }
        }.padding()
    }
}
