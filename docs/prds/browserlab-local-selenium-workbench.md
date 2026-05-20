# PRD: BrowserLab Local Selenium Workbench

## Problem Statement

人类和 AI Agent 需要在本地 macOS 环境中快速验证旧版和普通浏览器的兼容性，但直接使用 Selenium Docker 镜像、Grid、noVNC 和 WebDriver 命令过于繁琐。当前操作需要手写或记忆 Docker Compose、端口、镜像 tag、WebDriver capability、noVNC 地址和 session 生命周期；截图模式和人工接管模式也容易混淆，导致 noVNC 里只看到 Selenium 背景图而不是浏览器。

用户希望有一个 macOS 应用来管理可用的浏览器版本、安装本地镜像、一键打开旧版浏览器进行人工验收，同时提供 CLI 和本机 Selenium 服务接口，让本机其他服务和 AI Agent 可以自动跑测试、截图和报告。

## Solution

构建一个本机 C/S 架构的 BrowserLab：

- 一个常驻本机 daemon 作为核心服务，负责 Docker、Selenium Dynamic Grid、浏览器 registry、session、noVNC、截图和 artifacts。
- 一个 SwiftUI macOS App 作为人类客户端，提供浏览器搜索、安装、启动、session 管理、内嵌 noVNC、截图和状态诊断。
- 一个 CLI 作为人类和 AI Agent 客户端，调用同一个 daemon API，提供可脚本化、可 JSON 输出的操作入口。
- 底层浏览器运行时以官方 Selenium Docker 镜像为主，MVP 只承诺 Docker Selenium 浏览器；Safari 只检测当前系统版本并给出配置提示，不承诺旧版安装。
- Selenium 运行模式采用 Dynamic Grid。daemon 常驻 Grid，按 WebDriver session capability 动态创建浏览器容器；用户点击“启动浏览器”时，产品语义是创建并保持一个人工验收 session。

目标是让用户用原生 macOS App 完成“搜索版本 -> 安装 -> 打开浏览器 -> noVNC 人工验收 -> 截图/记录”的闭环，同时让 Agent 通过 CLI 或本机 API 完成同样的动作。

## User Stories

1. As a human tester, I want to search supported Chrome versions, so that I can find old browser versions without browsing Docker Hub manually.
2. As a human tester, I want search results to show browser version first, so that I do not need to understand Selenium image tag naming.
3. As a human tester, I want each result to show the exact Selenium image tag in details, so that I can understand what will be installed.
4. As a human tester, I want to install a browser version with one click, so that I do not need to run `docker pull` manually.
5. As a human tester, I want to see install progress and Docker pull errors, so that I can diagnose registry or network failures.
6. As a human tester, I want installed browsers listed on the home screen, so that I can quickly pick a browser for compatibility testing.
7. As a human tester, I want to open Chrome 90 or Chrome 100 from the App, so that I can manually inspect old-version behavior.
8. As a human tester, I want the App to open a noVNC view for the selected session, so that I can use the remote browser like a normal browser.
9. As a human tester, I want the App to support entering a target URL before opening a browser, so that the browser starts at the page under test.
10. As a human tester, I want the App to default to `about:blank` when no URL is provided, so that I can manually type a URL.
11. As a human tester, I want to keep a session open until I close it, so that noVNC does not return to the Selenium desktop background unexpectedly.
12. As a human tester, I want to close a session explicitly, so that I can free the browser slot.
13. As a human tester, I want to see active sessions, so that I know which browser versions are currently running.
14. As a human tester, I want to capture a screenshot from an active session, so that I can attach visual evidence to an issue or report.
15. As a human tester, I want artifacts grouped by session, so that screenshots and metadata remain traceable.
16. As a human tester, I want the App to show Grid health, so that I know whether the backend service is working.
17. As a human tester, I want the App to show Docker runtime status, so that I know whether Docker Desktop, OrbStack, or Colima is available.
18. As a human tester, I want clear guidance when Docker is missing, so that I can install or start a supported Docker runtime.
19. As a human tester, I want to see whether a browser runs through amd64 emulation on Apple Silicon, so that I understand performance tradeoffs.
20. As a human tester, I want to run Chrome mobile emulation presets, so that I can quickly check mobile layout without setting up Android.
21. As a human tester, I want Safari shown separately as a native macOS capability, so that I do not confuse current Safari detection with installable old Safari versions.
22. As a human tester, I want noVNC embedded in the App, so that I do not need to manage browser tabs manually.
23. As a human tester, I want an “open in external browser” option for noVNC, so that I can use a larger or separate browser window when needed.
24. As a human tester, I want the App to keep the daemon running after the window closes, so that background sessions and test endpoints continue to work.
25. As a human tester, I want a menu bar control for daemon status, so that I can start, stop, restart, and inspect the service quickly.
26. As a developer, I want a stable localhost WebDriver endpoint, so that local test suites can target BrowserLab without knowing Docker internals.
27. As a developer, I want browser sessions selected through browser name and version capabilities, so that tests run against the intended version.
28. As a developer, I want installed browser versions stored in a registry, so that the environment is reproducible and inspectable.
29. As a developer, I want generated Selenium/Docker config to be derived from the registry, so that manual config drift does not break the App.
30. As a developer, I want to view generated config read-only, so that I can debug without making unsupported edits.
31. As a developer, I want uninstall to remove a browser from the registry first, so that Docker images are not accidentally deleted.
32. As a developer, I want image deletion to require explicit confirmation, so that shared local Docker images are safe.
33. As an AI Agent, I want a CLI command to search browser versions, so that I can discover supported targets programmatically.
34. As an AI Agent, I want a CLI command to install a browser version, so that I can prepare a test environment without UI interaction.
35. As an AI Agent, I want a CLI command to open a held session, so that a human can inspect the same browser state through noVNC.
36. As an AI Agent, I want a CLI command to capture screenshots, so that I can produce acceptance evidence.
37. As an AI Agent, I want CLI output available as JSON, so that I can reliably parse session IDs, noVNC URLs, artifacts, and errors.
38. As an AI Agent, I want explicit exit codes for setup, Docker, Grid, browser, and navigation failures, so that automation can branch correctly.
39. As a local service, I want daemon APIs protected by localhost-only binding and a local token, so that arbitrary web pages cannot control Docker.
40. As a maintainer, I want daemon logs and artifacts stored in a standard user Application Support directory, so that support and cleanup are predictable.
41. As a maintainer, I want the daemon installed as a user LaunchAgent, so that it starts with the user without requiring root privileges.
42. As a maintainer, I want a small set of deeply tested daemon modules, so that Docker, Grid, registry, and session behavior remain reliable as UI evolves.
43. As a maintainer, I want the App, CLI, and daemon to share one domain model, so that browser state does not diverge across clients.
44. As a maintainer, I want Selenium-only runtime support in MVP, so that Selenoid and third-party image inconsistencies do not complicate the product.

## Implementation Decisions

- Product architecture is local C/S. The daemon is the product core; macOS App and CLI are clients.
- The macOS App will be built with SwiftUI.
- The App will use WKWebView to embed noVNC and will also support opening noVNC in an external browser.
- The daemon and CLI should be implemented in Go unless later implementation evidence forces a different choice.
- The daemon should be installed as a user-level LaunchAgent, not a system-level daemon.
- Closing the macOS App window must not stop the daemon.
- The daemon must expose a localhost-only API, protected by a local bearer token generated on first run.
- The daemon must manage Docker runtime detection and should support Docker Desktop, OrbStack, and Colima as external dependencies. Docker should not be bundled in MVP.
- The Selenium runtime mode is Dynamic Grid.
- The daemon should own the Selenium Dynamic Grid lifecycle.
- Browser installation means pulling the exact Selenium Docker image and registering it in the Browser Registry.
- The Browser Registry is the source of truth. Generated Docker and Selenium config are derived artifacts.
- Users should not directly edit generated Selenium/Docker config in MVP.
- Search UI should display browser version first, with image tag, driver version, grid version, architecture, and warnings in detail.
- Install records must save the full image tag for reproducibility.
- MVP uses official Selenium Docker images only.
- Selenoid and third-party image providers are out of scope for MVP.
- Safari should be represented as native capability detection and setup guidance only; old Safari installation is out of scope.
- Mobile support in MVP should be Chrome mobile emulation presets, not Android Emulator, Appium, iOS Simulator, or real devices.
- “Open Browser” in UI means creating a held WebDriver session, not merely starting a Docker container.
- Screenshot commands may create short-lived sessions, but manual noVNC use must create held sessions.
- Session state must be first-class: session ID, browser, version, URL, started time, noVNC URL, WebDriver endpoint, artifacts, and close action.
- Internal browser state should be layered: Available, Installed, Enabled, Grid Running, Session Active, Error.
- UI may aggregate internal states into simpler statuses, but daemon APIs should expose enough detail for diagnostics.
- Default concurrency is one session per browser/version and a configurable total session limit.
- Artifacts for MVP include screenshots, session metadata, JSON result, and Markdown summary. Video is deferred.
- CLI should mirror daemon concepts and support both human-readable and JSON output.
- The product name can remain BrowserLab as a working name.

Major modules:

- Browser Catalog: searches official Selenium Docker tags, normalizes browser versions, and chooses recommended tags.
- Browser Registry: stores installed/enabled browser records and exact image provenance.
- Docker Runtime Manager: detects Docker, pulls images, checks local image availability, and reports errors.
- Dynamic Grid Manager: generates Selenium Dynamic Grid config and controls Grid lifecycle.
- Session Manager: creates held sessions, maps sessions to browser versions, exposes noVNC URLs, closes sessions, and captures screenshots.
- Artifact Manager: stores screenshots, metadata, and summaries.
- Daemon API: localhost authenticated API used by App and CLI.
- CLI Client: scriptable interface for humans and AI Agents.
- SwiftUI App: browser management, daemon status, session UI, embedded noVNC, and artifact preview.

## Testing Decisions

- Tests should focus on external behavior and contracts, not implementation details.
- Browser Catalog tests should verify version normalization, tag selection, and filtering from representative Docker Hub tag fixtures.
- Browser Registry tests should verify install, enable, disable, uninstall, and exact image provenance persistence.
- Docker Runtime Manager tests should isolate command execution behind an interface and cover success, missing Docker, pull failure, platform mismatch, and image already present.
- Dynamic Grid Manager tests should verify generated config from registry state and avoid snapshot churn except for stable contract fixtures.
- Session Manager tests should verify capability construction, held session lifecycle, close behavior, screenshot artifact creation, and error mapping.
- Daemon API tests should verify authentication, localhost assumptions, status endpoints, browser install endpoints, session endpoints, and artifact endpoints.
- CLI tests should verify command output, `--json` shape, exit codes, and daemon-not-running behavior.
- SwiftUI App logic should keep core state transformations outside views where possible, so they can be unit tested without UI automation.
- End-to-end smoke tests should run against a small installed browser set and verify opening a URL, capturing a screenshot, and exposing a noVNC URL.
- Manual HAT should verify a human can search, install, open Chrome 90, inspect a target URL through noVNC, capture a screenshot, and close the session.

## Out of Scope

- Bundling Docker runtime inside the App.
- Supporting Selenoid.
- Supporting arbitrary third-party browser images in MVP.
- Installing old Safari versions.
- Running iOS Safari, Android Chrome, Android WebView, Appium, real devices, or Android Emulator.
- Full visual regression testing system.
- Full test report dashboard with history analytics.
- Video recording in MVP.
- LAN or remote daemon access.
- Multi-user shared Grid.
- System-level privileged daemon.
- Cloud browser providers.
- Automatic PRD-to-issue breakdown in this PRD.

## Further Notes

- The App should be optimized for human acceptance workflows, not only browser inventory management.
- The daemon should be useful even when the App is closed, because local services and AI Agents may call the CLI/API independently.
- The first implementation should prove the Chrome path end to end before expanding to Firefox and Edge.
- Chrome 90 and Chrome 100 are useful initial fixtures because they have already been exercised locally with official Selenium images.
- Apple Silicon support requires careful UI warnings when a selected image runs through amd64 emulation.

## Source Manifest

### Sources

- User conversation in this thread: requirements for local macOS App, CLI, Selenium service, Docker-based browser version management, noVNC manual testing, AI Agent integration, and Dynamic Grid direction.
- `AGENTS.md`: project workflow and Source Manifest requirements.
- `docs/agents/issue-tracker.md`: GitHub issue tracker target and publishing conventions.
- `docs/agents/triage-labels.md`: `ready-for-agent` label mapping.
- `docs/agents/domain.md`: domain documentation lookup expectations.
- Local Selenium experiment evidence from this thread: Chrome 100 and Chrome 90 official Selenium images were run locally, noVNC was used, and screenshots were captured.

### Produced artifacts

- `docs/prds/browserlab-local-selenium-workbench.md`
- GitHub issue: https://github.com/ivan-94/selenium-manager/issues/1

### Key decisions

- Use SwiftUI for the macOS App.
- Use local C/S architecture with a persistent daemon and App/CLI clients.
- Use user-level LaunchAgent for daemon lifecycle.
- Use localhost-only authenticated daemon API.
- Use Selenium Dynamic Grid as the primary execution layer.
- Use official Selenium Docker images only in MVP.
- Treat “open browser” as creating a held WebDriver session.
- Keep Browser Registry as the source of truth and generate runtime config from it.

### Verification evidence

- Project docs were read before producing this PRD.
- `git remote -v` confirmed the repository target as `git@github.com:ivan-94/selenium-manager.git`.
- Prior local runs in this thread verified Selenium Chrome 100 and Chrome 90 can open Baidu and expose noVNC sessions.

### Open questions / risks

- Dynamic Grid noVNC URL resolution and config reload behavior should be prototyped early.
- Docker Hub tag search availability and registry mirror failures need robust UX.
- Chrome old-version images on Apple Silicon may require amd64 emulation and can be slow.
- Firefox and Edge should be added after the Chrome path is proven.
