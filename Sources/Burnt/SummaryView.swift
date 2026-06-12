import SwiftUI
import UsageEngine
import BurntCore

struct SummaryView: View {
    let summary: Summary
    let stale: Date?
    @ObservedObject var settings: BurntCore.Settings
    let onGear: () -> Void
    let onRefresh: () -> Void
    let isLoading: Bool

    private var heroValue: String { Formatters.cost(summary.today.cost) }
    private var maxToolCost: Double { max(summary.byTool.map(\.cost).max() ?? 0.01, 0.01) }
    private var maxModelCost: Double { max(summary.byModel.map(\.cost).max() ?? 0.01, 0.01) }
    private var maxProjectCost: Double { max(summary.byProject.map(\.cost).max() ?? 0.01, 0.01) }

    private var style: DashboardStyle { settings.dashboardStyle }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            // Hero — always shown.
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(heroValue).font(.system(size: 30, weight: .semibold)).monospacedDigit()
                    HStack(spacing: 6) {
                        Text("today").font(.caption).foregroundStyle(.secondary)
                        // Trend: Standard and up.
                        if style >= .standard, let t = summary.weekTrend { TrendArrow(trend: t) }
                    }
                }
                Spacer()
                VStack(spacing: 6) {
                    Button(action: onGear) { Image(systemName: "gearshape") }
                        .buttonStyle(.borderless)
                    RefreshButton(isLoading: isLoading, action: onRefresh)
                }
            }

            // Budget bar — always shown when a budget is set.
            if settings.dailyBudget > 0 {
                BudgetBar(spent: summary.today.cost, budget: settings.dailyBudget)
            }

            Divider()

            // Stats row: Week / Month / All-time — shown at every level.
            HStack(spacing: 8) {
                StatCell(label: "Week", value: Formatters.cost(summary.thisWeek.cost))
                StatCell(label: "Month", value: Formatters.cost(summary.monthToDate.cost))
                StatCell(label: "All-time", value: Formatters.cost(summary.allTime.cost))
            }

            // avg/day + pace — Detailed only.
            if style >= .detailed {
                HStack(spacing: 4) {
                    Text("avg \(Formatters.cost(summary.avgPerDay))/day")
                    if let p = summary.projectedToday {
                        Text("· pace ~\(Formatters.cost(p)) today")
                    }
                }
                .font(.caption).foregroundStyle(.secondary)
            }

            // Sparkline — shown at every level.
            Sparkline(points: summary.weekByDay)

            if style >= .detailed {
                sectionHeader("Last 12 weeks")
                HeatmapView(days: summary.heatmapDays)
            }

            // By tool — Standard and up.
            if style >= .standard {
                sectionHeader("By tool")
                ForEach(summary.byTool, id: \.tool) { t in
                    BreakdownBar(color: ToolColor.of(t.tool), label: t.tool.rawValue.capitalized,
                                 fraction: t.cost / maxToolCost, cost: t.cost, tokens: t.totalTokens)
                }
            }

            // By model + cache savings — Detailed only.
            if style >= .detailed {
                sectionHeader("By model")
                ForEach(summary.byModel.prefix(5), id: \.modelName) { m in
                    BreakdownBar(color: ToolColor.of(m.tool), label: m.modelName,
                                 fraction: m.cost / maxModelCost, cost: m.cost, tokens: m.totalTokens)
                }

                if summary.cacheSavings > 0.01 {
                    Text("≈ \(Formatters.cost(summary.cacheSavings)) saved via cache")
                        .font(.caption).foregroundStyle(.green)
                }
            }

            if style >= .detailed, !summary.byProject.isEmpty {
                sectionHeader("By project")
                ForEach(summary.byProject.prefix(5), id: \.path) { p in
                    BreakdownBar(color: .secondary, label: p.name,
                                 fraction: p.cost / maxProjectCost, cost: p.cost, tokens: p.totalTokens)
                }
            }

            // Stale indicator only appears when data is stale.
            if let stale {
                Divider()
                StaleBadge(generatedAt: stale)
            }
        }
        .padding()
    }

    private func sectionHeader(_ title: String) -> some View {
        Text(title.uppercased())
            .font(.system(size: 10, weight: .semibold))
            .foregroundStyle(.secondary)
            .padding(.top, 2)
    }
}
