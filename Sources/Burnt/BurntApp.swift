import SwiftUI

@main
struct BurntApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuBarView(model: model)
        } label: {
            // SF Symbol flame + the dollar amount. The symbol is a template image,
            // so it auto-adapts to light/dark menu bars and the system tint.
            Label(model.menuBarText, systemImage: "flame.fill")
                .onAppear { model.startAutoRefresh() }   // kick off the 60s live poll
        }
        .menuBarExtraStyle(.window)
    }
}
