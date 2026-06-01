import SwiftUI
import UsageEngine
import BurntCore

/// Brand colors for the two tools, used for dots, %-bars, and the sparkline.
enum ToolColor {
    static let claude = Color(red: 0.949, green: 0.627, blue: 0.239) // #F2A03D amber
    static let codex  = Color(red: 0.239, green: 0.722, blue: 0.408) // #3DB868 green

    static func of(_ tool: Tool) -> Color { tool == .claude ? claude : codex }
}

/// A labelled secondary stat (e.g. "Week" / "$28.90").
struct StatCell: View {
    let label: String
    let value: String
    var body: some View {
        VStack(alignment: .leading, spacing: 1) {
            Text(label).font(.caption2).foregroundStyle(.secondary)
            Text(value).font(.system(size: 13, weight: .medium)).monospacedDigit()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// A breakdown row: colored dot, label, proportion bar, tokens, cost.
struct BreakdownBar: View {
    let color: Color
    let label: String
    let fraction: Double   // 0...1 of the section's max cost
    let cost: Double
    let tokens: Int
    var body: some View {
        HStack(spacing: 6) {
            Circle().fill(color).frame(width: 7, height: 7)
            Text(label).font(.system(size: 12)).lineLimit(1)
            Spacer(minLength: 6)
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule().fill(color.opacity(0.15)).frame(height: 4)
                    Capsule().fill(color).frame(width: max(2, geo.size.width * fraction), height: 4)
                }
            }
            .frame(width: 50, height: 4)
            Text(Formatters.tokens(tokens)).font(.system(size: 11)).foregroundStyle(.secondary)
                .frame(width: 44, alignment: .trailing)
            Text(Formatters.cost(cost)).font(.system(size: 12)).monospacedDigit()
                .frame(width: 56, alignment: .trailing)
        }
    }
}

/// Daily budget progress bar with color thresholds.
struct BudgetBar: View {
    let spent: Double
    let budget: Double   // > 0
    private var fraction: Double { min(spent / budget, 1.0) }
    private var ratio: Double { spent / budget }
    private var color: Color {
        if ratio > 1.0 { return .red }
        if ratio >= 0.8 { return .orange }
        return Color(red: 0.239, green: 0.722, blue: 0.408)
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule().fill(Color.secondary.opacity(0.18)).frame(height: 6)
                    Capsule().fill(color).frame(width: max(3, geo.size.width * fraction), height: 6)
                }
            }
            .frame(height: 6)
            Text("\(Formatters.percent(ratio)) of \(Formatters.cost(budget))")
                .font(.caption2).foregroundStyle(.secondary)
        }
    }
}

/// Week-over-week trend arrow. Positive = up (more spend) shown red; down = green.
struct TrendArrow: View {
    let trend: Double   // fraction, e.g. +0.12
    private var up: Bool { trend >= 0 }
    var body: some View {
        HStack(spacing: 2) {
            Image(systemName: up ? "arrow.up.right" : "arrow.down.right")
            Text("\(Formatters.percent(trend)) vs last week")
        }
        .font(.caption2)
        .foregroundStyle(up ? .red : .green)
    }
}

/// A minimal bar sparkline for 7 daily cost points, with hover tooltips.
struct Sparkline: View {
    let points: [DayPoint]
    var body: some View {
        let maxCost = max(points.map(\.cost).max() ?? 1, 0.01)
        HStack(alignment: .bottom, spacing: 3) {
            ForEach(points, id: \.date) { p in
                RoundedRectangle(cornerRadius: 2)
                    .fill(Color.accentColor)
                    .frame(width: 14, height: max(4, CGFloat(p.cost / maxCost) * 40))
                    .opacity(p.cost == 0 ? 0.25 : 1)
                    .help("\(p.date) · \(Formatters.cost(p.cost))")
            }
        }
        .frame(height: 44)
    }
}

/// Shown only if the bundled ccusage binary is somehow missing.
struct UnavailableView: View {
    let onRecheck: () -> Void
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("ccusage not found").font(.headline)
            Text("Burnt bundles ccusage, but it couldn't be located. Try rechecking, or reinstall Burnt.")
                .font(.system(size: 12)).foregroundStyle(.secondary)
            Button("Recheck", action: onRecheck)
        }.padding()
    }
}

struct StaleBadge: View {
    let generatedAt: Date
    var body: some View {
        Text("stale · \(generatedAt.formatted(date: .omitted, time: .shortened))")
            .font(.caption2).foregroundStyle(.orange)
    }
}
