import XCTest
@testable import BrowserLabApp

final class DaemonLifecycleViewModelTests: XCTestCase {
    func testStartStopRestartAndLogsCallLifecycleClient() async {
        let client = FakeLifecycleClient()
        let viewModel = await DaemonLifecycleViewModel(client: client)

        await viewModel.start()
        await viewModel.stop()
        await viewModel.restart()
        await viewModel.loadLogs()

        let actions = await client.actions
        XCTAssertEqual(actions, [.start, .stop, .restart, .logs])

        let state = await viewModel.state
        XCTAssertEqual(state, .succeeded("Loaded daemon logs"))
        let logs = await viewModel.logs
        XCTAssertEqual(logs, "daemon log text")
    }

    func testLifecycleFailureIsVisible() async {
        let client = FakeLifecycleClient(error: DaemonLifecycleClientError.commandFailed("launchctl bootstrap failed"))
        let viewModel = await DaemonLifecycleViewModel(client: client)

        await viewModel.start()

        let state = await viewModel.state
        guard case .failed(let message) = state else {
            return XCTFail("state = \(state), want failed")
        }
        XCTAssertTrue(message.contains("launchctl bootstrap failed"))
    }
}

private actor FakeLifecycleClient: DaemonLifecycleControlling {
    enum Action: Equatable {
        case start
        case stop
        case restart
        case logs
    }

    private(set) var actions: [Action] = []
    let error: Error?

    init(error: Error? = nil) {
        self.error = error
    }

    func start() async throws {
        actions.append(.start)
        if let error {
            throw error
        }
    }

    func stop() async throws {
        actions.append(.stop)
        if let error {
            throw error
        }
    }

    func restart() async throws {
        actions.append(.restart)
        if let error {
            throw error
        }
    }

    func logs() async throws -> String {
        actions.append(.logs)
        if let error {
            throw error
        }
        return "daemon log text"
    }
}
