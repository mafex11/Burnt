import SwiftUI

/// The menu bar label. With text, inline the flame into a Text so both render
/// (a plain Label collapses to icon-only in the menu bar). Empty text → just the
/// SF Symbol as a proper white template image (an empty Text-with-image renders
/// as an ugly dark blob, so we use Image directly for icon-only mode).
struct MenuBarLabel: View {
    let text: String
    var body: some View {
        if text.isEmpty {
            Image(systemName: "flame.fill")
        } else {
            Text("\(Image(systemName: "flame.fill"))  \(text)")
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
            MenuBarLabel(text: model.menuBarText)
                .onAppear { model.startAutoRefresh() }
        }
        .menuBarExtraStyle(.window)
    }
}
