import SwiftUI
import BurntCore

/// The menu bar label: an animated pixel-art flame + the value text.
/// Observes the `FlameAnimator` directly so a frame tick re-renders only this
/// label; the `App` body (and the popover) only update when usage data changes.
struct MenuBarLabel: View {
    let text: String
    @ObservedObject var flameAnimator: FlameAnimator

    var body: some View {
        let flame = Image(nsImage: PixelFlame.image(frame: flameAnimator.frame))
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
            MenuBarLabel(text: model.menuBarText, flameAnimator: model.flame)
                .onAppear { model.startAutoRefresh() }
        }
        .menuBarExtraStyle(.window)
    }
}
