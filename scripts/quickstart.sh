#!/usr/bin/env bash
#
# Quickstart for AI Coding Assistant (production-style: build frontend -> run backend).
# Requirements: Node.js + npm. (Backend uses the prebuilt binary in `backend/ai-coding-assistant`.)
#
# Usage:
#   ./scripts/quickstart.sh            # init (if needed) + build + run
#   ./scripts/quickstart.sh init       # interactive .env init
#   ./scripts/quickstart.sh build      # build frontend into backend/static
#   ./scripts/quickstart.sh start      # run backend server (uses .env)
#   ./scripts/quickstart.sh wizard     # run first-run web setup wizard
#   ./scripts/quickstart.sh up         # init + build + start
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/.env"
EXAMPLE_ENV="$ROOT/.env.example"
FRONTEND_DIR="$ROOT/frontend"
BACKEND_DIR="$ROOT/backend"
BACKEND_BIN="$BACKEND_DIR/ai-coding-assistant"

FORCE_INIT=0
SKIP_INSTALL=0

usage() {
  cat <<'EOF'
Usage: ./scripts/quickstart.sh [command] [options]

Commands:
  init        Interactive .env initialization
  build       Build frontend into backend/static
  start       Start backend server (reads .env)
  wizard      Run first-run web setup wizard
  up          init + build + start

Options:
  --force         Overwrite existing .env during init
  --skip-install  Skip `npm ci` (assumes frontend/node_modules already exists)
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[ERROR] Missing required command: $1" >&2
    exit 1
  fi
}

format_host_for_url() {
  local host="${1:-}"
  if [[ -z "${host}" ]]; then
    echo ""
    return 0
  fi
  if [[ "${host}" == \[*\] ]]; then
    echo "${host}"
    return 0
  fi
  if [[ "${host}" == *:* ]]; then
    echo "[${host}]"
    return 0
  fi
  echo "${host}"
}

is_private_ipv4() {
  local ip="${1:-}"
  if [[ "${ip}" == 10.* ]]; then
    return 0
  fi
  if [[ "${ip}" == 192.168.* ]]; then
    return 0
  fi
  if [[ "${ip}" =~ ^172\.([0-9]{1,3})\. ]]; then
    local second="${BASH_REMATCH[1]}"
    if [[ "${second}" =~ ^[0-9]+$ ]] && (( second >= 16 && second <= 31 )); then
      return 0
    fi
  fi
  return 1
}

ipv4_scope_label() {
  local ip="${1:-}"
  if [[ -z "${ip}" ]]; then
    echo "unknown"
    return 0
  fi
  if [[ "${ip}" == 127.* ]]; then
    echo "local"
    return 0
  fi
  if [[ "${ip}" == 169.254.* ]]; then
    echo "link"
    return 0
  fi
  if is_private_ipv4 "${ip}"; then
    echo "lan"
    return 0
  fi
  echo "public"
}

list_ipv4_addrs() {
  local out=""

  if command -v ip >/dev/null 2>&1; then
    out="$(ip -o -4 addr show up scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 || true)"
    if [[ -z "${out}" ]]; then
      out="$(ip -o -4 addr show up 2>/dev/null | awk '{print $4}' | cut -d/ -f1 || true)"
    fi
  elif command -v hostname >/dev/null 2>&1; then
    out="$(hostname -I 2>/dev/null | tr ' ' '\n' || true)"
  elif command -v ifconfig >/dev/null 2>&1; then
    out="$(ifconfig 2>/dev/null | awk '/inet /{print $2}' || true)"
  fi

  echo "${out}" | tr ' ' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | grep -v '^127\.' | sort -u
}

print_access_urls() {
  local label="${1:-Service}"
  local bind_host="${2:-}"
  local port="${3:-}"
  local path="${4:-}"

  if [[ -z "${port}" ]]; then
    return 0
  fi

  if [[ -n "${path}" && "${path:0:1}" != "/" ]]; then
    path="/${path}"
  fi

  echo "[INFO] ${label} URLs:"

  if [[ -z "${bind_host}" || "${bind_host}" == "0.0.0.0" || "${bind_host}" == "::" ]]; then
    local ips=()
    while IFS= read -r ip; do
      [[ -n "${ip}" ]] && ips+=("${ip}")
    done < <(list_ipv4_addrs)

    if [[ ${#ips[@]} -gt 0 ]]; then
      for ip in "${ips[@]}"; do
        local scope
        scope="$(ipv4_scope_label "${ip}")"
        echo "[INFO]   - (${scope}) http://$(format_host_for_url "${ip}"):${port}${path}"
      done
    fi

    echo "[INFO]   - (local) http://localhost:${port}${path}"
    echo "[INFO]   - (local) http://127.0.0.1:${port}${path}"
    return 0
  fi

  local show_host
  show_host="$(format_host_for_url "${bind_host}")"
  if [[ -z "${show_host}" ]]; then
    show_host="localhost"
  fi
  echo "[INFO]   - http://${show_host}:${port}${path}"
}

prompt() {
  local label="$1"
  local default_value="$2"
  local out_var="$3"
  local secret="${4:-0}"

  local value=""
  if [[ "$secret" == "1" ]]; then
    # shellcheck disable=SC2162
    read -r -s -p "${label} [${default_value}]: " value
    echo ""
  else
    # shellcheck disable=SC2162
    read -r -p "${label} [${default_value}]: " value
  fi

  if [[ -z "${value}" ]]; then
    value="${default_value}"
  fi

  printf -v "$out_var" "%s" "$value"
}

gen_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return 0
  fi
  if command -v python3 >/dev/null 2>&1; then
    python3 - <<'PY'
import secrets
print(secrets.token_hex(32))
PY
    return 0
  fi
  date +%s%N
}

init_env() {
  if [[ -f "$ENV_FILE" && "$FORCE_INIT" != "1" ]]; then
    echo "[INFO] .env already exists: $ENV_FILE"
    echo "[INFO] Use --force to overwrite."
    return 0
  fi

  if [[ -f "$EXAMPLE_ENV" ]]; then
    echo "[INFO] Using $EXAMPLE_ENV as reference."
  fi

  local server_host server_port db_dsn auth_user auth_pass jwt_secret terminal_shell terminal_login_dir log_level log_file
  local demo_mode

  prompt "SERVER_HOST" "0.0.0.0" server_host
  prompt "SERVER_PORT" "34007" server_port
  prompt "DEMO_MODE (true/false)" "false" demo_mode
  prompt "DATABASE_DSN (relative to backend/)" "./data/aca.db" db_dsn
  prompt "AUTH_USERNAME (first login bootstrap)" "admin" auth_user
  prompt "AUTH_PASSWORD (first login bootstrap)" "admin123" auth_pass 1

  local secret_default
  secret_default="$(gen_secret)"
  prompt "JWT_SECRET" "$secret_default" jwt_secret

  prompt "TERMINAL_SHELL" "/bin/bash" terminal_shell
  prompt "TERMINAL_DEFAULT_LOGIN_DIR" "~/" terminal_login_dir
  prompt "LOG_LEVEL" "info" log_level
  prompt "LOG_FILE (empty=stdout)" "" log_file

  cat >"$ENV_FILE" <<EOF
# Generated by ./scripts/quickstart.sh init

# Server
SERVER_HOST=${server_host}
SERVER_PORT=${server_port}
DEMO_MODE=${demo_mode}

# Database (SQLite)
DATABASE_DSN=${db_dsn}

# Auth
AUTH_USERNAME=${auth_user}
AUTH_PASSWORD=${auth_pass}
JWT_SECRET=${jwt_secret}

# Terminal
TERMINAL_SHELL=${terminal_shell}
TERMINAL_DEFAULT_LOGIN_DIR=${terminal_login_dir}

# Logging
LOG_LEVEL=${log_level}
LOG_FILE=${log_file}
EOF

  echo "[INFO] Wrote $ENV_FILE"
}

load_env() {
  if [[ ! -f "$ENV_FILE" ]]; then
    echo "[ERROR] Missing .env ($ENV_FILE). Run: ./scripts/quickstart.sh init" >&2
    exit 1
  fi
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
}

build_frontend() {
  require_cmd node
  require_cmd npm

  if [[ ! -d "$FRONTEND_DIR" ]]; then
    echo "[ERROR] Missing frontend dir: $FRONTEND_DIR" >&2
    exit 1
  fi

  echo "[INFO] Building frontend..."
  if [[ "$SKIP_INSTALL" != "1" ]]; then
    if [[ ! -d "$FRONTEND_DIR/node_modules" ]]; then
      echo "[INFO] Installing frontend deps (npm ci)..."
      (cd "$FRONTEND_DIR" && npm ci)
    else
      echo "[INFO] frontend/node_modules exists; skipping npm ci (use --skip-install to silence)."
    fi
  else
    echo "[INFO] --skip-install enabled; skipping npm ci."
  fi

  (cd "$FRONTEND_DIR" && npm run build)
  echo "[INFO] Frontend built into backend/static"
}

start_backend() {
  load_env

  if [[ ! -x "$BACKEND_BIN" ]]; then
    echo "[ERROR] Backend binary not found or not executable: $BACKEND_BIN" >&2
    echo "       Expected a prebuilt binary. If you have Go installed, you can build it with:" >&2
    echo "       (cd backend && go build -o ai-coding-assistant .)" >&2
    exit 1
  fi

  cd "$ROOT"

  echo "[INFO] Starting backend..."
  echo "[INFO] Bind: ${SERVER_HOST}:${SERVER_PORT}"
  print_access_urls "Backend" "${SERVER_HOST}" "${SERVER_PORT}" ""
  exec "$BACKEND_BIN"
}

run_setup_wizard() {
  cd "$ROOT"

  if [[ -x "$BACKEND_BIN" ]]; then
    exec "$BACKEND_BIN" setup
  fi

  if command -v go >/dev/null 2>&1; then
    echo "[WARN] Backend binary not found; running setup wizard via Go (go run ./cmd/setup-wizard)..."
    cd "$BACKEND_DIR"
    exec go run ./cmd/setup-wizard
  fi

  echo "[ERROR] Backend binary not found: $BACKEND_BIN" >&2
  echo "        Install Go 1.21+ to run: (cd backend && go run ./cmd/setup-wizard)" >&2
  exit 1
}

COMMAND="${1:-up}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    init|build|start|wizard|up)
      COMMAND="$1"
      shift
      ;;
    --force)
      FORCE_INIT=1
      shift
      ;;
    --skip-install)
      SKIP_INSTALL=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[ERROR] Unknown arg: $1" >&2
      usage
      exit 1
      ;;
  esac
done

case "$COMMAND" in
  init)
    init_env
    ;;
  build)
    build_frontend
    ;;
  start)
    start_backend
    ;;
  wizard)
    run_setup_wizard
    ;;
  up)
    init_env
    build_frontend
    start_backend
    ;;
  *)
    usage
    exit 1
    ;;
esac
