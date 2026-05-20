import Foundation
import SwiftUI

public struct DaemonStatus: Decodable, Equatable {
    public let state: String
    public let service: String
    public let version: String?
    public let api: APIStatus
    public let paths: PathStatus?
    public let nativeRuntimes: [NativeRuntimeStatus]

    public init(
        state: String,
        service: String,
        version: String?,
        api: APIStatus,
        paths: PathStatus?,
        nativeRuntimes: [NativeRuntimeStatus] = []
    ) {
        self.state = state
        self.service = service
        self.version = version
        self.api = api
        self.paths = paths
        self.nativeRuntimes = nativeRuntimes
    }

    private enum CodingKeys: String, CodingKey {
        case state
        case service
        case version
        case api
        case paths
        case nativeRuntimes
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        state = try container.decode(String.self, forKey: .state)
        service = try container.decode(String.self, forKey: .service)
        version = try container.decodeIfPresent(String.self, forKey: .version)
        api = try container.decode(APIStatus.self, forKey: .api)
        paths = try container.decodeIfPresent(PathStatus.self, forKey: .paths)
        nativeRuntimes = try container.decodeIfPresent([NativeRuntimeStatus].self, forKey: .nativeRuntimes) ?? []
    }

    public var nativeRuntimeDisplays: [NativeRuntimeDisplay] {
        nativeRuntimes.map(NativeRuntimeDisplay.init(runtime:))
    }

    public struct APIStatus: Decodable, Equatable {
        public let bind: String
        public let localhostOnly: Bool
    }

    public struct PathStatus: Decodable, Equatable {
        public let configDir: String
        public let logsDir: String
        public let artifactsDir: String
    }
}

public struct NativeRuntimeStatus: Decodable, Equatable, Identifiable {
    public let id: String
    public let displayName: String
    public let kind: String
    public let installable: Bool
    public let status: String
    public let browserAvailable: Bool
    public let browserVersion: String?
    public let driverAvailable: Bool
    public let driverVersion: String?
    public let message: String
    public let setupGuidance: [String]
    public let outOfScope: String
}

public struct NativeRuntimeDisplay: Equatable, Identifiable {
    public let id: String
    public let title: String
    public let subtitle: String
    public let details: [String]

    public init(title: String, subtitle: String, details: [String]) {
        self.id = title
        self.title = title
        self.subtitle = subtitle
        self.details = details
    }

    public init(runtime: NativeRuntimeStatus) {
        let installability = runtime.installable ? "installable" : "non-installable"
        var details: [String] = []
        if let browserVersion = runtime.browserVersion, !browserVersion.isEmpty {
            details.append("\(runtime.displayName) version: \(browserVersion)")
        }
        if let driverVersion = runtime.driverVersion, !driverVersion.isEmpty {
            details.append("SafariDriver: \(driverVersion)")
        }
        if !runtime.message.isEmpty {
            details.append(runtime.message)
        }
        details.append(contentsOf: runtime.setupGuidance.map { "Setup: \($0)" })
        if !runtime.outOfScope.isEmpty {
            details.append(runtime.outOfScope)
        }

        self.init(
            title: runtime.displayName,
            subtitle: "\(runtime.status) - \(runtime.kind) - \(installability)",
            details: details
        )
    }
}

public enum DaemonDisplayState: Equatable {
    case loading
    case running(DaemonStatus)
    case stopped
    case error(String)

    var title: String {
        switch self {
        case .loading:
            return "Checking daemon"
        case .running:
            return "Daemon running"
        case .stopped:
            return "Daemon stopped"
        case .error:
            return "Daemon error"
        }
    }

    var detail: String {
        switch self {
        case .loading:
            return "Contacting local BrowserLab service."
        case .running(let status):
            return "Listening on \(status.api.bind)"
        case .stopped:
            return "Start browserlabd to enable local controls."
        case .error(let message):
            return message
        }
    }
}

public protocol DaemonStatusFetching {
    func fetchStatus() async throws -> DaemonStatus
}

public enum DaemonStatusClientError: Error, Equatable {
    case tokenMissing(String)
    case badHTTPStatus(Int)
    case nonLocalDaemon(String)
    case apiProblem(String, String)
}

extension DaemonStatusClientError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .tokenMissing(let path):
            return "BrowserLab token is missing at \(path)"
        case .badHTTPStatus(let status):
            return "BrowserLab daemon returned HTTP \(status)"
        case .nonLocalDaemon(let bind):
            return "BrowserLab daemon is not localhost-only: \(bind)"
        case .apiProblem(let code, let message):
            return "\(code): \(message)"
        }
    }
}

public final class URLSessionDaemonStatusClient: DaemonStatusFetching {
    private let baseURL: URL
    private let tokenFile: URL
    private let session: URLSession

    public convenience init() {
        let paths = BrowserLabPaths.resolve()
        self.init(
            baseURL: URL(string: "http://127.0.0.1:49321")!,
            tokenFile: paths.tokenFile,
            session: .shared
        )
    }

    public init(baseURL: URL, tokenFile: URL, session: URLSession) {
        self.baseURL = baseURL
        self.tokenFile = tokenFile
        self.session = session
    }

    public func fetchStatus() async throws -> DaemonStatus {
        let token: String
        do {
            token = try String(contentsOf: tokenFile, encoding: .utf8)
                .trimmingCharacters(in: .whitespacesAndNewlines)
        } catch {
            throw DaemonStatusClientError.tokenMissing(tokenFile.path)
        }

        var request = URLRequest(url: baseURL.appendingPathComponent("v1/status"))
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw DaemonStatusClientError.badHTTPStatus(-1)
        }
        guard httpResponse.statusCode == 200 else {
            throw DaemonStatusClientError.badHTTPStatus(httpResponse.statusCode)
        }

        let status = try JSONDecoder().decode(DaemonStatus.self, from: data)
        if !status.api.localhostOnly {
            throw DaemonStatusClientError.nonLocalDaemon(status.api.bind)
        }
        return status
    }
}

public struct BrowserLabPaths {
    public let root: URL
    public let tokenFile: URL

    public static func resolve(environment: [String: String] = ProcessInfo.processInfo.environment) -> BrowserLabPaths {
        if let override = environment["BROWSERLAB_HOME"], !override.isEmpty {
            let root = URL(fileURLWithPath: override)
            return BrowserLabPaths(root: root, tokenFile: root.appendingPathComponent("config/token"))
        }

        let home = FileManager.default.homeDirectoryForCurrentUser
        let root = home
            .appendingPathComponent("Library")
            .appendingPathComponent("Application Support")
            .appendingPathComponent("BrowserLab")
        return BrowserLabPaths(root: root, tokenFile: root.appendingPathComponent("config/token"))
    }
}

@MainActor
public final class DaemonStatusViewModel: ObservableObject {
    @Published public private(set) var displayState: DaemonDisplayState = .loading
    private let client: DaemonStatusFetching

    public init(client: DaemonStatusFetching) {
        self.client = client
    }

    public func refresh() async {
        do {
            let status = try await client.fetchStatus()
            displayState = .running(status)
        } catch let error as URLError where error.code == .cannotConnectToHost || error.code == .notConnectedToInternet {
            displayState = .stopped
        } catch {
            displayState = .error(error.localizedDescription)
        }
    }
}

public struct DaemonStatusView: View {
    @StateObject private var viewModel: DaemonStatusViewModel

    public init(viewModel: DaemonStatusViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Circle()
                    .fill(indicatorColor)
                    .frame(width: 10, height: 10)
                Text(viewModel.displayState.title)
                    .font(.headline)
            }
            Text(viewModel.displayState.detail)
                .foregroundStyle(.secondary)
            if case .running(let status) = viewModel.displayState, !status.nativeRuntimeDisplays.isEmpty {
                Divider()
                VStack(alignment: .leading, spacing: 8) {
                    Text("Native runtimes")
                        .font(.subheadline)
                        .fontWeight(.semibold)
                    ForEach(status.nativeRuntimeDisplays) { runtime in
                        VStack(alignment: .leading, spacing: 4) {
                            Text(runtime.title)
                                .font(.body)
                                .fontWeight(.medium)
                            Text(runtime.subtitle)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            ForEach(runtime.details, id: \.self) { detail in
                                Text(detail)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
            Button("Refresh") {
                Task {
                    await viewModel.refresh()
                }
            }
        }
        .padding(24)
        .task {
            await viewModel.refresh()
        }
    }

    private var indicatorColor: Color {
        switch viewModel.displayState {
        case .loading:
            return .gray
        case .running:
            return .green
        case .stopped:
            return .orange
        case .error:
            return .red
        }
    }
}
