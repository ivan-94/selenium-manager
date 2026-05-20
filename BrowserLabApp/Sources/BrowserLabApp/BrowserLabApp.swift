import SwiftUI

@main
struct BrowserLabApplication: App {
    var body: some Scene {
        WindowGroup {
            VStack(alignment: .leading, spacing: 16) {
                DaemonStatusView(viewModel: DaemonStatusViewModel(
                    client: URLSessionDaemonStatusClient()
                ))
                DaemonLifecycleControlsView(viewModel: DaemonLifecycleViewModel(
                    client: BrowserLabCLIDaemonLifecycleClient()
                ))
                let browserClient = URLSessionBrowserSearchClient()
                BrowserSearchView(viewModel: BrowserSearchViewModel(
                    client: browserClient,
                    installer: browserClient,
                    lister: browserClient,
                    disabler: browserClient,
                    uninstaller: browserClient
                ))
                .padding(.horizontal, 24)
                .padding(.bottom, 24)
            }
            .frame(minWidth: 640, minHeight: 460)
        }
    }
}
