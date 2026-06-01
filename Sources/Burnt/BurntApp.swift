import SwiftUI

@main
struct BurntApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuBarRootView(model: model)
        } label: {
            // A Label renders icon-only in the menu bar; inline the symbol into Text
            // so both the flame and the value show. Empty text = icon-only mode.
            Text(model.menuBarText.isEmpty
                 ? "\(Image(systemName: "flame.fill"))"
                 : "\(Image(systemName: "flame.fill"))  \(model.menuBarText)")
                .onAppear { model.startAutoRefresh() }
        }
        .menuBarExtraStyle(.window)
    }
}
