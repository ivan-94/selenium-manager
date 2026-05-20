# BrowserLab Application Support Conventions

BrowserLab stores local daemon state under the current user's Application Support area:

```text
~/Library/Application Support/BrowserLab/
├── artifacts/
├── config/
│   ├── config.json
│   └── token
└── logs/
```

For tests and isolated agent runs, set `BROWSERLAB_HOME` to redirect the same layout to a temporary directory.

## Files

- `config/token`: local bearer token shared by `browserlabd`, the CLI, and the SwiftUI App. The daemon API expects `Authorization: Bearer <token>` for authenticated endpoints such as `GET /v1/status`.
- `config/config.json`: initial daemon config file. The bootstrap slice writes the default localhost API bind value, `127.0.0.1:49321`.
- `logs/`: daemon log directory convention for later lifecycle work.
- `artifacts/`: session artifact directory convention for later browser/session work.

## Local API

The bootstrap daemon binds to `127.0.0.1:49321` and refuses non-local bind addresses in this slice. It exposes:

- `GET /healthz`: bootstrap-safe unauthenticated health check.
- `GET /v1/status`: token-protected daemon status used by the CLI and SwiftUI App.

## Source Manifest

### Sources

- GitHub issue #2: https://github.com/ivan-94/selenium-manager/issues/2
- Parent PRD #1: https://github.com/ivan-94/selenium-manager/issues/1
- Local PRD artifact: `docs/prds/browserlab-local-selenium-workbench.md`
- Project rules: `AGENTS.md`
- Workflow and handoff rules: `/Users/ivan/.agents/docs/agents/workflows.md`, `/Users/ivan/.agents/docs/agents/handoff-policy.md`

### Produced artifacts

- `docs/runtime/app-support.md`

### Key decisions

- Use a standard per-user macOS Application Support root by default.
- Keep `BROWSERLAB_HOME` as a test and isolated-worker override.
- Use a bootstrap-safe `/healthz` endpoint plus token-protected daemon status for real clients.

### Verification evidence

- `go test ./...` covers config/token initialization and daemon/CLI status behavior.
- `swift test` in `BrowserLabApp/` covers SwiftUI status view-model state mapping.

### Open questions / risks

- LaunchAgent lifecycle, daemon start/stop controls, Docker/Grid status, browser sessions, and artifact production are intentionally deferred to later PRD slices.
