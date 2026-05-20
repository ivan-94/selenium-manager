# Selenium Manager

Initial workspace for `ivan-94/selenium-manager`.

## Status

This repository has been reset to a clean project scaffold. Add runtime code, documentation, and tests as the project shape becomes clear.

## Agent setup

Agent workflow configuration lives in:

- `AGENTS.md`
- `docs/agents/issue-tracker.md`
- `docs/agents/triage-labels.md`
- `docs/agents/domain.md`

## Runtime bootstrap

This scaffold now includes the first BrowserLab local control loop:

- `cmd/browserlabd`: localhost-only daemon with `GET /healthz` and token-protected `GET /v1/status`.
- `cmd/browserlab`: CLI with `browserlab status` and `browserlab status --json`.
- `BrowserLabApp/`: minimal SwiftUI status app that reads daemon state from the same API.

Application Support conventions are documented in `docs/runtime/app-support.md`.
