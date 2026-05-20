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

    func testDisableInstalledBrowserRefreshesRowsAndShowsMessage() async {
        let imageTag = "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
        let installed = InstalledBrowserRecord(
            family: "chrome",
            version: "119.0",
            imageTag: imageTag,
            platform: "linux/amd64",
            source: "selenium-dockerhub",
            enabled: true
        )
        let disabled = InstalledBrowserRecord(
            family: "chrome",
            version: "119.0",
            imageTag: imageTag,
            platform: "linux/amd64",
            source: "selenium-dockerhub",
            enabled: false
        )
        let manager = FakeBrowserManager(
            searchResponse: .success(.init(browser: "chrome", query: "", results: [])),
            installResponse: .failure(NSError(domain: "BrowserLab", code: 1)),
            installedResponse: .success(.init(browsers: [disabled])),
            disableResponse: .success(.init(record: disabled))
        )
        let viewModel = await BrowserSearchViewModel(client: manager, lister: manager, disabler: manager)

        await viewModel.disable(record: installed)

        let rows = await viewModel.installedRows
        let messages = await viewModel.installedMessages
        XCTAssertEqual(rows.count, 1)
        XCTAssertFalse(rows[0].enabled)
        XCTAssertTrue(messages[imageTag]?.contains("Disabled Chrome 119.0") == true)
    }

    func testUninstallInstalledBrowserRequiresExplicitImageDeleteConfirmation() async {
        let imageTag = "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
        let installed = InstalledBrowserRecord(
            family: "chrome",
            version: "119.0",
            imageTag: imageTag,
            platform: "linux/amd64",
            source: "selenium-dockerhub",
            enabled: true
        )
        let manager = FakeBrowserManager(
            searchResponse: .success(.init(browser: "chrome", query: "", results: [])),
            installResponse: .failure(NSError(domain: "BrowserLab", code: 1)),
            installedResponse: .success(.init(browsers: [])),
            uninstallResponses: [
                .failure(NSError(domain: "BrowserLab", code: 400, userInfo: [NSLocalizedDescriptionKey: "image_delete_confirmation_required"])),
                .success(.init(record: installed, imageDeleted: true))
            ]
        )
        let viewModel = await BrowserSearchViewModel(client: manager, lister: manager, uninstaller: manager)

        await viewModel.uninstall(record: installed, deleteImage: true, confirmDeleteImage: false)
        var messages = await viewModel.installedMessages
        XCTAssertTrue(messages[imageTag]?.contains("image_delete_confirmation_required") == true)

        await viewModel.uninstall(record: installed, deleteImage: true, confirmDeleteImage: true)
        messages = await viewModel.installedMessages
        let rows = await viewModel.installedRows
        XCTAssertTrue(messages[imageTag]?.contains("Deleted Docker image") == true)
        XCTAssertTrue(rows.isEmpty)
    }
}

private struct FakeBrowserSearchClient: BrowserVersionSearching {
    let response: Result<BrowserSearchResponse, Error>

    func searchChromeVersions(query: String) async throws -> BrowserSearchResponse {
        try response.get()
    }
}

private final class FakeBrowserManager: BrowserVersionSearching, BrowserVersionInstalling, InstalledBrowserListing, InstalledBrowserDisabling, InstalledBrowserUninstalling {
    let searchResponse: Result<BrowserSearchResponse, Error>
    let installResponse: Result<BrowserInstallResponse, Error>
    let installedResponse: Result<InstalledBrowsersResponse, Error>
    let disableResponse: Result<BrowserDisableResponse, Error>
    var uninstallResponses: [Result<BrowserUninstallResponse, Error>]

    init(
        searchResponse: Result<BrowserSearchResponse, Error>,
        installResponse: Result<BrowserInstallResponse, Error>,
        installedResponse: Result<InstalledBrowsersResponse, Error>,
        disableResponse: Result<BrowserDisableResponse, Error> = .failure(NSError(domain: "BrowserLab", code: 1)),
        uninstallResponses: [Result<BrowserUninstallResponse, Error>] = []
    ) {
        self.searchResponse = searchResponse
        self.installResponse = installResponse
        self.installedResponse = installedResponse
        self.disableResponse = disableResponse
        self.uninstallResponses = uninstallResponses
    }

    func searchChromeVersions(query: String) async throws -> BrowserSearchResponse {
        try searchResponse.get()
    }

    func installChrome(result: BrowserSearchResult) async throws -> BrowserInstallResponse {
        try installResponse.get()
    }

    func listInstalledBrowsers() async throws -> InstalledBrowsersResponse {
        try installedResponse.get()
    }

    func disableBrowser(record: InstalledBrowserRecord) async throws -> BrowserDisableResponse {
        try disableResponse.get()
    }

    func uninstallBrowser(record: InstalledBrowserRecord, deleteImage: Bool, confirmDeleteImage: Bool) async throws -> BrowserUninstallResponse {
        if uninstallResponses.isEmpty {
            throw NSError(domain: "BrowserLab", code: 1)
        }
        return try uninstallResponses.removeFirst().get()
    }
}
