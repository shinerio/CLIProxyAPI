#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BACKEND_SRC="${BACKEND_SRC:-$REPO_ROOT/backend}"
FRONTEND_SRC="${FRONTEND_SRC:-$REPO_ROOT/frontend}"
INSTALL_DIR="${INSTALL_DIR:-}"
SERVICE_NAME="${SERVICE_NAME:-}"
CONFIG_TEMPLATE="${CONFIG_TEMPLATE:-$BACKEND_SRC/config.example.yaml}"

TARGET_BIN=""
TARGET_STATIC_DIR=""
TARGET_CONFIG=""
TARGET_VERSION=""
BACKUP_DIR=""
TMP_DIR=""

DO_FRONTEND=1
DO_RESTART=0
DO_FORCE=0

log() {
  printf '[install-or-update] %s\n' "$*" >&2
}

die() {
  log "$*"
  exit 1
}

usage() {
  cat <<EOF
Usage: $(basename "$0") --install-dir <path> [options]

Options:
  --install-dir <path>   Required. Target install or update directory.
  --service-name <name>  Optional. Restart this systemd service after install/update.
  --config <path>        Optional. Source config template. Default: backend/config.example.yaml
  --no-frontend          Skip rebuilding and reinstalling frontend assets.
  --restart              Restart the service after install/update. Requires --service-name.
  --force                Allow install into a non-empty directory missing expected markers.
  -h, --help             Show this help.

Environment overrides:
  BACKEND_SRC      Default: $REPO_ROOT/backend
  FRONTEND_SRC     Default: $REPO_ROOT/frontend
  INSTALL_DIR      Alternative to --install-dir
  SERVICE_NAME     Alternative to --service-name
  CONFIG_TEMPLATE  Alternative to --config

Behavior:
  - Builds backend from local source with \`go build -mod=readonly\`
  - Builds frontend from local source and installs assets to <install-dir>/static
  - Copies config.example.yaml to <install-dir>/config.yaml on first install only
  - Keeps existing config.yaml untouched on update
  - Writes build metadata to <install-dir>/version.txt
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    die "missing required command: $1"
  fi
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --install-dir)
        [ "$#" -ge 2 ] || die "--install-dir requires a value"
        INSTALL_DIR="$2"
        shift
        ;;
      --service-name)
        [ "$#" -ge 2 ] || die "--service-name requires a value"
        SERVICE_NAME="$2"
        shift
        ;;
      --config)
        [ "$#" -ge 2 ] || die "--config requires a value"
        CONFIG_TEMPLATE="$2"
        shift
        ;;
      --no-frontend)
        DO_FRONTEND=0
        ;;
      --restart)
        DO_RESTART=1
        ;;
      --force)
        DO_FORCE=1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
    shift
  done
}

init_paths() {
  INSTALL_DIR="${INSTALL_DIR%/}"
  [ -n "$INSTALL_DIR" ] || die "--install-dir is required"

  TARGET_BIN="$INSTALL_DIR/cli-proxy-api"
  TARGET_STATIC_DIR="$INSTALL_DIR/static"
  TARGET_CONFIG="$INSTALL_DIR/config.yaml"
  TARGET_VERSION="$INSTALL_DIR/version.txt"
  BACKUP_DIR="$INSTALL_DIR/backups"
}

ensure_sources() {
  [ -d "$BACKEND_SRC" ] || die "backend source dir not found: $BACKEND_SRC"
  [ -f "$BACKEND_SRC/go.mod" ] || die "backend source dir is not a Go module: $BACKEND_SRC"
  [ -f "$CONFIG_TEMPLATE" ] || die "config template not found: $CONFIG_TEMPLATE"

  if [ "$DO_FRONTEND" -eq 1 ]; then
    [ -d "$FRONTEND_SRC" ] || die "frontend source dir not found: $FRONTEND_SRC"
    [ -f "$FRONTEND_SRC/package.json" ] || die "frontend source dir is missing package.json: $FRONTEND_SRC"
    [ -d "$FRONTEND_SRC/node_modules" ] || die "frontend dependencies are missing: $FRONTEND_SRC/node_modules"
  fi
}

ensure_install_dir() {
  if [ ! -e "$INSTALL_DIR" ]; then
    log "creating install dir: $INSTALL_DIR"
    install -d "$INSTALL_DIR"
  elif [ ! -d "$INSTALL_DIR" ]; then
    die "install dir path exists but is not a directory: $INSTALL_DIR"
  elif [ "$DO_FORCE" -ne 1 ] && [ -z "$(find "$INSTALL_DIR" -mindepth 1 -maxdepth 1 2>/dev/null)" ]; then
    :
  elif [ "$DO_FORCE" -ne 1 ] && [ ! -f "$TARGET_BIN" ] && [ ! -f "$TARGET_CONFIG" ] && [ ! -d "$TARGET_STATIC_DIR" ]; then
    die "install dir is non-empty but does not look like an existing install: $INSTALL_DIR (use --force to allow)"
  fi

  install -d "$TARGET_STATIC_DIR"
  install -d "$BACKUP_DIR"
}

make_tmpdir() {
  TMP_DIR="$(mktemp -d "$INSTALL_DIR/.install-tmp.XXXXXX")"
  trap cleanup EXIT
}

cleanup() {
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

build_backend() {
  local commit branch build_date version_line

  commit="$(git -C "$BACKEND_SRC" rev-parse --short HEAD 2>/dev/null || printf 'unknown')"
  branch="$(git -C "$BACKEND_SRC" rev-parse --abbrev-ref HEAD 2>/dev/null || printf 'unknown')"
  build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  version_line="${commit} (${branch}) ${build_date}"

  log "building backend from $BACKEND_SRC"
  (
    cd "$BACKEND_SRC"
    GOCACHE="${GOCACHE:-/tmp/go-build-cache}" go build \
      -mod=readonly \
      -ldflags "-X main.Version=${commit} -X main.Commit=${commit} -X main.BuildDate=${build_date}" \
      -o "$TMP_DIR/cli-proxy-api" \
      ./cmd/server
  )

  printf '%s\n' "$version_line" >"$TMP_DIR/version.txt"
}

build_frontend() {
  if [ "$DO_FRONTEND" -ne 1 ]; then
    return 0
  fi

  log "building frontend from $FRONTEND_SRC"
  (
    cd "$FRONTEND_SRC"
    npm run build
  )

  [ -f "$FRONTEND_SRC/dist/index.html" ] || die "frontend build did not produce dist/index.html"

  install -d "$TMP_DIR/static"
  cp -R "$FRONTEND_SRC/dist/." "$TMP_DIR/static/"
  if [ -f "$TMP_DIR/static/index.html" ] && [ ! -f "$TMP_DIR/static/management.html" ]; then
    cp "$TMP_DIR/static/index.html" "$TMP_DIR/static/management.html"
  fi
  [ -f "$TMP_DIR/static/management.html" ] || die "frontend build did not produce management.html or index.html"
}

backup_existing() {
  if [ ! -f "$TARGET_BIN" ] && [ ! -f "$TARGET_CONFIG" ] && [ ! -f "$TARGET_VERSION" ] && [ ! -d "$TARGET_STATIC_DIR" ]; then
    return 0
  fi

  local stamp backup_path
  stamp="$(date +%Y%m%d-%H%M%S)"
  backup_path="$BACKUP_DIR/$stamp"
  install -d "$backup_path"

  [ -f "$TARGET_BIN" ] && cp "$TARGET_BIN" "$backup_path/cli-proxy-api"
  [ -f "$TARGET_CONFIG" ] && cp "$TARGET_CONFIG" "$backup_path/config.yaml"
  [ -f "$TARGET_VERSION" ] && cp "$TARGET_VERSION" "$backup_path/version.txt"
  [ -d "$TARGET_STATIC_DIR" ] && cp -R "$TARGET_STATIC_DIR" "$backup_path/static"

  log "backup created: $backup_path"
}

install_artifacts() {
  log "installing backend binary"
  install -m 0755 "$TMP_DIR/cli-proxy-api" "$TARGET_BIN"
  install -m 0644 "$TMP_DIR/version.txt" "$TARGET_VERSION"

  if [ ! -f "$TARGET_CONFIG" ]; then
    log "installing initial config.yaml from template"
    install -m 0644 "$CONFIG_TEMPLATE" "$TARGET_CONFIG"
  else
    log "keeping existing config.yaml"
  fi

  if [ "$DO_FRONTEND" -eq 1 ]; then
    log "installing frontend assets"
    install -d "$TARGET_STATIC_DIR"
    cp -R "$TMP_DIR/static/." "$TARGET_STATIC_DIR/"
  fi
}

systemctl_cmd() {
  if [ "$(id -u)" -eq 0 ]; then
    systemctl "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo systemctl "$@"
  else
    die "systemctl requires root or sudo"
  fi
}

restart_service() {
  if [ "$DO_RESTART" -ne 1 ]; then
    return 0
  fi

  [ -n "$SERVICE_NAME" ] || die "--restart requires --service-name"

  log "restarting systemd service: $SERVICE_NAME"
  systemctl_cmd restart "$SERVICE_NAME"
  systemctl_cmd is-active --quiet "$SERVICE_NAME" || die "service restart failed: $SERVICE_NAME"
}

print_summary() {
  cat <<EOF
Install/update complete.

Install dir:  $INSTALL_DIR
Binary:       $TARGET_BIN
Config:       $TARGET_CONFIG
Static dir:   $TARGET_STATIC_DIR
Version file: $TARGET_VERSION
EOF
}

main() {
  parse_args "$@"
  init_paths

  require_cmd install
  require_cmd mktemp
  require_cmd cp
  require_cmd go
  if [ "$DO_FRONTEND" -eq 1 ]; then
    require_cmd npm
  fi

  ensure_sources
  ensure_install_dir
  make_tmpdir
  build_backend
  build_frontend
  backup_existing
  install_artifacts
  restart_service
  print_summary
}

main "$@"
