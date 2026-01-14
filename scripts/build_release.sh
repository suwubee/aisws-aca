#!/usr/bin/env bash
# Build a distributable "release/" folder (backend binary + static frontend + start scripts).
#
# Usage:
#   ./scripts/build_release.sh
#   ./scripts/build_release.sh --skip-frontend
#   ./scripts/build_release.sh --skip-windows
#
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$PROJECT_ROOT/backend"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
RELEASE_DIR="$PROJECT_ROOT/release"
GOCACHE_DIR="$BACKEND_DIR/.gocache"
GOMODCACHE_DIR="$BACKEND_DIR/.gomodcache"

SKIP_FRONTEND=0
SKIP_WINDOWS=0

log() { echo "[build_release] $*"; }
die() { echo "[build_release][ERROR] $*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-frontend)
      SKIP_FRONTEND=1
      shift
      ;;
    --skip-windows)
      SKIP_WINDOWS=1
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage: ./scripts/build_release.sh [options]

Options:
  --skip-frontend   Do not run frontend build (assumes backend/static already up-to-date)
  --skip-windows    Do not build windows .exe
EOF
      exit 0
      ;;
    *)
      die "Unknown arg: $1"
      ;;
  esac
done

if [[ ! -d "$BACKEND_DIR" ]]; then
  die "Missing backend dir: $BACKEND_DIR"
fi
if [[ ! -d "$FRONTEND_DIR" ]]; then
  die "Missing frontend dir: $FRONTEND_DIR"
fi
if [[ ! -d "$RELEASE_DIR" ]]; then
  die "Missing release dir (tracked): $RELEASE_DIR"
fi

if [[ "$SKIP_FRONTEND" != "1" ]]; then
  command -v node >/dev/null 2>&1 || die "Missing node (required to build frontend)."
  command -v npm >/dev/null 2>&1 || die "Missing npm (required to build frontend)."

  log "Building frontend -> backend/static ..."
  (cd "$FRONTEND_DIR" && npm run build)
else
  log "Skipping frontend build."
fi

command -v go >/dev/null 2>&1 || die "Missing go (required to build backend)."

log "Building backend (linux) -> release/ai-coding-assistant ..."
mkdir -p "$GOCACHE_DIR" "$GOMODCACHE_DIR"
(cd "$BACKEND_DIR" && GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" go build -o "$RELEASE_DIR/ai-coding-assistant" .)

if [[ "$SKIP_WINDOWS" != "1" ]]; then
  log "Building backend (windows) -> release/ai-coding-assistant.exe ..."
  (cd "$BACKEND_DIR" && GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$RELEASE_DIR/ai-coding-assistant.exe" .)
else
  log "Skipping windows build."
fi

if [[ ! -d "$BACKEND_DIR/static" ]]; then
  die "Missing backend/static (frontend build output)."
fi

log "Syncing static assets -> release/static ..."
rm -rf "$RELEASE_DIR/static"
cp -R "$BACKEND_DIR/static" "$RELEASE_DIR/static"

log "Syncing env example -> release/.env.example ..."
if [[ -f "$PROJECT_ROOT/.env.example" ]]; then
  cp "$PROJECT_ROOT/.env.example" "$RELEASE_DIR/.env.example"
fi

chmod +x "$RELEASE_DIR/start.sh" "$RELEASE_DIR/scripts/start.sh" 2>/dev/null || true

log "Done."
log "Run:"
log "  cd release && ./start.sh"
