import SwiftUI

@main
struct BurntApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuBarRootView(model: model)
        } label: {
            Label(model.menuBarText, systemImage: "flame.fill")
                .onAppear { model.startAutoRefresh() }
        }
        .menuBarExtraStyle(.window)
    }
}
