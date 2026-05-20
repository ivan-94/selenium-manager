import SwiftUI

@main
struct BrowserLabApplication: App {
    var body: some Scene {
        WindowGroup {
            VStack(alignment: .leading, spacing: 16) {
                DaemonStatusView(viewModel: DaemonStatusViewModel(
                    client: URLSessionDaemonStatusClient()
                ))
                BrowserSearchView(viewModel: BrowserSearchViewModel(
                    client: URLSessionBrowserSearchClient()
                ))
                .padding(.horizontal, 24)
                .padding(.bottom, 24)
            }
            .frame(minWidth: 640, minHeight: 460)
        }
    }
}
