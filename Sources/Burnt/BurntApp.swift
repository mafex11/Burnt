import SwiftUI
import BurntCore

/// The menu bar label: an animated pixel-art flame + the value text.
/// The flame frame is driven by `frame`; the same `Image(nsImage:)` updates live
/// when the observed frame index changes.
struct MenuBarLabel: View {
    let text: String
    let frame: Int

    var body: some View {
        let flame = Image(nsImage: PixelFlame.image(frame: frame))
        if text.isEmpty {
            flame
        } else {
            HStack(spacing: 4) {
                flame
                Text(text)
            }
        }
    }
}

@main
struct BurntApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuBarRootView(model: model)
        } label: {
            MenuBarLabel(text: model.menuBarText, frame: model.flameFrame)
                .onAppear { model.startAutoRefresh() }
        }
        .menuBarExtraStyle(.window)
    }
}
