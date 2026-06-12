import SwiftUI
import UsageEngine
import BurntCore

/// GitHub-contribution-style grid of daily cost (84 days). Columns = weeks.
struct HeatmapView: View {
    let days: [DayPoint]          // oldest→newest, length 84
    @State private var hovered: String?

    private var maxCost: Double { max(days.map(\.cost).max() ?? 0.01, 0.01) }

    private func color(_ cost: Double) -> Color {
        if cost <= 0 { return Color.secondary.opacity(0.15) }
        let t = min(cost / maxCost, 1.0)
        return Color(red: 0.95, green: 0.62 - 0.2 * t, blue: 0.24 - 0.14 * t)
            .opacity(0.35 + 0.65 * t)
    }

    private var weeks: [[DayPoint]] {
        stride(from: 0, to: days.count, by: 7).map { Array(days[$0..<min($0+7, days.count)]) }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(hovered.flatMap { key in days.first { $0.date == key }.map { "\(pretty($0.date)) · \(Formatters.cost($0.cost))" } } ?? "last 12 weeks")
                .font(.caption2).foregroundStyle(hovered == nil ? .secondary : .primary)
            HStack(spacing: 3) {
                ForEach(Array(weeks.enumerated()), id: \.offset) { _, week in
                    VStack(spacing: 3) {
                        ForEach(week, id: \.date) { d in
                            RoundedRectangle(cornerRadius: 2)
                                .fill(color(d.cost))
                .frame(width: 13, height: 13)
                                .onHover { inside in hovered = inside ? d.date : (hovered == d.date ? nil : hovered) }
                        }
                    }
                }
            }
        }
    }

    private func pretty(_ iso: String) -> String {
        let p = iso.split(separator: "-").compactMap { Int($0) }
        guard p.count == 3 else { return iso }
        let m = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
        return "\(p[1] >= 1 && p[1] <= 12 ? m[p[1]-1] : "?") \(p[2])"
    }
}
