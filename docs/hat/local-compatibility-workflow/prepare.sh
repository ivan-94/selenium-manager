#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
HAT_ROOT="${HAT_ROOT:-$ROOT_DIR/.hat/local-compatibility}"
HAT_BIN_DIR="$HAT_ROOT/bin"
HAT_EVIDENCE_DIR="$HAT_ROOT/evidence"
BROWSERLAB_HOME="${BROWSERLAB_HOME:-$HAT_ROOT/browserlab-home}"

usage() {
  cat <<'USAGE'
usage: prepare.sh <info|prepare|cleanup>

info      Print BrowserLab local compatibility HAT paths and commands.
prepare   Build HAT binaries and create an isolated BROWSERLAB_HOME.
cleanup   Stop the HAT Grid container if present and remove only .hat/local-compatibility.
USAGE
}

write_env() {
  mkdir -p "$HAT_ROOT"
  cat > "$HAT_ROOT/env.sh" <<EOF
export HAT_ROOT="$HAT_ROOT"
export HAT_BIN_DIR="$HAT_BIN_DIR"
export HAT_EVIDENCE_DIR="$HAT_EVIDENCE_DIR"
export BROWSERLAB_HOME="$BROWSERLAB_HOME"
export PATH="$HAT_BIN_DIR:\$PATH"
EOF
}

info() {
  write_env
  cat <<EOF
HAT_PREPARE_SUMMARY
mode=blank
status=info
repo=$ROOT_DIR
hat_root=$HAT_ROOT
bin_dir=$HAT_BIN_DIR
evidence_dir=$HAT_EVIDENCE_DIR
browserlab_home=$BROWSERLAB_HOME
app_url=not_applicable_local_macos_app
daemon_url=http://127.0.0.1:49321
webdriver_endpoint=http://127.0.0.1:4444/wd/hub
grid_ui=http://127.0.0.1:4444
guide=$ROOT_DIR/docs/hat/local-compatibility-workflow/guide.md
env=$HAT_ROOT/env.sh
END_HAT_PREPARE_SUMMARY
EOF
}

prepare() {
  command -v go >/dev/null 2>&1 || {
    echo "go is required to build browserlab and browserlabd" >&2
    exit 1
  }

  mkdir -p "$HAT_BIN_DIR" "$HAT_EVIDENCE_DIR" "$BROWSERLAB_HOME/config" "$BROWSERLAB_HOME/logs" "$BROWSERLAB_HOME/artifacts"
  write_env

  (cd "$ROOT_DIR" && go build -o "$HAT_BIN_DIR/browserlab" ./cmd/browserlab)
  (cd "$ROOT_DIR" && go build -o "$HAT_BIN_DIR/browserlabd" ./cmd/browserlabd)

  cat <<EOF
HAT_PREPARE_SUMMARY
mode=blank
status=prepared
repo=$ROOT_DIR
hat_root=$HAT_ROOT
bin_dir=$HAT_BIN_DIR
evidence_dir=$HAT_EVIDENCE_DIR
browserlab_home=$BROWSERLAB_HOME
daemon=$HAT_BIN_DIR/browserlabd
cli=$HAT_BIN_DIR/browserlab
daemon_url=http://127.0.0.1:49321
webdriver_endpoint=http://127.0.0.1:4444/wd/hub
grid_ui=http://127.0.0.1:4444
cleanup=$ROOT_DIR/docs/hat/local-compatibility-workflow/prepare.sh cleanup
guide=$ROOT_DIR/docs/hat/local-compatibility-workflow/guide.md
env=$HAT_ROOT/env.sh
END_HAT_PREPARE_SUMMARY
EOF
}

cleanup() {
  if command -v docker >/dev/null 2>&1; then
    docker rm -f browserlab-selenium-grid >/dev/null 2>&1 || true
  fi
  if [ -f "$HAT_ROOT/browserlabd.pid" ]; then
    kill "$(cat "$HAT_ROOT/browserlabd.pid")" >/dev/null 2>&1 || true
  fi
  rm -rf "$HAT_ROOT"
  cat <<EOF
HAT_PREPARE_SUMMARY
mode=blank
status=cleaned
hat_root=$HAT_ROOT
images=retained
END_HAT_PREPARE_SUMMARY
EOF
}

case "${1:-}" in
  info)
    info
    ;;
  prepare)
    prepare
    ;;
  cleanup)
    cleanup
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    exit 64
    ;;
esac
