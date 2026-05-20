import Foundation
import SwiftUI

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

public final class URLSessionBrowserSearchClient: BrowserVersionSearching {
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
        let token: String
        do {
            token = try String(contentsOf: tokenFile, encoding: .utf8)
                .trimmingCharacters(in: .whitespacesAndNewlines)
        } catch {
            throw DaemonStatusClientError.tokenMissing(tokenFile.path)
        }

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
}

public struct BrowserSearchRow: Equatable, Identifiable {
    public let id: String
    public let title: String
    public let detail: String
    public let recommended: Bool
}

@MainActor
public final class BrowserSearchViewModel: ObservableObject {
    @Published public var query: String = ""
    @Published public private(set) var rows: [BrowserSearchRow] = []
    @Published public private(set) var isSearching: Bool = false
    @Published public private(set) var errorMessage: String?

    private let client: BrowserVersionSearching

    public init(client: BrowserVersionSearching) {
        self.client = client
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
            recommended: result.recommended
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
                    }
                    Text(row.detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                }
                .padding(.vertical, 4)
            }
            .frame(minHeight: 180)
        }
    }
}
