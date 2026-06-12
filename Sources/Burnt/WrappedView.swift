import SwiftUI
import AppKit
import BurntCore

/// The shareable Burnt Wrapped card. The SAME view renders on screen and to PNG.
struct WrappedView: View {
    let data: WrappedData

    private var amber: Color { Color(red: 0.95, green: 0.62, blue: 0.24) }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Burnt · \(data.title)")
                .font(.system(size: 15, weight: .semibold))
                .foregroundStyle(amber)
            Text(data.headlineCost)
                .font(.system(size: 54, weight: .bold)).monospacedDigit()
                .foregroundStyle(.white)
            Text("\(data.headlineTokens) tokens burnt")
                .font(.system(size: 16)).foregroundStyle(.white.opacity(0.7))

            VStack(alignment: .leading, spacing: 6) {
                ForEach(data.modelBars, id: \.name) { b in
                    HStack(spacing: 8) {
                        Text(b.name).font(.system(size: 12)).foregroundStyle(.white.opacity(0.85))
                            .frame(width: 150, alignment: .leading).lineLimit(1)
                        GeometryReader { geo in
                            Capsule().fill(amber)
                                .frame(width: max(4, geo.size.width * b.fraction), height: 8)
                        }.frame(height: 8)
                    }
                }
            }
            HStack(spacing: 20) {
                stat("Busiest day", "\(data.busiestDay) · \(data.busiestDayCost)")
                stat("Cache saved", data.cacheSaved)
            }
            Text("How much have you burnt?")
                .font(.system(size: 13)).foregroundStyle(.white.opacity(0.55))
        }
        .padding(28)
        .frame(width: 420)
        .background(
            LinearGradient(colors: [Color(red:0.16,green:0.10,blue:0.07), Color(red:0.05,green:0.05,blue:0.06)],
                           startPoint: .top, endPoint: .bottom)
        )
    }

    private func stat(_ k: String, _ v: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(k).font(.system(size: 11)).foregroundStyle(.white.opacity(0.55))
            Text(v).font(.system(size: 14, weight: .medium)).foregroundStyle(.white)
        }
    }
}
