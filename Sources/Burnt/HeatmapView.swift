import SwiftUI
import UsageEngine
import BurntCore

/// GitHub-contribution-style grid of daily cost (84 days). Columns = weeks.
struct HeatmapView: View {
    let days: [DayPoint]          // oldest→newest, length 84
    @State private var hovered: String?

    // Quantile thresholds over NON-ZERO days. Spend is heavily right-skewed (a few
    // big days dwarf the rest), so a linear scale makes every normal day look the
    // same faint shade. Instead we bucket by rank — like GitHub's contribution
    // graph — so a light / typical / heavy / peak day are visibly distinct.
    private var thresholds: (q1: Double, q2: Double, q3: Double) {
        let nonzero = days.map(\.cost).filter { $0 > 0 }.sorted()
        guard nonzero.count >= 4 else {
            let m = max(nonzero.last ?? 1, 0.01)
            return (m * 0.25, m * 0.5, m * 0.75)   // too few points: fall back to linear-ish
        }
        func quantile(_ p: Double) -> Double {
            let idx = Int(Double(nonzero.count - 1) * p)
            return nonzero[idx]
        }
        return (quantile(0.25), quantile(0.50), quantile(0.75))
    }

    /// 0 = empty, 1..4 = increasing intensity bucket.
    private func level(_ cost: Double, _ t: (q1: Double, q2: Double, q3: Double)) -> Int {
        if cost <= 0 { return 0 }
        if cost <= t.q1 { return 1 }
        if cost <= t.q2 { return 2 }
        if cost <= t.q3 { return 3 }
        return 4
    }

    private func color(_ level: Int) -> Color {
        switch level {
        case 1: return Color(red: 0.95, green: 0.62, blue: 0.24).opacity(0.30) // light amber
        case 2: return Color(red: 0.95, green: 0.58, blue: 0.22).opacity(0.55)
        case 3: return Color(red: 0.93, green: 0.45, blue: 0.18).opacity(0.80) // deeper
        case 4: return Color(red: 0.86, green: 0.27, blue: 0.10)               // ember, full
        default: return Color.secondary.opacity(0.15)                          // empty
        }
    }

    private var weeks: [[DayPoint]] {
        stride(from: 0, to: days.count, by: 7).map { Array(days[$0..<min($0+7, days.count)]) }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(hovered.flatMap { key in days.first { $0.date == key }.map { "\(pretty($0.date)) · \(Formatters.cost($0.cost))" } } ?? "last 12 weeks")
                .font(.caption2).foregroundStyle(hovered == nil ? .secondary : .primary)
            let t = thresholds
            HStack(spacing: 3) {
                ForEach(Array(weeks.enumerated()), id: \.offset) { _, week in
                    VStack(spacing: 3) {
                        ForEach(week, id: \.date) { d in
                            RoundedRectangle(cornerRadius: 2)
                                .fill(color(level(d.cost, t)))
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
