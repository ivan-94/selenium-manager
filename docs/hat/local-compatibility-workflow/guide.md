# BrowserLab Local Compatibility Workflow HAT Guide

## Metadata

- Source issue: https://github.com/ivan-94/selenium-manager/issues/13
- Parent PRD: https://github.com/ivan-94/selenium-manager/issues/1
- Repo: `ivan-94/selenium-manager`
- Worktree used to create this guide: `/Users/ivan/workspace/ai/selenoid.worktrees/slice-1-13-end-to-end-hat-guide`
- Updated: 2026-05-20
- HAT mode: `blank`
- Preparation status: `syntax-checked`
- Scope: documentation/HAT artifact only. No product code is changed by this slice.

This guide verifies the BrowserLab MVP local compatibility workflow from a clean local HAT home. It covers both the CLI path and the SwiftUI App path for Chrome search, install, held manual session creation, noVNC inspection, screenshot artifact capture, and session close.

## Current Implementation Coverage

Implemented parent PRD slices reflected in this guide:

- #2 Bootstrap daemon, CLI, and SwiftUI status loop.
- #3 Manage daemon lifecycle with user LaunchAgent.
- #4 Search official Selenium Chrome versions.
- #5 Install Chrome versions into Browser Registry.
- #6 Start Selenium Dynamic Grid from installed registry.
- #7 Open held manual browser sessions with noVNC.
- #8 List, inspect, and close active sessions.
- #9 Capture screenshots and session artifacts.
- #10 Chrome mobile emulation presets for manual sessions and screenshots.
- #11 Show native Safari as detected non-installable runtime.
- #12 Disable and uninstall installed browser versions.

Not covered as product behavior in this HAT flow:

- Old Safari installation, Selenoid, third-party images, Android/iOS devices, video artifacts, remote daemon access, and cloud browser providers are outside the MVP PRD scope.

## Prerequisites

Use a local macOS machine. For Apple Silicon, old official Selenium Chrome images may be `linux/amd64` only and will run through Docker amd64 emulation. This is expected, but it can be slow and should be recorded as performance evidence rather than treated as an immediate failure.

Required:

- Docker runtime available locally: Docker Desktop, OrbStack, or Colima.
- Docker CLI can run containers and access `/var/run/docker.sock`.
- Network access to Docker Hub for `selenium/standalone-chrome` and `selenium/standalone-docker` images.
- Go toolchain for building `browserlab` and `browserlabd`.
- macOS 13 or newer and Swift 5.9+ for the SwiftUI App path.
- Optional but recommended: `jq` for parsing CLI JSON output.

Run all commands from the repo root.

```sh
pwd
./docs/hat/local-compatibility-workflow/prepare.sh prepare
source .hat/local-compatibility/env.sh
```

The helper creates:

- `.hat/local-compatibility/bin/browserlab`
- `.hat/local-compatibility/bin/browserlabd`
- `.hat/local-compatibility/browserlab-home/`
- `.hat/local-compatibility/evidence/`
- `.hat/local-compatibility/env.sh`

## Test Data

Default target URL:

```sh
export BL_TARGET_URL="${BL_TARGET_URL:-https://example.com/}"
```

Default browser search target:

```sh
export BL_SEARCH_QUERY="${BL_SEARCH_QUERY:-90}"
```

Default Chrome mobile emulation preset:

```sh
export BL_MOBILE_PRESET="${BL_MOBILE_PRESET:-iphone-14}"
```

Select an exact Chrome version from search output. Do not assume `90` is installable directly; BrowserLab stores exact Selenium image provenance.

```sh
browserlab search chrome "$BL_SEARCH_QUERY" --json | tee "$HAT_EVIDENCE_DIR/search-chrome-${BL_SEARCH_QUERY}.json"
```

If `jq` is available:

```sh
export BL_VERSION="$(jq -r '.results[0].browserVersion' "$HAT_EVIDENCE_DIR/search-chrome-${BL_SEARCH_QUERY}.json")"
export BL_IMAGE_TAG="$(jq -r '.results[0].imageTag' "$HAT_EVIDENCE_DIR/search-chrome-${BL_SEARCH_QUERY}.json")"
```

If `jq` is unavailable, manually copy `results[0].browserVersion` and `results[0].imageTag` from the JSON file.

Expected search evidence:

- JSON file exists at `.hat/local-compatibility/evidence/search-chrome-90.json`.
- At least one result has `browserName: "chrome"`, an exact `browserVersion`, an `imageTag` under `selenium/standalone-chrome`, and `recommended: true`.
- On Apple Silicon, results may include a warning that Chrome Selenium images are `linux/amd64` only and run through amd64 emulation.

## Setup

Start an isolated daemon for this HAT run:

```sh
source .hat/local-compatibility/env.sh
"$HAT_BIN_DIR/browserlabd" > "$HAT_ROOT/browserlabd.log" 2>&1 &
echo $! > "$HAT_ROOT/browserlabd.pid"
sleep 2
browserlab status --json | tee "$HAT_EVIDENCE_DIR/status.json"
```

Expected:

- `status.json` has `state: "running"`.
- `api.bind` is `127.0.0.1:49321`.
- `api.localhostOnly` is `true`.
- `paths.configDir`, `paths.logsDir`, and `paths.artifactsDir` point under `.hat/local-compatibility/browserlab-home/`.

If port `49321` is already in use, stop the existing BrowserLab daemon before this HAT run. The current daemon uses the fixed local bind address.

## CLI Acceptance Path

### P0. Search Official Selenium Chrome Versions

Steps:

```sh
browserlab search chrome "$BL_SEARCH_QUERY" --json | tee "$HAT_EVIDENCE_DIR/cli-search.json"
browserlab search chrome "$BL_SEARCH_QUERY" | tee "$HAT_EVIDENCE_DIR/cli-search.txt"
```

Expected:

- JSON output is machine-readable.
- Human output lists `Chrome <version>`, `Image: selenium/standalone-chrome:<tag>`, driver/grid metadata when available, platforms, and warnings.
- Apple Silicon old-version results call out amd64 emulation when the Docker Hub platform data requires it.

Evidence:

- `.hat/local-compatibility/evidence/cli-search.json`
- `.hat/local-compatibility/evidence/cli-search.txt`

### P0. Install Selected Chrome Version

Steps:

```sh
browserlab install chrome "$BL_VERSION" --image-tag "$BL_IMAGE_TAG" --json | tee "$HAT_EVIDENCE_DIR/cli-install.json"
browserlab browsers --json | tee "$HAT_EVIDENCE_DIR/cli-browsers-after-install.json"
```

Expected:

- Install returns exit code `0`.
- `record.family` is `chrome`.
- `record.version` matches `$BL_VERSION`.
- `record.imageTag` matches `$BL_IMAGE_TAG`.
- `record.enabled` is `true`.
- On Apple Silicon, `record.platform` may be `linux/amd64`; capture warnings as evidence.

Evidence:

- `.hat/local-compatibility/evidence/cli-install.json`
- `.hat/local-compatibility/evidence/cli-browsers-after-install.json`
- Registry file: `.hat/local-compatibility/browserlab-home/config/browser-registry.json`

### P0. Start Selenium Dynamic Grid

Steps:

```sh
browserlab grid start --json | tee "$HAT_EVIDENCE_DIR/cli-grid-start.json"
browserlab grid status --json | tee "$HAT_EVIDENCE_DIR/cli-grid-status.json"
browserlab grid config | tee "$HAT_EVIDENCE_DIR/cli-grid-config.toml"
```

Expected:

- Grid state is `running`.
- WebDriver endpoint is `http://127.0.0.1:4444/wd/hub`.
- Grid UI is `http://127.0.0.1:4444`.
- Generated config includes the selected Chrome image tag and browser version.
- Docker container `browserlab-selenium-grid` exists and is running.

Evidence:

- `.hat/local-compatibility/evidence/cli-grid-start.json`
- `.hat/local-compatibility/evidence/cli-grid-status.json`
- `.hat/local-compatibility/evidence/cli-grid-config.toml`
- Generated config: `.hat/local-compatibility/browserlab-home/config/selenium-docker.toml`
- Selenium Grid assets: `.hat/local-compatibility/browserlab-home/artifacts/selenium-grid/`

### P0. Open Held Manual Session and noVNC

Steps:

```sh
browserlab session open chrome "$BL_VERSION" "$BL_TARGET_URL" --json | tee "$HAT_EVIDENCE_DIR/cli-session-open.json"
export BL_SESSION_ID="$(jq -r '.sessionId' "$HAT_EVIDENCE_DIR/cli-session-open.json")"
export BL_NOVNC_URL="$(jq -r '.noVnc.url' "$HAT_EVIDENCE_DIR/cli-session-open.json")"
browserlab sessions list --json | tee "$HAT_EVIDENCE_DIR/cli-sessions-list-after-open.json"
browserlab sessions inspect "$BL_SESSION_ID" --json | tee "$HAT_EVIDENCE_DIR/cli-session-inspect.json"
```

If `jq` is unavailable, manually copy `sessionId` and `noVnc.url` from `cli-session-open.json`.

Open `$BL_NOVNC_URL` in a browser and manually verify:

- noVNC opens the selected session, not only the Selenium desktop background.
- Chrome is visible and usable.
- The target page loads, or the failure is a real page/network failure visible in Chrome.
- The session remains open until the explicit close step.

Expected:

- Session output has `status: "active"`.
- `browserName` is `chrome`.
- `browserVersion` matches `$BL_VERSION`.
- `requestedUrl` matches `$BL_TARGET_URL`.
- `noVnc.url` points to `http://127.0.0.1:4444/ui/#/sessions/<session-id>`.
- `noVnc.vncWebSocketUrl` points to the Grid-routed websocket on `127.0.0.1:4444`.

Evidence:

- `.hat/local-compatibility/evidence/cli-session-open.json`
- `.hat/local-compatibility/evidence/cli-sessions-list-after-open.json`
- `.hat/local-compatibility/evidence/cli-session-inspect.json`
- Manual note in the execution record with the observed noVNC URL and page state.

### P0. Capture Screenshot From Held Session

Steps:

```sh
browserlab session screenshot "$BL_SESSION_ID" \
  --browser chrome \
  --version "$BL_VERSION" \
  --webdriver-endpoint "http://127.0.0.1:4444/wd/hub" \
  --requested-url "$BL_TARGET_URL" \
  --json | tee "$HAT_EVIDENCE_DIR/cli-session-screenshot.json"
```

Expected:

- Screenshot command returns exit code `0`.
- Response has `groupType: "session"` and `groupId` equal to the session ID.
- The screenshot, metadata, result, and summary files exist.
- The screenshot is visually consistent with the noVNC browser state.

Expected artifact paths:

- `.hat/local-compatibility/browserlab-home/artifacts/sessions/<session-id>/<timestamp>-screenshot.png`
- `.hat/local-compatibility/browserlab-home/artifacts/sessions/<session-id>/<timestamp>-metadata.json`
- `.hat/local-compatibility/browserlab-home/artifacts/sessions/<session-id>/<timestamp>-result.json`
- `.hat/local-compatibility/browserlab-home/artifacts/sessions/<session-id>/<timestamp>-summary.md`

Evidence:

- `.hat/local-compatibility/evidence/cli-session-screenshot.json`
- Screenshot artifact files listed in that JSON response.

### P0. Close Held Session

Steps:

```sh
browserlab sessions close "$BL_SESSION_ID" --json | tee "$HAT_EVIDENCE_DIR/cli-session-close.json"
browserlab sessions list --json | tee "$HAT_EVIDENCE_DIR/cli-sessions-list-after-close.json"
```

Expected:

- Close returns exit code `0`.
- Closed session status is `closed`.
- The session no longer appears in active sessions.
- noVNC no longer controls an active browser for that session.

Evidence:

- `.hat/local-compatibility/evidence/cli-session-close.json`
- `.hat/local-compatibility/evidence/cli-sessions-list-after-close.json`

### P1. Short-Lived Screenshot Run

This verifies the Agent-oriented screenshot path that creates and closes its own WebDriver session.

Steps:

```sh
browserlab screenshot chrome "$BL_VERSION" "$BL_TARGET_URL" --json | tee "$HAT_EVIDENCE_DIR/cli-screenshot-run.json"
```

Expected:

- Response has `groupType: "run"`.
- A transient `sessionId` and non-empty `runId` are present.
- Artifact files are written under `.hat/local-compatibility/browserlab-home/artifacts/runs/<run-id>/`.
- The transient WebDriver session is not left in `browserlab sessions list`.

Evidence:

- `.hat/local-compatibility/evidence/cli-screenshot-run.json`
- Artifact files listed in the response.

### P1. Mobile Emulation Screenshot Run

This verifies the Chrome mobile emulation path with a real Selenium screenshot run.

Steps:

```sh
browserlab screenshot chrome "$BL_VERSION" "$BL_TARGET_URL" \
  --mobile-preset "$BL_MOBILE_PRESET" \
  --json | tee "$HAT_EVIDENCE_DIR/cli-mobile-screenshot-run.json"
```

Expected:

- Response has `mobileEmulation.presetId` equal to `$BL_MOBILE_PRESET`.
- Response records non-empty `mobileEmulation.deviceMetrics` and `mobileEmulation.userAgent`.
- Artifact metadata JSON records the same mobile preset, metrics, and user agent.
- Screenshot dimensions and visual layout are consistent with the selected preset.

Evidence:

- `.hat/local-compatibility/evidence/cli-mobile-screenshot-run.json`
- Artifact metadata and screenshot files listed in the response.

## App Acceptance Path

Run the App from the same shell so it uses the isolated `BROWSERLAB_HOME`:

```sh
source .hat/local-compatibility/env.sh
cd BrowserLabApp
swift run BrowserLabApp
```

The App path exercises the same daemon API as the CLI. Use the CLI evidence above to cross-check App results when needed.

### P0. Daemon Status and Lifecycle Controls

Steps:

1. Confirm the top status panel shows the daemon as running.
2. Confirm the detail shows `127.0.0.1:49321`.
3. Use Logs only if diagnosing startup issues.

Expected:

- App treats non-local daemon binding as an error.
- Missing token or stopped daemon appears as visible error/stopped state.

Evidence:

- Manual note in the execution record.
- Optional screenshot of the App status panel saved outside BrowserLab artifacts and referenced in the execution record.

### P0. App Search and Install

Steps:

1. Enter the same Chrome query, for example `90`.
2. Click `Search`.
3. Confirm results show browser version first, exact Selenium image tag, driver/grid metadata, platforms, recommendation status, and warnings.
4. Click `Install` for the selected result.

Expected:

- Install progress or already-installed state appears in the result row.
- The selected browser appears in `Installed Browsers`.
- On Apple Silicon, amd64 emulation warning is visible either in search result details, install messages, or installed browser platform.

Evidence:

- Manual note in the execution record.
- Registry evidence can be reused from `.hat/local-compatibility/browserlab-home/config/browser-registry.json`.

### P0. App Manual Session, Embedded noVNC, Screenshot, and Close

Steps:

1. In `Installed Browsers`, enter `$BL_TARGET_URL` in the URL field.
2. Select `$BL_MOBILE_PRESET` in the `Mobile` picker, or keep `Desktop` for the desktop control path.
3. Click `Open` for the selected Chrome version.
4. Confirm an active session appears.
5. Confirm the embedded noVNC view loads and shows the browser session.
6. Click `Open External` and confirm the external noVNC URL opens the same session.
7. If a mobile preset was selected, confirm the active session details show the mobile preset name.
8. Click `Capture` while the session is active.
9. Confirm `Recent Artifacts` lists screenshot, metadata, and summary paths.
10. In `Active Sessions`, click `Close`.
11. Confirm the session disappears from active sessions.

Expected:

- The App-created session has the same fields as CLI-created sessions: session ID, Chrome version, requested URL, current URL/title when WebDriver returns them, Grid URL, WebDriver endpoint, and noVNC URL.
- `Capture` writes the same session artifact shape as the CLI path.
- Mobile preset selection writes the same mobile emulation metadata as the CLI path.
- `Close` frees the browser slot and removes the session from active sessions.

Evidence:

- BrowserLab artifact paths listed by `Recent Artifacts`.
- `.hat/local-compatibility/browserlab-home/artifacts/sessions/<session-id>/`.
- Manual execution record noting embedded noVNC and external noVNC behavior.

## Failure Diagnostics

Use this table before marking the HAT failed.

| Symptom | Likely cause | Diagnostic commands | Expected evidence |
| --- | --- | --- | --- |
| `daemon_unreachable` | `browserlabd` is not running or port is occupied | `cat "$HAT_ROOT/browserlabd.log"`; `lsof -nP -iTCP:49321 -sTCP:LISTEN` | Log file and listener owner |
| HTTP 401 / unauthorized | CLI/App is reading the wrong token or `BROWSERLAB_HOME` | `echo "$BROWSERLAB_HOME"`; `ls -l "$BROWSERLAB_HOME/config/token"` | Token path under `.hat/local-compatibility/browserlab-home` |
| `catalog_search_failed` | Docker Hub unavailable or network blocked | Re-run search; open `.hat/local-compatibility/evidence/cli-search.json` if present | HTTP/error message from daemon |
| `pull_failed` | Docker unavailable, network blocked, or image platform unsupported | `docker info`; `docker pull "$BL_IMAGE_TAG"` or `docker pull --platform "$(jq -r '.record.platform' "$HAT_EVIDENCE_DIR/cli-install.json")" "$BL_IMAGE_TAG"` | Docker error and install JSON/problem |
| `platform_mismatch` | Image platforms do not include host arch or amd64 fallback | Inspect search JSON `platforms` and `warnings` | Search result platform list |
| `no_enabled_browsers` | Grid started before install or browser disabled | `browserlab browsers --json`; `browserlab grid config` | Registry and generated config |
| `grid_start_failed` | Docker socket/container error or port 4444 occupied | `docker ps -a --filter name=browserlab-selenium-grid`; `lsof -nP -iTCP:4444 -sTCP:LISTEN` | Grid status JSON and Docker output |
| noVNC opens Selenium background only | Wrong session URL or session died | `browserlab sessions inspect "$BL_SESSION_ID" --json`; `docker logs browserlab-selenium-grid` | Inspect output and Grid logs |
| `webdriver_session_failed` | Grid cannot create browser container | `browserlab grid status --json`; `docker logs browserlab-selenium-grid`; `docker ps -a` | Grid logs and session problem |
| `navigation_failed` | Target URL unreachable from browser container | Try `https://example.com/`; inspect browser in noVNC | noVNC observation and error JSON |
| `screenshot_failed` | Session stale, WebDriver endpoint wrong, or browser crashed | `browserlab sessions inspect "$BL_SESSION_ID" --json`; `docker logs browserlab-selenium-grid` | Screenshot problem and logs |
| App shows stopped while CLI works | App not launched from shell with HAT env or missing token | Relaunch with `source .hat/local-compatibility/env.sh && cd BrowserLabApp && swift run BrowserLabApp` | App status and env path |

## Cleanup

Close active sessions first:

```sh
browserlab sessions list --json | tee "$HAT_EVIDENCE_DIR/cleanup-sessions-before.json"
```

For each remaining active session:

```sh
browserlab sessions close "<session-id>" --json | tee "$HAT_EVIDENCE_DIR/cleanup-close-<session-id>.json"
```

Stop Grid and daemon:

```sh
browserlab grid stop --json | tee "$HAT_EVIDENCE_DIR/cleanup-grid-stop.json" || true
if [ -f "$HAT_ROOT/browserlabd.pid" ]; then
  kill "$(cat "$HAT_ROOT/browserlabd.pid")" 2>/dev/null || true
fi
```

Remove the HAT working directory only when evidence has been copied or the run no longer needs to be inspected:

```sh
./docs/hat/local-compatibility-workflow/prepare.sh cleanup
```

Docker images are retained by default because they may be shared outside this HAT run. If a human explicitly wants image deletion, use:

```sh
browserlab browsers uninstall --image-tag "$BL_IMAGE_TAG" --delete-image --confirm-delete-image --json
```

## Pass Criteria

- All P0 CLI checks pass.
- App P0 path can complete search, install or already-installed confirmation, manual session open, embedded noVNC, external noVNC, screenshot capture, and close session.
- Screenshot artifacts exist and are visually attributable to the selected session or run.
- Any Apple Silicon amd64 emulation warning is visible and recorded.
- No active sessions remain after cleanup.
- Grid and daemon are stopped or intentionally left running with a note.
- P1 short-lived screenshot run either passes or has a clear product/runtime diagnostic.

## Execution Record Template

Copy this section into a run note or HAT summary when executing.

```text
Run date:
Executor:
Machine:
Architecture:
Docker runtime:
Repo commit:
BROWSERLAB_HOME:
Selected Chrome version:
Selected image tag:
Target URL:

P0 CLI search:
P0 CLI install:
P0 CLI grid:
P0 CLI manual session:
P0 CLI noVNC:
P0 CLI session screenshot:
P0 CLI close:
P0 App status:
P0 App search/install:
P0 App session/noVNC/screenshot/close:
P1 screenshot run:
P1 mobile screenshot run:

Evidence directory:
Session artifact directory:
Run artifact directory:
Failures/diagnostics:
Cleanup status:
Final verdict:
```

## Source Manifest

### Sources

- Parent PRD: https://github.com/ivan-94/selenium-manager/issues/1
- Slice issue: https://github.com/ivan-94/selenium-manager/issues/13
- Local PRD artifact: `docs/prds/browserlab-local-selenium-workbench.md`
- Project instructions: `AGENTS.md`
- Workflow requirements: `/Users/ivan/.agents/docs/agents/workflows.md`
- Handoff policy and Source Manifest requirements: `/Users/ivan/.agents/docs/agents/handoff-policy.md`
- HAT preparation skill instructions: `/Users/ivan/.agents/skills/hat-prepare/SKILL.md`
- Runtime support conventions: `docs/runtime/app-support.md`
- Mobile emulation implementation: `internal/browserlab/emulation/catalog.go`
- CLI/API implementation: `internal/browserlab/cli/cli.go`, `internal/browserlab/api/status.go`
- Browser/session/artifact implementation: `internal/browserlab/catalog/catalog.go`, `internal/browserlab/install/install.go`, `internal/browserlab/grid/grid.go`, `internal/browserlab/grid/manager.go`, `internal/browserlab/session/session.go`, `internal/browserlab/artifact/artifact.go`
- SwiftUI App implementation: `BrowserLabApp/Sources/BrowserLabApp/BrowserLabApp.swift`, `BrowserLabApp/Sources/BrowserLabApp/BrowserSearch.swift`, `BrowserLabApp/Sources/BrowserLabApp/DaemonStatus.swift`, `BrowserLabApp/Sources/BrowserLabApp/DaemonLifecycle.swift`

### Produced artifacts

- `docs/hat/local-compatibility-workflow/guide.md`
- `docs/hat/local-compatibility-workflow/prepare.sh`

### Key decisions

- Use `blank` HAT mode with an isolated `.hat/local-compatibility/browserlab-home` via `BROWSERLAB_HOME`.
- Build HAT binaries under `.hat/local-compatibility/bin` instead of requiring global installation.
- Keep Docker image deletion out of default cleanup because images may be shared.
- Cover #10 mobile emulation through both CLI screenshot and App picker/manual-session paths.
- Treat Apple Silicon `linux/amd64` Chrome images as expected emulation behavior that must be visible in evidence.

### Verification evidence

- `bash -n docs/hat/local-compatibility-workflow/prepare.sh`
- `./docs/hat/local-compatibility-workflow/prepare.sh info`
- `go test ./...`
- `swift test` from `BrowserLabApp/`

### Open questions / risks

- Full HAT execution still depends on Docker Hub availability and local Docker runtime state.
- The daemon currently binds fixed port `127.0.0.1:49321`; an existing local daemon can block isolated HAT startup.
- noVNC and old Chrome startup can be slow on Apple Silicon because official Selenium Chrome images may run through amd64 emulation.
- App execution with `swift run BrowserLabApp` requires a graphical macOS session.
