#!/usr/bin/env bash
# ACA Release - Linux starter (no Node required; embedded/disk static)
#
# Usage:
#   ./start.sh                 # interactive guide (TTY)
#   ./start.sh start           # start backend
#   ./start.sh stop            # stop backend
#   ./start.sh restart         # restart backend
#   ./start.sh status          # show status
#   ./start.sh logs            # tail logs
#   ./start.sh setup           # run setup wizard (foreground)
#   ./start.sh demo            # start in demo mode (uses demo.db)
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

ENV_FILE="$ROOT/.env"
BACKEND_BIN="$ROOT/ai-coding-assistant"
DEMO_DB_SOURCE="$ROOT/data/demo.db"
DATA_DIR="$ROOT/data"

RUNTIME_DIR="$ROOT/.aca"
LOG_DIR="$RUNTIME_DIR/logs"
PID_DIR="$RUNTIME_DIR/pids"
BACKEND_LOG="$LOG_DIR/backend.log"
BACKEND_PID_FILE="$PID_DIR/backend.pid"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
  cat <<'EOF'
Usage: ./start.sh [command] [options]

Commands:
  guide     Step-by-step interactive guide (default when run in a TTY without args)
  setup     Run web setup wizard (first run)
  start     Start backend (serves UI + API)
  stop      Stop backend
  restart   Restart backend
  status    Show status and URLs
  logs      Tail backend logs
  demo      Start in demo mode (read-only, uses demo.db)

Options:
  --takeover-ports   Kill any process listening on SERVER_PORT before starting

Env (via .env):
  SERVER_HOST, SERVER_PORT, DATABASE_DSN, AUTH_USERNAME, AUTH_PASSWORD, JWT_SECRET, DEMO_MODE, ...
EOF
}

is_interactive() {
  [[ -t 0 && -t 1 ]]
}

confirm() {
  local prompt_text="$1"
  local answer=""
  read -r -p "${prompt_text} [y/N]: " answer || true
  case "${answer}" in
    y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

load_env() {
  if [[ ! -f "$ENV_FILE" ]]; then
    return 0
  fi

  # Minimal dotenv parser:
  # - Preserves spaces in values (e.g. PostgreSQL DSN "host=... user=...").
  # - Ignores blank lines and comments.
  # - Supports optional leading "export ".
  while IFS= read -r line || [[ -n "$line" ]]; do
    # trim leading/trailing whitespace
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "${line}" ]] && continue
    [[ "${line:0:1}" == "#" ]] && continue
    [[ "${line}" == "export "* ]] && line="${line#export }"
    [[ "${line}" != *"="* ]] && continue

    local key value
    key="${line%%=*}"
    value="${line#*=}"

    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"
    [[ -z "${key}" ]] && continue

    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"

    if [[ "${value}" == \"*\" && "${value}" == *\" ]]; then
      value="${value:1:-1}"
    elif [[ "${value}" == \'*\' && "${value}" == *\' ]]; then
      value="${value:1:-1}"
    fi

    export "${key}=${value}"
  done < "$ENV_FILE"
}

init_env() {
  load_env
  export SERVER_HOST="${SERVER_HOST:-0.0.0.0}"
  export SERVER_PORT="${SERVER_PORT:-34007}"
  mkdir -p "$LOG_DIR" "$PID_DIR"
}

is_running() {
  local pid_file="$1"
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && ps -p "$pid" >/dev/null 2>&1; then
      return 0
    fi
  fi
  return 1
}

is_port_listening() {
  local port="$1"
  if [[ -z "${port}" ]]; then
    return 1
  fi

  if command -v ss >/dev/null 2>&1; then
    ss -H -ltn "sport = :${port}" 2>/dev/null | grep -q .
    return $?
  fi

  if command -v netstat >/dev/null 2>&1; then
    netstat -tln 2>/dev/null | awk '{print $4}' | grep -Eq "(:|\\.)${port}\$"
    return $?
  fi

  return 1
}

list_listening_pids() {
  local port="$1"
  if [[ -z "${port}" ]]; then
    return 0
  fi

  if command -v ss >/dev/null 2>&1; then
    ss -H -ltnp "sport = :${port}" 2>/dev/null | grep -o 'pid=[0-9]\+' | cut -d= -f2 | sort -u
    return 0
  fi

  if command -v lsof >/dev/null 2>&1; then
    lsof -n -iTCP:"${port}" -sTCP:LISTEN -t 2>/dev/null | sort -u
    return 0
  fi

  if command -v fuser >/dev/null 2>&1; then
    fuser -n tcp "${port}" 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+$' | sort -u
    return 0
  fi

  return 0
}

takeover_port() {
  local port="$1"
  if [[ -z "${port}" ]]; then
    return 0
  fi

  local pids=()
  while IFS= read -r pid; do
    [[ -n "${pid}" ]] && pids+=("${pid}")
  done < <(list_listening_pids "${port}")

  if [[ ${#pids[@]} -eq 0 ]]; then
    return 0
  fi

  log_warn "Port ${port} is in use. Taking over..."
  for pid in "${pids[@]}"; do
    local cmd
    cmd="$(ps -p "${pid}" -o cmd= 2>/dev/null | sed 's/^[[:space:]]*//')"
    log_warn "Stopping PID ${pid}: ${cmd:-unknown}"
    kill "${pid}" 2>/dev/null || true
  done

  local deadline=$((SECONDS + 5))
  while is_port_listening "${port}" && [[ ${SECONDS} -lt ${deadline} ]]; do
    sleep 0.2
  done

  if is_port_listening "${port}"; then
    log_warn "Port ${port} still in use, forcing stop (SIGKILL)..."
    for pid in "${pids[@]}"; do
      kill -9 "${pid}" 2>/dev/null || true
    done
    sleep 0.5
  fi

  if is_port_listening "${port}"; then
    log_error "Port ${port} is still in use; cannot take over."
    return 1
  fi
  return 0
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
  if [[ "${ip}" == 10.* ]]; then return 0; fi
  if [[ "${ip}" == 192.168.* ]]; then return 0; fi
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
  if [[ -z "${ip}" ]]; then echo "unknown"; return 0; fi
  if [[ "${ip}" == 127.* ]]; then echo "local"; return 0; fi
  if [[ "${ip}" == 169.254.* ]]; then echo "link"; return 0; fi
  if is_private_ipv4 "${ip}"; then echo "lan"; return 0; fi
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

  echo "${label} URLs:"

  if [[ -z "${bind_host}" || "${bind_host}" == "0.0.0.0" || "${bind_host}" == "::" ]]; then
    local ips=()
    while IFS= read -r ip; do
      [[ -n "${ip}" ]] && ips+=("${ip}")
    done < <(list_ipv4_addrs)

    if [[ ${#ips[@]} -gt 0 ]]; then
      for ip in "${ips[@]}"; do
        local scope
        scope="$(ipv4_scope_label "${ip}")"
        echo "  - (${scope}) http://$(format_host_for_url "${ip}"):${port}${path}"
      done
    fi

    echo "  - (local) http://localhost:${port}${path}"
    echo "  - (local) http://127.0.0.1:${port}${path}"
    return 0
  fi

  local show_host
  show_host="$(format_host_for_url "${bind_host}")"
  if [[ -z "${show_host}" ]]; then
    show_host="localhost"
  fi
  echo "  - http://${show_host}:${port}${path}"
}

status() {
  echo "=== AI Coding Assistant (Release) ==="
  echo "Root:  ${ROOT}"
  echo "Bind:  ${SERVER_HOST}:${SERVER_PORT}"
  print_access_urls "App" "${SERVER_HOST}" "${SERVER_PORT}" ""
  echo "Log:   ${BACKEND_LOG}"
  echo ""
  if is_running "${BACKEND_PID_FILE}"; then
    echo -e "Backend: ${GREEN}Running${NC} (PID: $(cat "${BACKEND_PID_FILE}"))"
  else
    echo -e "Backend: ${RED}Stopped${NC}"
  fi
}

start_backend() {
  if [[ ! -x "$BACKEND_BIN" ]]; then
    log_error "Backend binary not found or not executable: $BACKEND_BIN"
    log_error "This release package should contain the binary."
    return 1
  fi

  if is_running "$BACKEND_PID_FILE"; then
    log_warn "Backend already running (PID: $(cat "$BACKEND_PID_FILE"))"
    status
    return 0
  fi

  if is_port_listening "${SERVER_PORT}"; then
    if [[ "${ACA_TAKEOVER_PORTS:-0}" == "1" ]]; then
      takeover_port "${SERVER_PORT}" || return 1
    else
      log_warn "Port ${SERVER_PORT} is already in use."
      if is_interactive && confirm "Take over backend port ${SERVER_PORT} (stop existing process)?"; then
        takeover_port "${SERVER_PORT}" || return 1
      else
        log_error "Port ${SERVER_PORT} is already in use. Use --takeover-ports or change SERVER_PORT."
        return 1
      fi
    fi
  fi

  log_info "Starting backend..."
  cd "$ROOT"
  nohup "$BACKEND_BIN" > "$BACKEND_LOG" 2>&1 &
  echo $! > "$BACKEND_PID_FILE"
  sleep 1

  if is_running "$BACKEND_PID_FILE"; then
    log_info "Backend started (PID: $(cat "$BACKEND_PID_FILE"))"
    status
    return 0
  fi

  log_error "Failed to start backend. Check: $BACKEND_LOG"
  return 1
}

stop_backend() {
  if is_running "$BACKEND_PID_FILE"; then
    local pid
    pid="$(cat "$BACKEND_PID_FILE")"
    log_info "Stopping backend (PID: $pid)..."
    kill "$pid" 2>/dev/null || true
    sleep 1
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$BACKEND_PID_FILE"
    log_info "Backend stopped"
    return 0
  fi
  log_warn "Backend not running"
  return 0
}

run_setup() {
  if [[ ! -x "$BACKEND_BIN" ]]; then
    log_error "Backend binary not found or not executable: $BACKEND_BIN"
    return 1
  fi
  log_info "Starting setup wizard..."
  log_info "It will print a setup URL in the terminal."
  cd "$ROOT"
  exec "$BACKEND_BIN" setup
}

run_guide() {
  local step=0

  step=$((step + 1))
  echo "=== Step ${step}: Overview ==="
  status
  echo ""

  step=$((step + 1))
  echo "=== Step ${step}: Choose action ==="
  echo "  1) Setup wizard (first run)"
  echo "  2) Start backend"
  echo "  3) Stop backend"
  echo "  4) Restart backend"
  echo "  5) Show status"
  echo "  6) View logs"
  echo "  7) Exit"
  echo ""

  local choice=""
  read -r -p "Select [2]: " choice || true
  case "${choice}" in
    1) run_setup ;;
    ""|2) start_backend ;;
    3) stop_backend ;;
    4) stop_backend && sleep 1 && start_backend ;;
    5) status ;;
    6) show_logs ;;
    7) return 0 ;;
    *) log_error "Invalid selection" ; return 1 ;;
  esac

  echo ""
  echo "Done."
  echo "Tips:"
  echo "  - View logs: ./start.sh logs"
  echo "  - Stop:      ./start.sh stop"
  return 0
}

show_logs() {
  if [[ ! -f "$BACKEND_LOG" ]]; then
    log_warn "No log file: $BACKEND_LOG"
    return 0
  fi
  tail -n 120 "$BACKEND_LOG" 2>/dev/null || true
}

run_demo() {
  if [[ ! -f "$DEMO_DB_SOURCE" ]]; then
    log_error "Demo database not found: $DEMO_DB_SOURCE"
    log_error "Please ensure data/demo.db exists in the release directory."
    return 1
  fi

  log_info "Starting in DEMO mode..."

  # Create data directory if not exists
  mkdir -p "$DATA_DIR"

  # Copy demo database to working location
  local demo_db="$DATA_DIR/aca.db"
  log_info "Copying demo database to $demo_db"
  cp -f "$DEMO_DB_SOURCE" "$demo_db"

  # Set demo environment variables
  export DEMO_MODE=true
  export DATABASE_DSN="$demo_db"
  export AUTH_USERNAME=demo
  export AUTH_PASSWORD=demo123

  log_info "Demo credentials: demo / demo123"
  log_info "Demo mode is READ-ONLY"

  # Start backend with demo settings
  start_backend
}

init_env

COMMAND=""
if [[ $# -eq 0 ]]; then
  if is_interactive; then
    COMMAND="guide"
  else
    COMMAND="start"
  fi
else
  COMMAND="$1"
  shift
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --takeover-ports)
      export ACA_TAKEOVER_PORTS=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      log_error "Unknown arg: $1"
      usage
      exit 1
      ;;
  esac
done

case "${COMMAND}" in
  guide) run_guide ;;
  setup) run_setup ;;
  start) start_backend ;;
  stop) stop_backend ;;
  restart) stop_backend && sleep 1 && start_backend ;;
  status) status ;;
  logs) show_logs ;;
  demo) run_demo ;;
  *) usage ; exit 1 ;;
esac
