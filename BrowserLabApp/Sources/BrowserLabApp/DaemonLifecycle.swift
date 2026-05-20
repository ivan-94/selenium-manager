import Foundation
import SwiftUI

public enum DaemonLifecycleClientError: Error, Equatable, LocalizedError {
    case commandFailed(String)

    public var errorDescription: String? {
        switch self {
        case .commandFailed(let message):
            return message
        }
    }
}

public protocol DaemonLifecycleControlling {
    func start() async throws
    func stop() async throws
    func restart() async throws
    func logs() async throws -> String
}

public final class BrowserLabCLIDaemonLifecycleClient: DaemonLifecycleControlling {
    private let executableURL: URL
    private let baseArguments: [String]

    public convenience init() {
        self.init(executableURL: URL(fileURLWithPath: "/usr/bin/env"), baseArguments: ["browserlab"])
    }

    public init(executableURL: URL, baseArguments: [String] = []) {
        self.executableURL = executableURL
        self.baseArguments = baseArguments
    }

    public func start() async throws {
        _ = try await runDaemonCommand("start")
    }

    public func stop() async throws {
        _ = try await runDaemonCommand("stop")
    }

    public func restart() async throws {
        _ = try await runDaemonCommand("restart")
    }

    public func logs() async throws -> String {
        try await runDaemonCommand("logs")
    }

    private func runDaemonCommand(_ command: String) async throws -> String {
        try await Task.detached(priority: .userInitiated) { [executableURL, baseArguments] in
            let process = Process()
            process.executableURL = executableURL
            process.arguments = baseArguments + ["daemon", command]

            let output = Pipe()
            let error = Pipe()
            process.standardOutput = output
            process.standardError = error

            try process.run()
            process.waitUntilExit()

            let stdout = String(data: output.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
            let stderr = String(data: error.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
            if process.terminationStatus != 0 {
                let message = (stderr.isEmpty ? stdout : stderr).trimmingCharacters(in: .whitespacesAndNewlines)
                throw DaemonLifecycleClientError.commandFailed(message.isEmpty ? "browserlab daemon \(command) failed" : message)
            }
            return stdout
        }.value
    }
}

public enum DaemonLifecycleState: Equatable {
    case idle
    case running(String)
    case succeeded(String)
    case failed(String)
}

@MainActor
public final class DaemonLifecycleViewModel: ObservableObject {
    @Published public private(set) var state: DaemonLifecycleState = .idle
    @Published public private(set) var logs: String = ""

    private let client: DaemonLifecycleControlling

    public init(client: DaemonLifecycleControlling) {
        self.client = client
    }

    public func start() async {
        await perform(inProgress: "Starting daemon", success: "Started daemon") {
            try await client.start()
        }
    }

    public func stop() async {
        await perform(inProgress: "Stopping daemon", success: "Stopped daemon") {
            try await client.stop()
        }
    }

    public func restart() async {
        await perform(inProgress: "Restarting daemon", success: "Restarted daemon") {
            try await client.restart()
        }
    }

    public func loadLogs() async {
        await perform(inProgress: "Loading daemon logs", success: "Loaded daemon logs") {
            logs = try await client.logs()
        }
    }

    private func perform(inProgress: String, success: String, action: () async throws -> Void) async {
        state = .running(inProgress)
        do {
            try await action()
            state = .succeeded(success)
        } catch {
            state = .failed(error.localizedDescription)
        }
    }
}

public struct DaemonLifecycleControlsView: View {
    @StateObject private var viewModel: DaemonLifecycleViewModel

    public init(viewModel: DaemonLifecycleViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Button("Start") {
                    Task { await viewModel.start() }
                }
                Button("Stop") {
                    Task { await viewModel.stop() }
                }
                Button("Restart") {
                    Task { await viewModel.restart() }
                }
                Button("Logs") {
                    Task { await viewModel.loadLogs() }
                }
            }

            if let message = stateMessage {
                Text(message)
                    .foregroundStyle(stateColor)
            }

            if !viewModel.logs.isEmpty {
                ScrollView {
                    Text(viewModel.logs)
                        .font(.system(.caption, design: .monospaced))
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .frame(minHeight: 100)
            }
        }
        .padding(.horizontal, 24)
        .padding(.bottom, 24)
    }

    private var stateMessage: String? {
        switch viewModel.state {
        case .idle:
            return nil
        case .running(let message), .succeeded(let message), .failed(let message):
            return message
        }
    }

    private var stateColor: Color {
        switch viewModel.state {
        case .idle, .running:
            return .secondary
        case .succeeded:
            return .green
        case .failed:
            return .red
        }
    }
}
