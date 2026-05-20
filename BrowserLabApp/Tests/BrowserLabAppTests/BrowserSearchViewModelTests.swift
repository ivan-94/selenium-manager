import XCTest
@testable import BrowserLabApp

final class BrowserSearchViewModelTests: XCTestCase {
    func testSearchRendersChromeVersionFirstWithImageDetails() async {
        let result = BrowserSearchResult(
            browserName: "chrome",
            browserVersion: "119.0",
            imageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
            repository: "selenium/standalone-chrome",
            tag: "119.0-chromedriver-119.0-grid-4.43.0-20260404",
            driverVersion: "119.0",
            gridVersion: "4.43.0",
            releaseDate: "20260404",
            platforms: ["linux/amd64"],
            warnings: ["Chrome Selenium images are linux/amd64 only; Apple Silicon will run this image through amd64 emulation."],
            recommended: true
        )
        let viewModel = await BrowserSearchViewModel(client: FakeBrowserSearchClient(
            response: .success(.init(browser: "chrome", query: "119", results: [result]))
        ))

        await viewModel.search(query: "119")

        let rows = await viewModel.rows
        XCTAssertEqual(rows.count, 1)
        XCTAssertEqual(rows[0].title, "Chrome 119.0")
        XCTAssertTrue(rows[0].detail.contains("selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"))
        XCTAssertTrue(rows[0].detail.contains("linux/amd64"))
        XCTAssertTrue(rows[0].detail.contains("Apple Silicon"))
    }

    func testInstallResultShowsProgressSuccessAndRefreshesInstalledBrowsers() async {
        let result = BrowserSearchResult(
            browserName: "chrome",
            browserVersion: "119.0",
            imageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
            repository: "selenium/standalone-chrome",
            tag: "119.0-chromedriver-119.0-grid-4.43.0-20260404",
            driverVersion: "119.0",
            gridVersion: "4.43.0",
            releaseDate: "20260404",
            platforms: ["linux/amd64"],
            warnings: nil,
            recommended: true
        )
        let installed = InstalledBrowserRecord(
            family: "chrome",
            version: "119.0",
            imageTag: result.imageTag,
            platform: "linux/amd64",
            source: "selenium-dockerhub",
            enabled: true
        )
        let manager = FakeBrowserManager(
            searchResponse: .success(.init(browser: "chrome", query: "119", results: [result])),
            installResponse: .success(.init(
                record: installed,
                alreadyInstalled: false,
                progress: [.init(stage: "pulling", message: "pulling \(result.imageTag)")],
                warnings: []
            )),
            installedResponse: .success(.init(browsers: [installed]))
        )
        let viewModel = await BrowserSearchViewModel(client: manager, installer: manager, lister: manager)

        await viewModel.search(query: "119")
        await viewModel.install(result: result)

        let rows = await viewModel.installedRows
        let installMessages = await viewModel.installMessages
        XCTAssertEqual(rows.map(\.title), ["Chrome 119.0"])
        XCTAssertTrue(installMessages[result.imageTag]?.contains("Installed Chrome 119.0") == true)
    }

    func testInstallPullErrorIsDisplayed() async {
        let result = BrowserSearchResult(
            browserName: "chrome",
            browserVersion: "119.0",
            imageTag: "selenium/standalone-chrome:missing",
            repository: "selenium/standalone-chrome",
            tag: "missing",
            driverVersion: nil,
            gridVersion: nil,
            releaseDate: nil,
            platforms: ["linux/amd64"],
            warnings: nil,
            recommended: true
        )
        let manager = FakeBrowserManager(
            searchResponse: .success(.init(browser: "chrome", query: "119", results: [result])),
            installResponse: .failure(NSError(domain: "BrowserLab", code: 1, userInfo: [NSLocalizedDescriptionKey: "pull_failed: manifest unknown"])),
            installedResponse: .success(.init(browsers: []))
        )
        let viewModel = await BrowserSearchViewModel(client: manager, installer: manager, lister: manager)

        await viewModel.install(result: result)

        let installMessages = await viewModel.installMessages
        XCTAssertTrue(installMessages[result.imageTag]?.contains("pull_failed") == true)
    }

    func testOpenManualSessionDefaultsBlankURLAndExposesNoVNCURL() async {
        let installed = InstalledBrowserRecord(
            family: "chrome",
            version: "119.0",
            imageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
            platform: "linux/amd64",
            source: "selenium-dockerhub",
            enabled: true
        )
        let opener = FakeManualSessionOpener(response: .success(.init(
            sessionId: "session-123",
            browserName: "chrome",
            browserVersion: "119.0",
            requestedUrl: "about:blank",
            currentUrl: "about:blank",
            title: nil,
            gridUrl: "http://127.0.0.1:4444",
            webdriverEndpoint: "http://127.0.0.1:4444/wd/hub",
            noVnc: .init(
                url: "http://127.0.0.1:4444/ui/#/sessions/session-123",
                vncWebSocketUrl: "ws://127.0.0.1:4444/session/session-123/se/vnc",
                vncLocalAddress: nil,
                gridSessionUrl: "http://127.0.0.1:4444/ui/#/sessions/session-123"
            ),
            startedAt: "2026-05-20T10:30:00Z"
        )))
        let manager = FakeBrowserManager(
            searchResponse: .success(.init(browser: "chrome", query: "", results: [])),
            installResponse: .failure(NSError(domain: "BrowserLab", code: 1)),
            installedResponse: .success(.init(browsers: [installed]))
        )
        let viewModel = await BrowserSearchViewModel(
            client: manager,
            lister: manager,
            sessionOpener: opener
        )

        await viewModel.refreshInstalledBrowsers()
        let rows = await viewModel.installedRows
        await viewModel.openManualSession(record: rows[0].record)

        let activeSession = await viewModel.activeSession
        let activeNoVNCURL = await viewModel.activeNoVNCURL
        XCTAssertEqual(opener.requestedURL, nil)
        XCTAssertEqual(activeSession?.sessionId, "session-123")
        XCTAssertEqual(activeSession?.requestedUrl, "about:blank")
        XCTAssertEqual(activeNoVNCURL?.absoluteString, "http://127.0.0.1:4444/ui/#/sessions/session-123")
    }
}

private struct FakeBrowserSearchClient: BrowserVersionSearching {
    let response: Result<BrowserSearchResponse, Error>

    func searchChromeVersions(query: String) async throws -> BrowserSearchResponse {
        try response.get()
    }
}

private struct FakeBrowserManager: BrowserVersionSearching, BrowserVersionInstalling, InstalledBrowserListing {
    let searchResponse: Result<BrowserSearchResponse, Error>
    let installResponse: Result<BrowserInstallResponse, Error>
    let installedResponse: Result<InstalledBrowsersResponse, Error>

    func searchChromeVersions(query: String) async throws -> BrowserSearchResponse {
        try searchResponse.get()
    }

    func installChrome(result: BrowserSearchResult) async throws -> BrowserInstallResponse {
        try installResponse.get()
    }

    func listInstalledBrowsers() async throws -> InstalledBrowsersResponse {
        try installedResponse.get()
    }
}

private final class FakeManualSessionOpener: ManualSessionOpening {
    let response: Result<ManualSessionResponse, Error>
    private(set) var requestedBrowser: InstalledBrowserRecord?
    private(set) var requestedURL: String?

    init(response: Result<ManualSessionResponse, Error>) {
        self.response = response
    }

    func openManualSession(browser: InstalledBrowserRecord, url: String?) async throws -> ManualSessionResponse {
        requestedBrowser = browser
        requestedURL = url
        return try response.get()
    }
}
