import SwiftUI

public struct BrowserLabWorkbenchView: View {
    @StateObject private var daemonStatusViewModel: DaemonStatusViewModel
    @StateObject private var daemonLifecycleViewModel: DaemonLifecycleViewModel
    @StateObject private var browserViewModel: BrowserSearchViewModel

    @State private var workspaceTab: BrowserWorkspaceTab = .available
    @State private var selectedResultID: String?
    @State private var selectedInstalledID: String?
    @State private var selectedSessionID: String?
    @State private var showDeleteOptions = false

    public init(
        daemonStatusViewModel: DaemonStatusViewModel,
        daemonLifecycleViewModel: DaemonLifecycleViewModel,
        browserViewModel: BrowserSearchViewModel
    ) {
        _daemonStatusViewModel = StateObject(wrappedValue: daemonStatusViewModel)
        _daemonLifecycleViewModel = StateObject(wrappedValue: daemonLifecycleViewModel)
        _browserViewModel = StateObject(wrappedValue: browserViewModel)
    }

    public var body: some View {
        HStack(spacing: 0) {
            RuntimeSidebarView(
                daemonStatusViewModel: daemonStatusViewModel,
                daemonLifecycleViewModel: daemonLifecycleViewModel,
                browserViewModel: browserViewModel
            )
            Divider()
                .overlay(WorkbenchStyle.border)
            BrowserWorkspaceView(
                viewModel: browserViewModel,
                workspaceTab: $workspaceTab,
                selectedResultID: $selectedResultID,
                selectedInstalledID: $selectedInstalledID,
                selectedSessionID: $selectedSessionID
            )
            Divider()
                .overlay(WorkbenchStyle.border)
            InspectorPanelView(
                viewModel: browserViewModel,
                workspaceTab: workspaceTab,
                selectedResult: selectedResult,
                selectedInstalled: selectedInstalled,
                selectedSession: selectedSession,
                showDeleteOptions: $showDeleteOptions
            )
        }
        .background(WorkbenchStyle.background)
        .foregroundStyle(WorkbenchStyle.text)
        .task {
            await refreshAll()
        }
    }

    private var selectedResult: BrowserSearchRow? {
        if let selectedResultID, let selected = browserViewModel.rows.first(where: { $0.id == selectedResultID }) {
            return selected
        }
        return browserViewModel.rows.first
    }

    private var selectedInstalled: InstalledBrowserRow? {
        if let selectedInstalledID, let selected = browserViewModel.installedRows.first(where: { $0.id == selectedInstalledID }) {
            return selected
        }
        return browserViewModel.installedRows.first
    }

    private var selectedSession: ActiveSessionRow? {
        if let selectedSessionID, let selected = browserViewModel.activeSessionRows.first(where: { $0.id == selectedSessionID }) {
            return selected
        }
        return browserViewModel.activeSessionRows.first
    }

    private func refreshAll() async {
        await daemonStatusViewModel.refresh()
        await browserViewModel.refreshMobilePresets()
        await browserViewModel.refreshInstalledBrowsers()
        await browserViewModel.refreshActiveSessions()
    }
}

private enum BrowserWorkspaceTab: String, CaseIterable, Identifiable {
    case available = "Available"
    case installed = "Installed"
    case sessions = "Sessions"

    var id: String { rawValue }
}

private enum WorkbenchStyle {
    static let background = Color(red: 0.055, green: 0.065, blue: 0.075)
    static let sidebar = Color(red: 0.075, green: 0.09, blue: 0.105)
    static let panel = Color(red: 0.09, green: 0.105, blue: 0.12)
    static let row = Color(red: 0.115, green: 0.13, blue: 0.145)
    static let rowSelected = Color(red: 0.055, green: 0.22, blue: 0.24)
    static let border = Color.white.opacity(0.08)
    static let text = Color(red: 0.92, green: 0.95, blue: 0.96)
    static let muted = Color(red: 0.62, green: 0.68, blue: 0.72)
    static let teal = Color(red: 0.05, green: 0.55, blue: 0.60)
    static let green = Color(red: 0.22, green: 0.82, blue: 0.36)
    static let amber = Color(red: 1.0, green: 0.68, blue: 0.12)
    static let red = Color(red: 0.95, green: 0.28, blue: 0.28)
}

private struct RuntimeSidebarView: View {
    @ObservedObject var daemonStatusViewModel: DaemonStatusViewModel
    @ObservedObject var daemonLifecycleViewModel: DaemonLifecycleViewModel
    @ObservedObject var browserViewModel: BrowserSearchViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            BrandHeader()
            SidebarNavigation()
            runtimeStatus
            Spacer(minLength: 12)
            sidebarActions
        }
        .padding(16)
        .frame(width: 226)
        .background(WorkbenchStyle.sidebar)
    }

    private var runtimeStatus: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("RUNTIME STATUS")
                .font(.caption)
                .fontWeight(.semibold)
                .foregroundStyle(WorkbenchStyle.muted)
                .padding(.top, 6)
            StatusBlock(
                icon: "power.circle.fill",
                title: "Daemon",
                value: daemonStatusTitle,
                detail: daemonStatusDetail,
                color: daemonStatusColor
            )
            StatusBlock(
                icon: "square.stack.3d.up.fill",
                title: "Grid",
                value: browserViewModel.activeSessionRows.isEmpty ? "Idle" : "Sessions Active",
                detail: browserViewModel.activeSessionRows.isEmpty ? "Starts on demand" : "\(browserViewModel.activeSessionRows.count) active session(s)",
                color: browserViewModel.activeSessionRows.isEmpty ? WorkbenchStyle.muted : WorkbenchStyle.green
            )
            StatusBlock(
                icon: "shippingbox.fill",
                title: "Docker",
                value: "External Runtime",
                detail: "Required for Selenium Chrome",
                color: WorkbenchStyle.teal
            )
            if let safari = safariDisplay {
                StatusBlock(
                    icon: "safari.fill",
                    title: "Safari (Native)",
                    value: safari.title,
                    detail: safari.subtitle,
                    color: safari.subtitle.contains("ready") ? WorkbenchStyle.green : WorkbenchStyle.amber
                )
            }
        }
    }

    private var sidebarActions: some View {
        VStack(spacing: 10) {
            Button {
                Task {
                    await daemonStatusViewModel.refresh()
                    await browserViewModel.refreshMobilePresets()
                    await browserViewModel.refreshInstalledBrowsers()
                    await browserViewModel.refreshActiveSessions()
                }
            } label: {
                Label("Refresh All", systemImage: "arrow.clockwise")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(SidebarButtonStyle())

            Button {
                Task { await daemonLifecycleViewModel.loadLogs() }
            } label: {
                Label("Open Logs", systemImage: "book.pages")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(SidebarButtonStyle())

            if let message = lifecycleMessage {
                Text(message)
                    .font(.caption)
                    .foregroundStyle(lifecycleColor)
                    .lineLimit(3)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }

    private var daemonStatusTitle: String {
        switch daemonStatusViewModel.displayState {
        case .loading: "Checking"
        case .running: "Daemon Running"
        case .stopped: "Daemon Stopped"
        case .error: "Daemon Error"
        }
    }

    private var daemonStatusDetail: String {
        daemonStatusViewModel.displayState.detail
    }

    private var daemonStatusColor: Color {
        switch daemonStatusViewModel.displayState {
        case .loading: WorkbenchStyle.muted
        case .running: WorkbenchStyle.green
        case .stopped: WorkbenchStyle.amber
        case .error: WorkbenchStyle.red
        }
    }

    private var safariDisplay: NativeRuntimeDisplay? {
        if case .running(let status) = daemonStatusViewModel.displayState {
            return status.nativeRuntimeDisplays.first
        }
        return nil
    }

    private var lifecycleMessage: String? {
        switch daemonLifecycleViewModel.state {
        case .idle: nil
        case .running(let message), .succeeded(let message), .failed(let message): message
        }
    }

    private var lifecycleColor: Color {
        switch daemonLifecycleViewModel.state {
        case .idle, .running: WorkbenchStyle.muted
        case .succeeded: WorkbenchStyle.green
        case .failed: WorkbenchStyle.red
        }
    }
}

private struct BrandHeader: View {
    var body: some View {
        HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 8)
                    .fill(WorkbenchStyle.teal.opacity(0.18))
                Image(systemName: "globe.desk.fill")
                    .foregroundStyle(WorkbenchStyle.teal)
                    .font(.system(size: 24, weight: .semibold))
            }
            .frame(width: 44, height: 44)
            VStack(alignment: .leading, spacing: 3) {
                Text("BrowserLab")
                    .font(.title3)
                    .fontWeight(.semibold)
                Text("Local Selenium Workbench")
                    .font(.caption)
                    .foregroundStyle(WorkbenchStyle.muted)
            }
        }
        .padding(.top, 10)
    }
}

private struct SidebarNavigation: View {
    private let items: [(String, String, Bool)] = [
        ("Dashboard", "gauge.with.dots.needle.33percent", true),
        ("Browsers", "globe", false),
        ("Sessions", "rectangle.on.rectangle", false),
        ("Artifacts", "doc.text.below.ecg", false),
        ("Settings", "gearshape", false),
    ]

    var body: some View {
        VStack(spacing: 4) {
            ForEach(items, id: \.0) { item in
                HStack(spacing: 12) {
                    Image(systemName: item.1)
                        .frame(width: 20)
                    Text(item.0)
                    Spacer()
                }
                .font(.system(size: 14, weight: item.2 ? .semibold : .regular))
                .foregroundStyle(item.2 ? WorkbenchStyle.text : WorkbenchStyle.text.opacity(0.82))
                .padding(.horizontal, 12)
                .padding(.vertical, 10)
                .background(item.2 ? WorkbenchStyle.teal.opacity(0.26) : Color.clear)
                .clipShape(RoundedRectangle(cornerRadius: 7))
            }
        }
        .padding(.top, 12)
    }
}

private struct StatusBlock: View {
    let icon: String
    let title: String
    let value: String
    let detail: String
    let color: Color

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            HStack(spacing: 8) {
                Image(systemName: icon)
                    .foregroundStyle(color)
                    .frame(width: 16)
                Text(title)
                    .font(.subheadline)
                    .fontWeight(.semibold)
                Spacer()
            }
            Text(value)
                .font(.subheadline)
                .foregroundStyle(color)
                .lineLimit(1)
            Text(detail)
                .font(.caption)
                .foregroundStyle(WorkbenchStyle.muted)
                .lineLimit(2)
        }
        .padding(12)
        .background(WorkbenchStyle.panel)
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(WorkbenchStyle.border))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct BrowserWorkspaceView: View {
    @ObservedObject var viewModel: BrowserSearchViewModel
    @Binding var workspaceTab: BrowserWorkspaceTab
    @Binding var selectedResultID: String?
    @Binding var selectedInstalledID: String?
    @Binding var selectedSessionID: String?

    var body: some View {
        VStack(spacing: 12) {
            workspaceToolbar
            workspaceBody
            ActiveSessionsTable(
                viewModel: viewModel,
                selectedSessionID: $selectedSessionID
            )
        }
        .padding(16)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(WorkbenchStyle.background)
    }

    private var workspaceToolbar: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: 10) {
                searchField
                    .frame(minWidth: 220, maxWidth: .infinity)
                tabPicker
                    .frame(width: 200)
                devicePicker
                    .frame(width: 130)
                searchButton
            }

            VStack(spacing: 10) {
                HStack(spacing: 10) {
                    searchField
                    searchButton
                }
                HStack(spacing: 10) {
                    tabPicker
                    devicePicker
                }
            }
        }
    }

    private var searchField: some View {
        HStack(spacing: 8) {
            Image(systemName: "magnifyingglass")
                .foregroundStyle(WorkbenchStyle.muted)
            TextField("Search Chrome versions", text: $viewModel.query)
                .textFieldStyle(.plain)
                .onSubmit { Task { await runSearch() } }
        }
        .padding(.horizontal, 12)
        .frame(height: 38)
        .background(WorkbenchStyle.panel)
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(WorkbenchStyle.border))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    private var tabPicker: some View {
        Picker("", selection: $workspaceTab) {
            ForEach(BrowserWorkspaceTab.allCases) { tab in
                Text(tab.rawValue).tag(tab)
            }
        }
        .pickerStyle(.segmented)
    }

    private var devicePicker: some View {
        Picker("", selection: $viewModel.selectedMobilePresetID) {
            Label("Desktop", systemImage: "desktopcomputer").tag("")
            ForEach(viewModel.mobilePresets) { preset in
                Text(preset.name).tag(preset.id)
            }
        }
    }

    private var searchButton: some View {
        Button {
            Task { await runSearch() }
        } label: {
            Label(viewModel.isSearching ? "Searching" : "Search", systemImage: "magnifyingglass")
                .frame(width: 92)
        }
        .buttonStyle(PrimaryButtonStyle())
        .disabled(viewModel.isSearching)
    }

    @ViewBuilder
    private var workspaceBody: some View {
        switch workspaceTab {
        case .available:
            BrowserResultsTable(
                viewModel: viewModel,
                selectedResultID: $selectedResultID
            )
        case .installed:
            InstalledBrowsersTable(
                viewModel: viewModel,
                selectedInstalledID: $selectedInstalledID
            )
        case .sessions:
            ActiveSessionsTable(
                viewModel: viewModel,
                selectedSessionID: $selectedSessionID,
                isPrimary: true
            )
        }
    }

    private func runSearch() async {
        workspaceTab = .available
        await viewModel.search()
        selectedResultID = viewModel.rows.first?.id
    }
}

private struct BrowserResultsTable: View {
    @ObservedObject var viewModel: BrowserSearchViewModel
    @Binding var selectedResultID: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            TableHeader(
                title: "Results for \(viewModel.query.isEmpty ? "Chrome" : viewModel.query)",
                count: viewModel.rows.count,
                action: nil
            )
            ResultColumnHeader()
            ScrollView {
                LazyVStack(spacing: 8) {
                    if viewModel.rows.isEmpty {
                        EmptyStateView(
                            icon: "magnifyingglass",
                            title: "Search official Selenium Chrome versions",
                            detail: "Use the search field above to find installable Chrome images."
                        )
                        .frame(maxWidth: .infinity, minHeight: 220)
                    } else {
                        ForEach(viewModel.rows) { row in
                            BrowserResultRowView(
                                row: row,
                                isSelected: selectedResultID == row.id,
                                isInstalling: viewModel.installingImageTag == row.id,
                                message: viewModel.installMessages[row.id],
                                onSelect: { selectedResultID = row.id },
                                onInstall: {
                                    selectedResultID = row.id
                                    Task { await viewModel.install(result: row.result) }
                                }
                            )
                        }
                    }
                }
                .padding(8)
            }
        }
        .frame(minHeight: 330)
        .background(WorkbenchStyle.panel)
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(WorkbenchStyle.border))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct ResultColumnHeader: View {
    var body: some View {
        HStack {
            Text("Version").frame(width: 122, alignment: .leading)
            Text("Image (Selenium Docker)").frame(maxWidth: .infinity, alignment: .leading)
            Text("Driver").frame(width: 90, alignment: .leading)
            Text("Grid").frame(width: 66, alignment: .leading)
            Text("Platform").frame(width: 96, alignment: .leading)
            Text("Actions").frame(width: 116, alignment: .trailing)
        }
        .font(.caption)
        .foregroundStyle(WorkbenchStyle.muted)
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
        .background(WorkbenchStyle.background.opacity(0.44))
    }
}

private struct BrowserResultRowView: View {
    let row: BrowserSearchRow
    let isSelected: Bool
    let isInstalling: Bool
    let message: String?
    let onSelect: () -> Void
    let onInstall: () -> Void

    var body: some View {
        Button(action: onSelect) {
            HStack(alignment: .top, spacing: 14) {
                HStack(alignment: .top, spacing: 10) {
                    ChromeGlyph()
                        .frame(width: 22, height: 22)
                    VStack(alignment: .leading, spacing: 4) {
                        Text(row.title.replacingOccurrences(of: "Chrome ", with: ""))
                            .font(.system(size: 14, weight: .semibold))
                            .lineLimit(1)
                            .minimumScaleFactor(0.82)
                        if row.recommended {
                            Chip("Recommended", color: WorkbenchStyle.teal)
                        }
                        if let warning = row.result.warnings?.first, !warning.isEmpty {
                            Label("Apple Silicon", systemImage: "exclamationmark.triangle.fill")
                                .font(.caption)
                                .foregroundStyle(WorkbenchStyle.amber)
                        }
                    }
                }
                .frame(width: 122, alignment: .leading)

                Text(row.result.imageTag)
                    .font(.system(size: 12, design: .monospaced))
                    .foregroundStyle(WorkbenchStyle.text.opacity(0.86))
                    .lineLimit(2)
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)

                Text(row.result.driverVersion ?? "-")
                    .font(.caption)
                    .foregroundStyle(WorkbenchStyle.text.opacity(0.82))
                    .frame(width: 90, alignment: .leading)
                Text(row.result.gridVersion ?? "-")
                    .font(.caption)
                    .foregroundStyle(WorkbenchStyle.text.opacity(0.82))
                    .frame(width: 66, alignment: .leading)
                VStack(alignment: .leading, spacing: 4) {
                    ForEach(row.result.platforms.prefix(2), id: \.self) { platform in
                        Chip(platform, color: WorkbenchStyle.muted)
                    }
                }
                .frame(width: 96, alignment: .leading)

                HStack(spacing: 8) {
                    Button(isInstalling ? "Installing" : "Install", action: onInstall)
                        .buttonStyle(CompactTealButtonStyle())
                        .disabled(isInstalling)
                    Button("Open") {}
                        .buttonStyle(CompactDarkButtonStyle())
                        .disabled(true)
                }
                .frame(width: 116, alignment: .trailing)
            }
            .padding(12)
            .background(isSelected ? WorkbenchStyle.rowSelected : WorkbenchStyle.row)
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(isSelected ? WorkbenchStyle.teal : WorkbenchStyle.border))
            .clipShape(RoundedRectangle(cornerRadius: 7))
        }
        .buttonStyle(.plain)

        if let message, !message.isEmpty {
            Text(message)
                .font(.caption)
                .foregroundStyle(WorkbenchStyle.muted)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 16)
        }
    }
}

private struct InstalledBrowsersTable: View {
    @ObservedObject var viewModel: BrowserSearchViewModel
    @Binding var selectedInstalledID: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            TableHeader(title: "Installed Browsers", count: viewModel.installedRows.count, action: nil)
            HStack(spacing: 10) {
                TextField("Target URL, defaults to about:blank", text: $viewModel.targetURL)
                    .textFieldStyle(.plain)
                    .padding(.horizontal, 12)
                    .frame(height: 36)
                    .background(WorkbenchStyle.background.opacity(0.65))
                    .clipShape(RoundedRectangle(cornerRadius: 7))
                Toggle("Delete image", isOn: $viewModel.deleteImageOnUninstall)
                Toggle("Confirm", isOn: $viewModel.confirmDeleteImage)
                    .disabled(!viewModel.deleteImageOnUninstall)
            }
            .padding(12)
            ScrollView {
                LazyVStack(spacing: 8) {
                    if viewModel.installedRows.isEmpty {
                        EmptyStateView(
                            icon: "shippingbox",
                            title: "No installed browsers",
                            detail: "Install a Chrome image from Available to open manual sessions."
                        )
                        .frame(maxWidth: .infinity, minHeight: 220)
                    } else {
                        ForEach(viewModel.installedRows) { row in
                            InstalledBrowserRowView(
                                row: row,
                                isSelected: selectedInstalledID == row.id,
                                openingImageTag: viewModel.openingImageTag,
                                message: viewModel.installedMessages[row.id],
                                onSelect: { selectedInstalledID = row.id },
                                onOpen: {
                                    selectedInstalledID = row.id
                                    Task { await viewModel.openManualSession(record: row.record) }
                                },
                                onDisable: { Task { await viewModel.disable(record: row.record) } },
                                onUninstall: {
                                    Task {
                                        await viewModel.uninstall(
                                            record: row.record,
                                            deleteImage: viewModel.deleteImageOnUninstall,
                                            confirmDeleteImage: viewModel.confirmDeleteImage
                                        )
                                    }
                                }
                            )
                        }
                    }
                }
                .padding(8)
            }
        }
        .frame(minHeight: 330)
        .background(WorkbenchStyle.panel)
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(WorkbenchStyle.border))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct InstalledBrowserRowView: View {
    let row: InstalledBrowserRow
    let isSelected: Bool
    let openingImageTag: String?
    let message: String?
    let onSelect: () -> Void
    let onOpen: () -> Void
    let onDisable: () -> Void
    let onUninstall: () -> Void

    var body: some View {
        Button(action: onSelect) {
            HStack(alignment: .top, spacing: 14) {
                Image(systemName: row.enabled ? "checkmark.seal.fill" : "pause.circle.fill")
                    .foregroundStyle(row.enabled ? WorkbenchStyle.green : WorkbenchStyle.amber)
                    .font(.title3)
                VStack(alignment: .leading, spacing: 5) {
                    Text(row.title)
                        .font(.system(size: 15, weight: .semibold))
                    Text(row.record.imageTag)
                        .font(.system(size: 12, design: .monospaced))
                        .foregroundStyle(WorkbenchStyle.muted)
                        .lineLimit(2)
                        .textSelection(.enabled)
                    HStack(spacing: 6) {
                        Chip(row.record.platform, color: WorkbenchStyle.muted)
                        Chip(row.enabled ? "enabled" : "disabled", color: row.enabled ? WorkbenchStyle.green : WorkbenchStyle.amber)
                    }
                    if let message {
                        Text(message)
                            .font(.caption)
                            .foregroundStyle(WorkbenchStyle.muted)
                            .textSelection(.enabled)
                    }
                }
                Spacer()
                HStack(spacing: 8) {
                    Button(openingImageTag == row.id ? "Opening" : "Open", action: onOpen)
                        .buttonStyle(CompactTealButtonStyle())
                        .disabled(openingImageTag == row.id)
                    if row.enabled {
                        Button("Disable", action: onDisable)
                            .buttonStyle(CompactDarkButtonStyle())
                    }
                    Button("Uninstall", action: onUninstall)
                        .buttonStyle(CompactDangerButtonStyle())
                }
            }
            .padding(12)
            .background(isSelected ? WorkbenchStyle.rowSelected : WorkbenchStyle.row)
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(isSelected ? WorkbenchStyle.teal : WorkbenchStyle.border))
            .clipShape(RoundedRectangle(cornerRadius: 7))
        }
        .buttonStyle(.plain)
    }
}

private struct ActiveSessionsTable: View {
    @ObservedObject var viewModel: BrowserSearchViewModel
    @Binding var selectedSessionID: String?
    var isPrimary = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            TableHeader(
                title: "Active Sessions",
                count: viewModel.activeSessionRows.count,
                action: isPrimary ? nil : "View All Sessions"
            )
            if viewModel.activeSessionRows.isEmpty {
                EmptyStateView(
                    icon: "rectangle.on.rectangle",
                    title: "No active sessions",
                    detail: "Open an installed browser to start a held noVNC session."
                )
                .frame(maxWidth: .infinity, minHeight: isPrimary ? 330 : 190)
            } else {
                ScrollView {
                    LazyVStack(spacing: 7) {
                        ForEach(viewModel.activeSessionRows) { row in
                            SessionRowView(
                                row: row,
                                isSelected: selectedSessionID == row.id,
                                isClosing: viewModel.closingSessionID == row.id,
                                onSelect: { selectedSessionID = row.id },
                                onClose: { Task { await viewModel.closeSession(sessionId: row.id) } },
                                onCapture: {
                                    Task {
                                        if viewModel.activeSession?.sessionId != row.id {
                                            selectedSessionID = row.id
                                        }
                                        await viewModel.captureActiveSessionScreenshot()
                                    }
                                }
                            )
                        }
                    }
                    .padding(8)
                }
            }
        }
        .frame(minHeight: isPrimary ? 430 : 260)
        .background(WorkbenchStyle.panel)
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(WorkbenchStyle.border))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct SessionRowView: View {
    let row: ActiveSessionRow
    let isSelected: Bool
    let isClosing: Bool
    let onSelect: () -> Void
    let onClose: () -> Void
    let onCapture: () -> Void

    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: 12) {
                Text(shortSessionID)
                    .font(.system(size: 12, design: .monospaced))
                    .foregroundStyle(WorkbenchStyle.text.opacity(0.88))
                    .frame(width: 128, alignment: .leading)
                VStack(alignment: .leading, spacing: 3) {
                    Text(row.session.browserVersion)
                        .font(.subheadline)
                        .fontWeight(.semibold)
                    Text(row.session.mobileEmulation?.name ?? "Desktop")
                        .font(.caption)
                        .foregroundStyle(WorkbenchStyle.muted)
                }
                .frame(width: 110, alignment: .leading)
                Text(row.session.currentUrl ?? row.session.requestedUrl)
                    .font(.caption)
                    .foregroundStyle(WorkbenchStyle.muted)
                    .lineLimit(1)
                    .frame(maxWidth: .infinity, alignment: .leading)
                if let url = row.noVNCURL {
                    Link(destination: url) {
                        Label("Open", systemImage: "arrow.up.right.square")
                    }
                    .buttonStyle(CompactTealButtonStyle())
                }
                Button {
                    onCapture()
                } label: {
                    Image(systemName: "camera")
                }
                .buttonStyle(IconButtonStyle())
                Button {
                    onClose()
                } label: {
                    Image(systemName: isClosing ? "hourglass" : "trash")
                }
                .buttonStyle(DangerIconButtonStyle())
                .disabled(isClosing)
            }
            .padding(10)
            .background(isSelected ? WorkbenchStyle.rowSelected : WorkbenchStyle.row)
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(isSelected ? WorkbenchStyle.teal : WorkbenchStyle.border))
            .clipShape(RoundedRectangle(cornerRadius: 7))
        }
        .buttonStyle(.plain)
    }

    private var shortSessionID: String {
        String(row.id.prefix(18))
    }
}

private struct InspectorPanelView: View {
    @ObservedObject var viewModel: BrowserSearchViewModel
    let workspaceTab: BrowserWorkspaceTab
    let selectedResult: BrowserSearchRow?
    let selectedInstalled: InstalledBrowserRow?
    let selectedSession: ActiveSessionRow?
    @Binding var showDeleteOptions: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            inspectorHeader
            Divider().overlay(WorkbenchStyle.border)
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    selectedDetails
                    artifacts
                }
                .padding(.bottom, 16)
            }
        }
        .padding(16)
        .frame(width: 280)
        .background(WorkbenchStyle.sidebar)
    }

    @ViewBuilder
    private var inspectorHeader: some View {
        switch workspaceTab {
        case .available:
            if let selectedResult {
                HStack(spacing: 12) {
                    ChromeGlyph()
                        .frame(width: 28, height: 28)
                    VStack(alignment: .leading, spacing: 4) {
                        Text(selectedResult.title)
                            .font(.headline)
                        if selectedResult.recommended {
                            Chip("Recommended", color: WorkbenchStyle.teal)
                        }
                    }
                    Spacer()
                }
            } else {
                EmptyInspectorHeader(title: "No Browser Selected")
            }
        case .installed:
            if let selectedInstalled {
                EmptyInspectorHeader(title: selectedInstalled.title, icon: "shippingbox.fill")
            } else {
                EmptyInspectorHeader(title: "No Installed Browser")
            }
        case .sessions:
            if let selectedSession {
                EmptyInspectorHeader(title: "Session \(String(selectedSession.id.prefix(8)))", icon: "rectangle.on.rectangle")
            } else {
                EmptyInspectorHeader(title: "No Session Selected")
            }
        }
    }

    @ViewBuilder
    private var selectedDetails: some View {
        switch workspaceTab {
        case .available:
            if let selectedResult {
                InspectorSection(title: "Image & Provenance") {
                    DetailLine("Image", selectedResult.result.imageTag, monospaced: true)
                    DetailLine("Driver", selectedResult.result.driverVersion ?? "-")
                    DetailLine("Grid", selectedResult.result.gridVersion ?? "-")
                    DetailLine("Platforms", selectedResult.result.platforms.joined(separator: ", "))
                    DetailLine("Source", selectedResult.result.repository)
                }
                InspectorSection(title: "Install Status") {
                    DetailLine("Status", viewModel.installMessages[selectedResult.id] ?? "Not Installed")
                    Button {
                        Task { await viewModel.install(result: selectedResult.result) }
                    } label: {
                        Label(viewModel.installingImageTag == selectedResult.id ? "Installing" : "Install", systemImage: "arrow.down.to.line")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(PrimaryButtonStyle())
                    .disabled(viewModel.installingImageTag == selectedResult.id)
                }
                if let warning = selectedResult.result.warnings?.first, !warning.isEmpty {
                    InspectorSection(title: "Runtime Notes") {
                        Label(warning, systemImage: "exclamationmark.triangle.fill")
                            .font(.caption)
                            .foregroundStyle(WorkbenchStyle.amber)
                    }
                }
            } else {
                EmptyStateView(icon: "sidebar.right", title: "Nothing selected", detail: "Select a browser row to inspect image provenance.")
            }
        case .installed:
            if let selectedInstalled {
                InspectorSection(title: "Installed Browser") {
                    DetailLine("Version", selectedInstalled.record.version)
                    DetailLine("Image", selectedInstalled.record.imageTag, monospaced: true)
                    DetailLine("Platform", selectedInstalled.record.platform)
                    DetailLine("Source", selectedInstalled.record.source)
                    DetailLine("Enabled", selectedInstalled.enabled ? "Yes" : "No")
                }
                InspectorSection(title: "Manual Session") {
                    TextField("Target URL", text: $viewModel.targetURL)
                        .textFieldStyle(.roundedBorder)
                    Button {
                        Task { await viewModel.openManualSession(record: selectedInstalled.record) }
                    } label: {
                        Label(viewModel.openingImageTag == selectedInstalled.id ? "Opening" : "Open Browser", systemImage: "play.fill")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(PrimaryButtonStyle())
                    .disabled(viewModel.openingImageTag == selectedInstalled.id)
                }
            } else {
                EmptyStateView(icon: "shippingbox", title: "Nothing installed", detail: "Install a browser to manage local sessions.")
            }
        case .sessions:
            if let selectedSession {
                InspectorSection(title: "Session Detail") {
                    DetailLine("Session ID", selectedSession.session.sessionId, monospaced: true)
                    DetailLine("Browser", "\(selectedSession.session.browserName) \(selectedSession.session.browserVersion)")
                    DetailLine("URL", selectedSession.session.currentUrl ?? selectedSession.session.requestedUrl)
                    DetailLine("Started", selectedSession.session.startedAt)
                    DetailLine("Mobile", selectedSession.session.mobileEmulation?.name ?? "Desktop")
                }
                InspectorSection(title: "Session Actions") {
                    if let url = selectedSession.noVNCURL {
                        Link(destination: url) {
                            Label("Open noVNC", systemImage: "arrow.up.right.square")
                                .frame(maxWidth: .infinity)
                        }
                        .buttonStyle(PrimaryButtonStyle())
                    }
                    Button {
                        Task { await viewModel.captureActiveSessionScreenshot() }
                    } label: {
                        Label(viewModel.capturingScreenshot ? "Capturing" : "Capture Screenshot", systemImage: "camera")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(CompactDarkButtonStyle())
                    .disabled(viewModel.capturingScreenshot)
                }
            } else {
                EmptyStateView(icon: "rectangle.on.rectangle", title: "No active session", detail: "Open a browser to inspect session details.")
            }
        }
    }

    private var artifacts: some View {
        InspectorSection(title: "Recent Artifacts") {
            if viewModel.recentArtifacts.isEmpty {
                Text("Screenshots and metadata appear here after capture.")
                    .font(.caption)
                    .foregroundStyle(WorkbenchStyle.muted)
            } else {
                ForEach(viewModel.recentArtifacts.prefix(4)) { artifact in
                    HStack(spacing: 10) {
                        Image(systemName: "photo")
                            .foregroundStyle(WorkbenchStyle.muted)
                        VStack(alignment: .leading, spacing: 2) {
                            Text(artifact.title)
                                .font(.caption)
                                .lineLimit(1)
                            Text(artifact.artifact.summaryPath)
                                .font(.caption2)
                                .foregroundStyle(WorkbenchStyle.muted)
                                .lineLimit(1)
                        }
                        Spacer()
                        Link(destination: artifact.screenshotURL) {
                            Image(systemName: "arrow.up.right.square")
                        }
                    }
                    .padding(8)
                    .background(WorkbenchStyle.row)
                    .clipShape(RoundedRectangle(cornerRadius: 7))
                }
            }
        }
    }
}

private struct EmptyInspectorHeader: View {
    let title: String
    var icon = "sidebar.right"

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: icon)
                .foregroundStyle(WorkbenchStyle.teal)
                .font(.title3)
            Text(title)
                .font(.headline)
            Spacer()
        }
    }
}

private struct InspectorSection<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title)
                .font(.subheadline)
                .fontWeight(.semibold)
            content
        }
        .padding(12)
        .background(WorkbenchStyle.panel)
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(WorkbenchStyle.border))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct DetailLine: View {
    let label: String
    let value: String
    let monospaced: Bool

    init(_ label: String, _ value: String, monospaced: Bool = false) {
        self.label = label
        self.value = value
        self.monospaced = monospaced
    }

    var body: some View {
        Grid(alignment: .leading, horizontalSpacing: 12, verticalSpacing: 4) {
            GridRow {
                Text(label)
                    .font(.caption)
                    .foregroundStyle(WorkbenchStyle.muted)
                    .frame(width: 72, alignment: .leading)
                Text(value.isEmpty ? "-" : value)
                    .font(monospaced ? .system(size: 12, design: .monospaced) : .caption)
                    .foregroundStyle(WorkbenchStyle.text.opacity(0.9))
                    .textSelection(.enabled)
                    .lineLimit(monospaced ? 3 : 2)
            }
        }
    }
}

private struct TableHeader: View {
    let title: String
    let count: Int
    let action: String?

    var body: some View {
        HStack {
            Text(title)
                .font(.headline)
            Text("\(count)")
                .font(.caption)
                .foregroundStyle(WorkbenchStyle.muted)
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(WorkbenchStyle.background)
                .clipShape(Capsule())
            Spacer()
            if let action {
                Button(action) {}
                    .buttonStyle(CompactDarkButtonStyle())
                    .disabled(true)
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
    }
}

private struct EmptyStateView: View {
    let icon: String
    let title: String
    let detail: String

    var body: some View {
        VStack(spacing: 10) {
            Image(systemName: icon)
                .font(.largeTitle)
                .foregroundStyle(WorkbenchStyle.muted)
            Text(title)
                .font(.headline)
            Text(detail)
                .font(.caption)
                .foregroundStyle(WorkbenchStyle.muted)
        }
        .multilineTextAlignment(.center)
        .padding(24)
    }
}

private struct ChromeGlyph: View {
    var body: some View {
        ZStack {
            Circle()
                .fill(
                    AngularGradient(
                        colors: [
                            Color(red: 0.93, green: 0.23, blue: 0.18),
                            Color(red: 0.98, green: 0.75, blue: 0.14),
                            Color(red: 0.20, green: 0.64, blue: 0.28),
                            Color(red: 0.93, green: 0.23, blue: 0.18),
                        ],
                        center: .center
                    )
                )
            Circle()
                .fill(Color(red: 0.12, green: 0.48, blue: 0.92))
                .padding(5)
            Circle()
                .stroke(Color.white.opacity(0.75), lineWidth: 1)
                .padding(5)
        }
        .accessibilityLabel("Chrome")
    }
}

private struct Chip: View {
    let text: String
    let color: Color

    init(_ text: String, color: Color) {
        self.text = text
        self.color = color
    }

    var body: some View {
        Text(text)
            .font(.caption2)
            .fontWeight(.medium)
            .foregroundStyle(color)
            .padding(.horizontal, 7)
            .padding(.vertical, 3)
            .background(color.opacity(0.14))
            .clipShape(RoundedRectangle(cornerRadius: 5))
    }
}

private struct PrimaryButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(.white)
            .padding(.horizontal, 12)
            .frame(height: 38)
            .background(WorkbenchStyle.teal.opacity(configuration.isPressed ? 0.7 : 1))
            .clipShape(RoundedRectangle(cornerRadius: 7))
    }
}

private struct SidebarButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 13, weight: .medium))
            .foregroundStyle(WorkbenchStyle.text)
            .padding(.horizontal, 12)
            .frame(height: 36)
            .background(WorkbenchStyle.panel.opacity(configuration.isPressed ? 0.7 : 1))
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(WorkbenchStyle.border))
            .clipShape(RoundedRectangle(cornerRadius: 7))
    }
}

private struct CompactTealButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 12, weight: .semibold))
            .foregroundStyle(.white)
            .padding(.horizontal, 10)
            .frame(height: 30)
            .background(WorkbenchStyle.teal.opacity(configuration.isPressed ? 0.7 : 1))
            .clipShape(RoundedRectangle(cornerRadius: 6))
    }
}

private struct CompactDarkButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 12, weight: .medium))
            .foregroundStyle(WorkbenchStyle.text)
            .padding(.horizontal, 10)
            .frame(height: 30)
            .background(WorkbenchStyle.background.opacity(configuration.isPressed ? 0.5 : 0.85))
            .overlay(RoundedRectangle(cornerRadius: 6).stroke(WorkbenchStyle.border))
            .clipShape(RoundedRectangle(cornerRadius: 6))
    }
}

private struct CompactDangerButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 12, weight: .medium))
            .foregroundStyle(WorkbenchStyle.red)
            .padding(.horizontal, 10)
            .frame(height: 30)
            .background(WorkbenchStyle.red.opacity(configuration.isPressed ? 0.2 : 0.12))
            .clipShape(RoundedRectangle(cornerRadius: 6))
    }
}

private struct IconButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(WorkbenchStyle.text)
            .frame(width: 30, height: 30)
            .background(WorkbenchStyle.background.opacity(configuration.isPressed ? 0.5 : 0.85))
            .overlay(RoundedRectangle(cornerRadius: 6).stroke(WorkbenchStyle.border))
            .clipShape(RoundedRectangle(cornerRadius: 6))
    }
}

private struct DangerIconButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(WorkbenchStyle.red)
            .frame(width: 30, height: 30)
            .background(WorkbenchStyle.red.opacity(configuration.isPressed ? 0.2 : 0.12))
            .clipShape(RoundedRectangle(cornerRadius: 6))
    }
}
