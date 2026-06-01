import SwiftUI
import UsageEngine

/// A minimal bar sparkline for 7 daily cost points.
struct Sparkline: View {
    let points: [DayPoint]
    var body: some View {
        let maxCost = max(points.map(\.cost).max() ?? 1, 0.01)
        HStack(alignment: .bottom, spacing: 3) {
            ForEach(points, id: \.date) { p in
                RoundedRectangle(cornerRadius: 2)
                    .frame(width: 14, height: max(4, CGFloat(p.cost / maxCost) * 40))
                    .opacity(p.cost == 0 ? 0.25 : 1)
            }
        }
        .frame(height: 44)
    }
}

struct BreakdownRow: View {
    let label: String
    let cost: Double
    let tokens: Int
    var body: some View {
        HStack {
            Text(label).lineLimit(1)
            Spacer()
            Text("\(tokens.formatted(.number.notation(.compactName)))").foregroundStyle(.secondary)
            Text(String(format: "$%.2f", cost)).monospacedDigit().frame(width: 64, alignment: .trailing)
        }
        .font(.system(size: 12))
    }
}

/// Shown only if the bundled ccusage binary is somehow missing — unreachable
/// in a normal install. A diagnostic, not an install instruction.
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
