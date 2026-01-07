#!/usr/bin/env bash

set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

# 默认端口（可通过环境变量覆盖）
# - 后端: 36017
# - 前端: 36011
BACKEND_PORT="${BACKEND_PORT:-36017}"
FRONTEND_PORT="${FRONTEND_PORT:-36011}"

# Backend (Go)
export SERVER_HOST="${SERVER_HOST:-0.0.0.0}"
export SERVER_PORT="$BACKEND_PORT"

# Frontend (Vite dev server) -> proxy /api to backend
export ACA_BACKEND_HOST="${ACA_BACKEND_HOST:-localhost}"
export ACA_BACKEND_PORT="$BACKEND_PORT"
export ACA_FRONTEND_PORT="$FRONTEND_PORT"

# 避免 /root/.cache/go-build 权限问题（使用工作区内缓存）
export GOCACHE="${GOCACHE:-$ROOT_DIR/.gocache}"
mkdir -p "$GOCACHE"

echo "[ACA] Backend  : http://localhost:${BACKEND_PORT}"
echo "[ACA] Frontend : http://localhost:${FRONTEND_PORT}"

cleanup() {
  echo ""
  echo "[ACA] Stopping..."
  if [[ -n "${BACKEND_PID:-}" ]]; then
    kill "$BACKEND_PID" 2>/dev/null || true
  fi
  if [[ -n "${FRONTEND_PID:-}" ]]; then
    kill "$FRONTEND_PID" 2>/dev/null || true
  fi
  if [[ -n "${BACKEND_PID:-}" ]]; then
    wait "$BACKEND_PID" 2>/dev/null || true
  fi
  if [[ -n "${FRONTEND_PID:-}" ]]; then
    wait "$FRONTEND_PID" 2>/dev/null || true
  fi
}

trap cleanup EXIT INT TERM

echo "[ACA] Starting backend on :${BACKEND_PORT} ..."
(cd "$BACKEND_DIR" && go run main.go) &
BACKEND_PID=$!

echo "[ACA] Starting frontend on :${FRONTEND_PORT} ..."
(cd "$FRONTEND_DIR" && npm run dev -- --host 0.0.0.0 --port "$FRONTEND_PORT") &
FRONTEND_PID=$!

wait "$BACKEND_PID" "$FRONTEND_PID"

