import XCTest
@testable import BrowserLabApp

final class DaemonStatusViewModelTests: XCTestCase {
    func testRefreshShowsRunningStatusFromDaemonAPI() async {
        let status = DaemonStatus(
            state: "running",
            service: "browserlabd",
            version: "test-version",
            api: .init(bind: "127.0.0.1:49321", localhostOnly: true),
            paths: nil
        )
        let viewModel = await DaemonStatusViewModel(client: FakeStatusClient(result: .success(status)))

        await viewModel.refresh()

        let displayState = await viewModel.displayState
        XCTAssertEqual(displayState, .running(status))
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
