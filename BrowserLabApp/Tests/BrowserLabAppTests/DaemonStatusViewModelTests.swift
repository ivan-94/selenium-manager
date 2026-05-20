import XCTest
@testable import BrowserLabApp

final class DaemonStatusViewModelTests: XCTestCase {
    func testRefreshShowsRunningStatusFromDaemonAPI() async {
        let status = DaemonStatus(
            state: "running",
            service: "browserlabd",
            version: "test-version",
            api: .init(bind: "127.0.0.1:49321", localhostOnly: true),
            paths: nil,
            nativeRuntimes: []
        )
        let viewModel = await DaemonStatusViewModel(client: FakeStatusClient(result: .success(status)))

        await viewModel.refresh()

        let displayState = await viewModel.displayState
        XCTAssertEqual(displayState, .running(status))
    }

    func testNativeSafariRuntimeDisplaysSeparatelyFromDockerBrowsers() async {
        let safari = NativeRuntimeStatus(
            id: "safari",
            displayName: "Safari",
            kind: "native",
            installable: false,
            status: "setup_required",
            browserAvailable: true,
            browserVersion: "17.5",
            driverAvailable: false,
            driverVersion: nil,
            message: "Safari is installed, but SafariDriver is not available to BrowserLab.",
            setupGuidance: ["Enable Safari WebDriver support with: safaridriver --enable"],
            outOfScope: "Old Safari versions are out of scope for the Docker Selenium MVP. BrowserLab only detects the current macOS Safari and SafariDriver."
        )
        let status = DaemonStatus(
            state: "running",
            service: "browserlabd",
            version: "test-version",
            api: .init(bind: "127.0.0.1:49321", localhostOnly: true),
            paths: nil,
            nativeRuntimes: [safari]
        )

        XCTAssertEqual(status.nativeRuntimeDisplays, [
            NativeRuntimeDisplay(
                title: "Safari",
                subtitle: "setup_required - native - non-installable",
                details: [
                    "Safari version: 17.5",
                    "Safari is installed, but SafariDriver is not available to BrowserLab.",
                    "Setup: Enable Safari WebDriver support with: safaridriver --enable",
                    "Old Safari versions are out of scope for the Docker Selenium MVP. BrowserLab only detects the current macOS Safari and SafariDriver.",
                ]
            )
        ])
    }

    func testRefreshShowsStoppedWhenDaemonCannotBeReached() async {
        let viewModel = await DaemonStatusViewModel(client: FakeStatusClient(
            result: .failure(URLError(.cannotConnectToHost))
        ))

        await viewModel.refresh()

        let displayState = await viewModel.displayState
        XCTAssertEqual(displayState, .stopped)
    }

    func testRefreshShowsErrorForAuthenticatedDaemonFailures() async {
        let viewModel = await DaemonStatusViewModel(client: FakeStatusClient(
            result: .failure(DaemonStatusClientError.badHTTPStatus(401))
        ))

        await viewModel.refresh()

        let displayState = await viewModel.displayState
        guard case .error(let message) = displayState else {
            return XCTFail("displayState = \(displayState), want error")
        }
        XCTAssertFalse(message.isEmpty)
    }
}

private struct FakeStatusClient: DaemonStatusFetching {
    let result: Result<DaemonStatus, Error>

    func fetchStatus() async throws -> DaemonStatus {
        try result.get()
    }
}
