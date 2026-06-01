import SwiftUI

@main
struct BurntApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuBarView(model: model)
        } label: {
            Text("◔ \(model.menuBarText)")
                .onAppear { model.startAutoRefresh() }   // kick off the 60s live poll
        }
        .menuBarExtraStyle(.window)
    }
}
