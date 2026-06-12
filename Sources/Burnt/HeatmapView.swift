import SwiftUI
import AppKit
import UsageEngine
import BurntCore

/// GitHub-contribution-style grid of daily cost (84 days). Columns = weeks.
struct HeatmapView: View {
    let days: [DayPoint]          // oldest→newest, length 84
    @State private var hovered: String?

    // Absolute $50 bands across $0–$1000+ mapped onto a cool→hot ramp, so the
    // actual dollar amount drives the color — a $161 day and a $400 day read as
    // clearly different shades (unlike a max-relative or quantile scale, where
    // big days all collapse to one color).
    private static let bandSize: Double = 50    // $ per band
    private static let maxBands = 20            // caps the ramp at $1000

    /// Colour stops for the ramp, low spend → high spend.
    /// dim teal → green → amber → orange → red → deep ember.
    private static let ramp: [Color] = [
        Color(red: 0.20, green: 0.45, blue: 0.55),   // ~$0–50  cool
        Color(red: 0.24, green: 0.72, blue: 0.49),   // green
        Color(red: 0.55, green: 0.78, blue: 0.30),   // lime
        Color(red: 0.86, green: 0.78, blue: 0.25),   // yellow
        Color(red: 0.95, green: 0.62, blue: 0.24),   // amber
        Color(red: 0.93, green: 0.45, blue: 0.16),   // orange
        Color(red: 0.86, green: 0.27, blue: 0.10),   // red-ember
        Color(red: 0.65, green: 0.13, blue: 0.06),   // deep ember (~$1000+)
    ]

    private func color(for cost: Double) -> Color {
        if cost <= 0 { return Color.secondary.opacity(0.13) }
        // band index 0..maxBands by absolute dollars, then position along the ramp.
        let band = min(cost / Self.bandSize, Double(Self.maxBands))   // 0...20
        let t = band / Double(Self.maxBands)                          // 0...1
        return Self.rampColor(t)
    }

    /// Linearly interpolate the ramp at position t (0...1).
    private static func rampColor(_ t: Double) -> Color {
        let clamped = min(max(t, 0), 1)
        let scaled = clamped * Double(ramp.count - 1)
        let i = Int(scaled)
        if i >= ramp.count - 1 { return ramp[ramp.count - 1] }
        let frac = scaled - Double(i)
        return mix(ramp[i], ramp[i + 1], frac)
    }

    private static func mix(_ a: Color, _ b: Color, _ f: Double) -> Color {
        let ca = NSColor(a).usingColorSpace(.sRGB) ?? .gray
        let cb = NSColor(b).usingColorSpace(.sRGB) ?? .gray
        return Color(red: Double(ca.redComponent) + (Double(cb.redComponent) - Double(ca.redComponent)) * f,
                     green: Double(ca.greenComponent) + (Double(cb.greenComponent) - Double(ca.greenComponent)) * f,
                     blue: Double(ca.blueComponent) + (Double(cb.blueComponent) - Double(ca.blueComponent)) * f)
    }

    private var weeks: [[DayPoint]] {
        stride(from: 0, to: days.count, by: 7).map { Array(days[$0..<min($0+7, days.count)]) }
    }

    // Popover content is ~268pt wide (300 − padding); 12 columns + 11 gaps of 3pt
    // → cell ≈ 19.5pt. The grid is 7 rows tall, so reserve that height for the
    // GeometryReader (which otherwise collapses).
    private var gridHeight: CGFloat {
        let cols = max(weeks.count, 1)
        let gap: CGFloat = 3
        let assumedWidth: CGFloat = 268
        let cell = max(8, (assumedWidth - gap * CGFloat(cols - 1)) / CGFloat(cols))
        return cell * 7 + gap * 6
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(hovered.flatMap { key in days.first { $0.date == key }.map { "\(pretty($0.date)) · \(Formatters.cost($0.cost))" } } ?? "last 12 weeks")
                .font(.caption2).foregroundStyle(hovered == nil ? .secondary : .primary)
            // Cells sized from the available width so the grid spans the popover.
            GeometryReader { geo in
                let cols = weeks.count                       // 12
                let gap: CGFloat = 3
                let cell = max(8, (geo.size.width - gap * CGFloat(cols - 1)) / CGFloat(cols))
                HStack(spacing: gap) {
                    ForEach(Array(weeks.enumerated()), id: \.offset) { _, week in
                        VStack(spacing: gap) {
                            ForEach(week, id: \.date) { d in
                                RoundedRectangle(cornerRadius: 2)
                                    .fill(color(for: d.cost))
                                    .frame(width: cell, height: cell)
                                    .onHover { inside in hovered = inside ? d.date : (hovered == d.date ? nil : hovered) }
                            }
                        }
                    }
                }
            }
            .frame(height: gridHeight)
            // Legend: $0 → $1000+ ramp (spans full width).
            HStack(spacing: 4) {
                Text("$0").font(.system(size: 9)).foregroundStyle(.secondary)
                HStack(spacing: 2) {
                    ForEach(0..<10, id: \.self) { i in
                        RoundedRectangle(cornerRadius: 1)
                            .fill(Self.rampColor(Double(i) / 9.0))
                            .frame(maxWidth: .infinity)
                            .frame(height: 7)
                    }
                }
                Text("$1k+").font(.system(size: 9)).foregroundStyle(.secondary)
            }
            .padding(.top, 2)
        }
    }

    private func pretty(_ iso: String) -> String {
        let p = iso.split(separator: "-").compactMap { Int($0) }
        guard p.count == 3 else { return iso }
        let m = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
        return "\(p[1] >= 1 && p[1] <= 12 ? m[p[1]-1] : "?") \(p[2])"
    }
}
