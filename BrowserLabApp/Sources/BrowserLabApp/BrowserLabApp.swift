import SwiftUI

@main
struct BrowserLabApplication: App {
    var body: some Scene {
        WindowGroup("BrowserLab") {
            let browserClient = URLSessionBrowserSearchClient()
            BrowserLabWorkbenchView(
                daemonStatusViewModel: DaemonStatusViewModel(
                    client: URLSessionDaemonStatusClient()
                ),
                daemonLifecycleViewModel: DaemonLifecycleViewModel(
                    client: BrowserLabCLIDaemonLifecycleClient()
                ),
                browserViewModel: BrowserSearchViewModel(
                    client: browserClient,
                    installer: browserClient,
                    lister: browserClient,
                    mobilePresetLister: browserClient,
                    sessionOpener: browserClient,
                    disabler: browserClient,
                    uninstaller: browserClient,
                    sessionLister: browserClient,
                    sessionCloser: browserClient,
                    screenshotCapturer: browserClient
                )
            )
            .frame(minWidth: 1120, minHeight: 740)
        }
        .defaultSize(width: 1360, height: 860)
    }
}
