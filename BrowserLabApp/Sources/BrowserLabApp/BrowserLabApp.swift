import SwiftUI

@main
struct BrowserLabApplication: App {
    var body: some Scene {
        WindowGroup {
            VStack(alignment: .leading, spacing: 0) {
                DaemonStatusView(viewModel: DaemonStatusViewModel(
                    client: URLSessionDaemonStatusClient()
                ))
                DaemonLifecycleControlsView(viewModel: DaemonLifecycleViewModel(
                    client: BrowserLabCLIDaemonLifecycleClient()
                ))
            }
            .frame(minWidth: 460, minHeight: 280)
        }
    }
}
