import Foundation
import SwiftUI
import WebKit

public struct BrowserSearchResponse: Decodable, Equatable {
    public let browser: String
    public let query: String
    public let results: [BrowserSearchResult]

    public init(browser: String, query: String, results: [BrowserSearchResult]) {
        self.browser = browser
        self.query = query
        self.results = results
    }
}

public struct BrowserSearchResult: Decodable, Equatable, Identifiable {
    public var id: String { imageTag }

    public let browserName: String
    public let browserVersion: String
    public let imageTag: String
    public let repository: String
    public let tag: String
    public let driverVersion: String?
    public let gridVersion: String?
    public let releaseDate: String?
    public let platforms: [String]
    public let warnings: [String]?
    public let recommended: Bool

    public init(
        browserName: String,
        browserVersion: String,
        imageTag: String,
        repository: String,
        tag: String,
        driverVersion: String?,
        gridVersion: String?,
        releaseDate: String?,
        platforms: [String],
        warnings: [String]?,
        recommended: Bool
    ) {
        self.browserName = browserName
        self.browserVersion = browserVersion
        self.imageTag = imageTag
        self.repository = repository
        self.tag = tag
        self.driverVersion = driverVersion
        self.gridVersion = gridVersion
        self.releaseDate = releaseDate
        self.platforms = platforms
        self.warnings = warnings
        self.recommended = recommended
    }
}

public protocol BrowserVersionSearching {
    func searchChromeVersions(query: String) async throws -> BrowserSearchResponse
}

public struct InstalledBrowsersResponse: Decodable, Equatable {
    public let browsers: [InstalledBrowserRecord]

    public init(browsers: [InstalledBrowserRecord]) {
        self.browsers = browsers
    }
}

public struct InstalledBrowserRecord: Decodable, Equatable, Identifiable {
    public var id: String { imageTag }

    public let family: String
    public let version: String
    public let imageTag: String
    public let platform: String
    public let source: String
    public let enabled: Bool

    public init(family: String, version: String, imageTag: String, platform: String, source: String, enabled: Bool) {
        self.family = family
        self.version = version
        self.imageTag = imageTag
        self.platform = platform
        self.source = source
        self.enabled = enabled
    }
}

public struct BrowserInstallProgress: Decodable, Equatable {
    public let stage: String
    public let message: String

    public init(stage: String, message: String) {
        self.stage = stage
        self.message = message
    }
}

public struct BrowserInstallResponse: Decodable, Equatable {
    public let record: InstalledBrowserRecord
    public let alreadyInstalled: Bool
    public let progress: [BrowserInstallProgress]
    public let warnings: [String]

    public init(record: InstalledBrowserRecord, alreadyInstalled: Bool, progress: [BrowserInstallProgress], warnings: [String]) {
        self.record = record
        self.alreadyInstalled = alreadyInstalled
        self.progress = progress
        self.warnings = warnings
    }

    private enum CodingKeys: String, CodingKey {
        case record
        case alreadyInstalled
        case progress
        case warnings
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        record = try container.decode(InstalledBrowserRecord.self, forKey: .record)
        alreadyInstalled = try container.decode(Bool.self, forKey: .alreadyInstalled)
        progress = try container.decodeIfPresent([BrowserInstallProgress].self, forKey: .progress) ?? []
        warnings = try container.decodeIfPresent([String].self, forKey: .warnings) ?? []
    }
}

public protocol BrowserVersionInstalling {
    func installChrome(result: BrowserSearchResult) async throws -> BrowserInstallResponse
}

public protocol InstalledBrowserListing {
    func listInstalledBrowsers() async throws -> InstalledBrowsersResponse
}

public struct NoVNCResolution: Decodable, Equatable {
    public let url: String?
    public let vncWebSocketUrl: String?
    public let vncLocalAddress: String?
    public let gridSessionUrl: String?

    public init(url: String?, vncWebSocketUrl: String?, vncLocalAddress: String?, gridSessionUrl: String?) {
        self.url = url
        self.vncWebSocketUrl = vncWebSocketUrl
        self.vncLocalAddress = vncLocalAddress
        self.gridSessionUrl = gridSessionUrl
    }
}

public struct ManualSessionResponse: Decodable, Equatable {
    public let sessionId: String
    public let browserName: String
    public let browserVersion: String
    public let requestedUrl: String
    public let currentUrl: String?
    public let title: String?
    public let gridUrl: String
    public let webdriverEndpoint: String
    public let noVnc: NoVNCResolution
    public let startedAt: String

    public init(
        sessionId: String,
        browserName: String,
        browserVersion: String,
        requestedUrl: String,
        currentUrl: String?,
        title: String?,
        gridUrl: String,
        webdriverEndpoint: String,
        noVnc: NoVNCResolution,
        startedAt: String
    ) {
        self.sessionId = sessionId
        self.browserName = browserName
        self.browserVersion = browserVersion
        self.requestedUrl = requestedUrl
        self.currentUrl = currentUrl
        self.title = title
        self.gridUrl = gridUrl
        self.webdriverEndpoint = webdriverEndpoint
        self.noVnc = noVnc
        self.startedAt = startedAt
    }
}

public protocol ManualSessionOpening {
    func openManualSession(browser: InstalledBrowserRecord, url: String?) async throws -> ManualSessionResponse
}

public final class URLSessionBrowserSearchClient: BrowserVersionSearching, BrowserVersionInstalling, InstalledBrowserListing, ManualSessionOpening {
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

    public func searchChromeVersions(query: String) async throws -> BrowserSearchResponse {
        let token = try readToken()

        var components = URLComponents(
            url: baseURL
                .appendingPathComponent("v1")
                .appendingPathComponent("browsers")
                .appendingPathComponent("search"),
            resolvingAgainstBaseURL: false
        )!
        components.queryItems = [
            URLQueryItem(name: "browser", value: "chrome"),
            URLQueryItem(name: "q", value: query)
        ]

        var request = URLRequest(url: components.url!)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw DaemonStatusClientError.badHTTPStatus(-1)
        }
        guard httpResponse.statusCode == 200 else {
            throw DaemonStatusClientError.badHTTPStatus(httpResponse.statusCode)
        }
        return try JSONDecoder().decode(BrowserSearchResponse.self, from: data)
    }

    public func installChrome(result: BrowserSearchResult) async throws -> BrowserInstallResponse {
        let token = try readToken()
        let body = BrowserInstallRequest(result: result)
        var request = URLRequest(
            url: baseURL
                .appendingPathComponent("v1")
                .appendingPathComponent("browsers")
                .appendingPathComponent("install")
        )
        request.httpMethod = "POST"
        request.httpBody = try JSONEncoder().encode(body)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw DaemonStatusClientError.badHTTPStatus(-1)
        }
        guard httpResponse.statusCode == 200 else {
            throw DaemonStatusClientError.badHTTPStatus(httpResponse.statusCode)
        }
        return try JSONDecoder().decode(BrowserInstallResponse.self, from: data)
    }

    public func listInstalledBrowsers() async throws -> InstalledBrowsersResponse {
        let token = try readToken()
        var request = URLRequest(
            url: baseURL
                .appendingPathComponent("v1")
                .appendingPathComponent("browsers")
                .appendingPathComponent("installed")
        )
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw DaemonStatusClientError.badHTTPStatus(-1)
        }
        guard httpResponse.statusCode == 200 else {
            throw DaemonStatusClientError.badHTTPStatus(httpResponse.statusCode)
        }
        return try JSONDecoder().decode(InstalledBrowsersResponse.self, from: data)
    }

    public func openManualSession(browser: InstalledBrowserRecord, url: String?) async throws -> ManualSessionResponse {
        let token = try readToken()
        let body = ManualSessionRequest(browser: browser, url: url)
        var request = URLRequest(
            url: baseURL
                .appendingPathComponent("v1")
                .appendingPathComponent("sessions")
                .appendingPathComponent("manual")
        )
        request.httpMethod = "POST"
        request.httpBody = try JSONEncoder().encode(body)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw DaemonStatusClientError.badHTTPStatus(-1)
        }
        guard httpResponse.statusCode == 200 else {
            throw DaemonStatusClientError.badHTTPStatus(httpResponse.statusCode)
        }
        return try JSONDecoder().decode(ManualSessionResponse.self, from: data)
    }

    private func readToken() throws -> String {
        do {
            return try String(contentsOf: tokenFile, encoding: .utf8)
                .trimmingCharacters(in: .whitespacesAndNewlines)
        } catch {
            throw DaemonStatusClientError.tokenMissing(tokenFile.path)
        }
    }
}

private struct BrowserInstallRequest: Encodable {
    let browserName: String
    let browserVersion: String
    let imageTag: String
    let repository: String
    let tag: String
    let platforms: [String]

    init(result: BrowserSearchResult) {
        browserName = result.browserName
        browserVersion = result.browserVersion
        imageTag = result.imageTag
        repository = result.repository
        tag = result.tag
        platforms = result.platforms
    }
}

private struct ManualSessionRequest: Encodable {
    let browserName: String
    let browserVersion: String
    let url: String?

    init(browser: InstalledBrowserRecord, url: String?) {
        browserName = browser.family
        browserVersion = browser.version
        self.url = url
    }
}

public struct BrowserSearchRow: Equatable, Identifiable {
    public let id: String
    public let title: String
    public let detail: String
    public let recommended: Bool
    public let result: BrowserSearchResult
}

public struct InstalledBrowserRow: Equatable, Identifiable {
    public let id: String
    public let title: String
    public let detail: String
    public let enabled: Bool
    public let record: InstalledBrowserRecord
}

@MainActor
public final class BrowserSearchViewModel: ObservableObject {
    @Published public var query: String = ""
    @Published public private(set) var rows: [BrowserSearchRow] = []
    @Published public private(set) var installedRows: [InstalledBrowserRow] = []
    @Published public private(set) var isSearching: Bool = false
    @Published public private(set) var installingImageTag: String?
    @Published public private(set) var openingImageTag: String?
    @Published public var targetURL: String = ""
    @Published public private(set) var activeSession: ManualSessionResponse?
    @Published public private(set) var installMessages: [String: String] = [:]
    @Published public private(set) var errorMessage: String?

    private let client: BrowserVersionSearching
    private let installer: BrowserVersionInstalling?
    private let lister: InstalledBrowserListing?
    private let sessionOpener: ManualSessionOpening?

    public init(
        client: BrowserVersionSearching,
        installer: BrowserVersionInstalling? = nil,
        lister: InstalledBrowserListing? = nil,
        sessionOpener: ManualSessionOpening? = nil
    ) {
        self.client = client
        self.installer = installer
        self.lister = lister
        self.sessionOpener = sessionOpener
    }

    public func search(query: String? = nil) async {
        let searchQuery = query ?? self.query
        isSearching = true
        errorMessage = nil
        defer { isSearching = false }

        do {
            let response = try await client.searchChromeVersions(query: searchQuery)
            rows = response.results.map(Self.row)
        } catch let error as URLError where error.code == .cannotConnectToHost || error.code == .notConnectedToInternet {
            rows = []
            errorMessage = "Daemon stopped"
        } catch {
            rows = []
            errorMessage = error.localizedDescription
        }
    }

    public func refreshInstalledBrowsers() async {
        guard let lister else { return }
        do {
            installedRows = try await lister.listInstalledBrowsers().browsers.map(Self.installedRow)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    public func install(result: BrowserSearchResult) async {
        guard let installer else { return }
        installingImageTag = result.imageTag
        installMessages[result.imageTag] = "Pulling \(result.imageTag)"
        defer { installingImageTag = nil }

        do {
            let response = try await installer.installChrome(result: result)
            var messages: [String] = []
            if response.alreadyInstalled {
                messages.append("Chrome \(response.record.version) already installed")
            } else {
                messages.append("Installed Chrome \(response.record.version)")
            }
            messages.append(contentsOf: response.progress.map(\.message).filter { !$0.isEmpty })
            messages.append(contentsOf: response.warnings.map { "Warning: \($0)" })
            installMessages[result.imageTag] = messages.joined(separator: "\n")
            await refreshInstalledBrowsers()
        } catch {
            installMessages[result.imageTag] = error.localizedDescription
        }
    }

    public func openManualSession(record: InstalledBrowserRecord) async {
        guard let sessionOpener else { return }
        openingImageTag = record.imageTag
        errorMessage = nil
        defer { openingImageTag = nil }

        let trimmedURL = targetURL.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            activeSession = try await sessionOpener.openManualSession(
                browser: record,
                url: trimmedURL.isEmpty ? nil : trimmedURL
            )
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    public var activeNoVNCURL: URL? {
        guard let raw = activeSession?.noVnc.url else { return nil }
        return URL(string: raw)
    }

    private static func row(for result: BrowserSearchResult) -> BrowserSearchRow {
        var details = ["Image: \(result.imageTag)"]
        var metadata: [String] = []
        if let driverVersion = result.driverVersion, !driverVersion.isEmpty {
            metadata.append("Driver \(driverVersion)")
        }
        if let gridVersion = result.gridVersion, !gridVersion.isEmpty {
            metadata.append("Grid \(gridVersion)")
        }
        if let releaseDate = result.releaseDate, !releaseDate.isEmpty {
            metadata.append("Released \(releaseDate)")
        }
        if !metadata.isEmpty {
            details.append(metadata.joined(separator: "  "))
        }
        if !result.platforms.isEmpty {
            details.append("Platforms: \(result.platforms.joined(separator: ", "))")
        }
        for warning in result.warnings ?? [] {
            details.append("Warning: \(warning)")
        }

        return BrowserSearchRow(
            id: result.imageTag,
            title: "Chrome \(result.browserVersion)",
            detail: details.joined(separator: "\n"),
            recommended: result.recommended,
            result: result
        )
    }

    private static func installedRow(for record: InstalledBrowserRecord) -> InstalledBrowserRow {
        InstalledBrowserRow(
            id: record.imageTag,
            title: "Chrome \(record.version)",
            detail: [
                "Image: \(record.imageTag)",
                "Platform: \(record.platform)",
                "Source: \(record.source)"
            ].joined(separator: "\n"),
            enabled: record.enabled,
            record: record
        )
    }
}

public struct BrowserSearchView: View {
    @StateObject private var viewModel: BrowserSearchViewModel

    public init(viewModel: BrowserSearchViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                TextField("Chrome version", text: $viewModel.query)
                    .textFieldStyle(.roundedBorder)
                Button("Search") {
                    Task {
                        await viewModel.search()
                    }
                }
                .disabled(viewModel.isSearching)
            }
            if let errorMessage = viewModel.errorMessage {
                Text(errorMessage)
                    .foregroundStyle(.red)
            }
            List(viewModel.rows) { row in
                VStack(alignment: .leading, spacing: 4) {
                    HStack {
                        Text(row.title)
                            .font(.headline)
                        if row.recommended {
                            Text("Recommended")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        Spacer()
                        Button(viewModel.installingImageTag == row.id ? "Installing" : "Install") {
                            Task {
                                await viewModel.install(result: row.result)
                            }
                        }
                        .disabled(viewModel.installingImageTag == row.id)
                    }
                    Text(row.detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                    if let message = viewModel.installMessages[row.id] {
                        Text(message)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                    }
                }
                .padding(.vertical, 4)
            }
            .frame(minHeight: 180)
            if !viewModel.installedRows.isEmpty {
                Text("Installed Browsers")
                    .font(.headline)
                TextField("URL", text: $viewModel.targetURL)
                    .textFieldStyle(.roundedBorder)
                List(viewModel.installedRows) { row in
                    VStack(alignment: .leading, spacing: 4) {
                        HStack {
                            Text(row.title)
                                .font(.headline)
                            Spacer()
                            Button(viewModel.openingImageTag == row.id ? "Opening" : "Open") {
                                Task {
                                    await viewModel.openManualSession(record: row.record)
                                }
                            }
                            .disabled(viewModel.openingImageTag == row.id)
                        }
                        Text(row.detail)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                    }
                    .padding(.vertical, 4)
                }
                .frame(minHeight: 120)
            }
            if let session = viewModel.activeSession {
                VStack(alignment: .leading, spacing: 8) {
                    HStack {
                        Text("Session \(session.sessionId)")
                            .font(.headline)
                        Spacer()
                        if let url = viewModel.activeNoVNCURL {
                            Link("Open External", destination: url)
                        }
                    }
                    if let currentURL = session.currentUrl, !currentURL.isEmpty {
                        Text(currentURL)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                    }
                    if let url = viewModel.activeNoVNCURL {
                        NoVNCWebView(url: url)
                            .frame(minHeight: 260)
                    }
                }
            }
        }
        .task {
            await viewModel.refreshInstalledBrowsers()
        }
    }
}

public struct NoVNCWebView: NSViewRepresentable {
    public let url: URL

    public init(url: URL) {
        self.url = url
    }

    public func makeNSView(context: Context) -> WKWebView {
        WKWebView()
    }

    public func updateNSView(_ webView: WKWebView, context: Context) {
        if webView.url != url {
            webView.load(URLRequest(url: url))
        }
    }
}
