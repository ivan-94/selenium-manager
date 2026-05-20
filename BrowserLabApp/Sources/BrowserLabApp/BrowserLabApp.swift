import SwiftUI

@main
struct BrowserLabApplication: App {
    var body: some Scene {
        WindowGroup {
            DaemonStatusView(viewModel: DaemonStatusViewModel(
                client: URLSessionDaemonStatusClient()
            ))
            .frame(minWidth: 360, minHeight: 180)
        }
    }
}
