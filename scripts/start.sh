#!/usr/bin/env bash
# AI Coding Assistant - 启动脚本
# 用法: ./scripts/start.sh [command] [options]

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$PROJECT_ROOT/backend"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

# Runtime files are stored under project root to avoid relying on system dirs like /tmp.
RUNTIME_DIR="$PROJECT_ROOT/.aca"
LOG_DIR="$RUNTIME_DIR/logs"
PID_DIR="$RUNTIME_DIR/pids"

BACKEND_LOG="$LOG_DIR/backend.log"
FRONTEND_LOG="$LOG_DIR/frontend.log"
BACKEND_PID_FILE="$PID_DIR/backend.pid"
FRONTEND_PID_FILE="$PID_DIR/frontend.pid"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
    cat <<'EOF'
Usage: ./scripts/start.sh [command] [options]

Commands:
  guide            Step-by-step interactive guide (default when run in a TTY without args)
  dev              Start backend(go run) + frontend(dev)
  backend          Start backend only
  backend-dev      Start backend with go run (dev)
  frontend         Start frontend only
  all              Start both (default in non-interactive mode)
  restart          Restart both services
  stop             Stop both services
  status           Show service status
  logs [backend|frontend]  Show logs

Options:
  --takeover-ports   Kill any process listening on SERVER_PORT / ACA_FRONTEND_PORT before starting

Environment:
  ACA_TAKEOVER_PORTS=1   Same as --takeover-ports
EOF
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        log_error "Missing required command: $1"
        return 1
    fi
    return 0
}

is_interactive() {
    [[ -t 0 && -t 1 ]]
}

confirm() {
    local prompt_text="$1"
    local answer=""
    read -r -p "${prompt_text} [y/N]: " answer || true
    case "${answer}" in
        y|Y|yes|YES)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
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

display_backend_url() {
    local host="${SERVER_HOST:-}"
    local port="${SERVER_PORT:-}"
    log_info "Backend bind: ${host}:${port}"
    print_access_urls "Backend" "${host}" "${port}" ""
}

display_frontend_url() {
    local port="${ACA_FRONTEND_PORT:-}"
    local host="${ACA_FRONTEND_HOST:-0.0.0.0}"
    log_info "Frontend dev bind: ${host}:${port}"
    print_access_urls "Frontend dev" "${host}" "${port}" "/"
}

load_env() {
    local env_file="$PROJECT_ROOT/.env"
    if [[ -f "$env_file" ]]; then
        set -a
        # shellcheck disable=SC1090
        source "$env_file"
        set +a
        log_info "Loaded .env"
        return 0
    fi
    log_warn "No .env found at $env_file, using defaults"
}

init_dev_env() {
    load_env

    export SERVER_HOST="${SERVER_HOST:-0.0.0.0}"
    export SERVER_PORT="${SERVER_PORT:-34007}"

    # Vite dev server proxy variables (see frontend/vite.config.ts)
    export ACA_BACKEND_HOST="${ACA_BACKEND_HOST:-localhost}"
    export ACA_BACKEND_PORT="${ACA_BACKEND_PORT:-$SERVER_PORT}"
    export ACA_FRONTEND_HOST="${ACA_FRONTEND_HOST:-0.0.0.0}"
    export ACA_FRONTEND_PORT="${ACA_FRONTEND_PORT:-34001}"

    mkdir -p "$LOG_DIR" "$PID_DIR"
}

backend_mode() {
    echo "${ACA_BACKEND_MODE:-binary}"
}

# 检查进程是否运行
is_running() {
    local pid_file=$1
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0
        fi
    fi
    return 1
}

# 启动后端（Go run，开发用）
start_backend_go_run() {
    if ! require_cmd go; then
        log_error "Go not found. Install Go 1.21+ or use ACA_BACKEND_MODE=binary."
        return 1
    fi

    export GOCACHE="${GOCACHE:-$RUNTIME_DIR/go-build-cache}"
    mkdir -p "$GOCACHE"

    log_info "Starting backend (go run)..."
    cd "$BACKEND_DIR"
    nohup go run . > "$BACKEND_LOG" 2>&1 &
    echo $! > "$BACKEND_PID_FILE"
    sleep 2
}

# 启动后端（二进制）
start_backend_binary() {
    if [[ ! -x "$BACKEND_DIR/ai-coding-assistant" ]]; then
        log_error "Backend binary not found: $BACKEND_DIR/ai-coding-assistant"
        log_error "Try: (cd backend && go build -o ai-coding-assistant .)"
        return 1
    fi

    log_info "Starting backend (binary)..."
    cd "$BACKEND_DIR"
    nohup ./ai-coding-assistant > "$BACKEND_LOG" 2>&1 &
    echo $! > "$BACKEND_PID_FILE"
    sleep 2
}

# 启动后端
start_backend() {
    if is_running "$BACKEND_PID_FILE"; then
        log_warn "Backend already running (PID: $(cat $BACKEND_PID_FILE))"
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

    case "$(backend_mode)" in
        go-run|gorun|go)
            start_backend_go_run
            ;;
        binary|"")
            start_backend_binary
            ;;
        *)
            log_error "Unsupported ACA_BACKEND_MODE=$(backend_mode) (supported: binary, go-run)"
            return 1
            ;;
    esac

    if is_running "$BACKEND_PID_FILE"; then
        log_info "Backend started (PID: $(cat $BACKEND_PID_FILE))"
        display_backend_url
    else
        log_error "Failed to start backend. Check $BACKEND_LOG"
        return 1
    fi
}

# 启动前端
start_frontend() {
    if is_running "$FRONTEND_PID_FILE"; then
        log_warn "Frontend already running (PID: $(cat $FRONTEND_PID_FILE))"
        return 0
    fi

    require_cmd node || return 1
    require_cmd npm || return 1

    if is_port_listening "${ACA_FRONTEND_PORT}"; then
        if [[ "${ACA_TAKEOVER_PORTS:-0}" == "1" ]]; then
            takeover_port "${ACA_FRONTEND_PORT}" || return 1
        else
            log_warn "Port ${ACA_FRONTEND_PORT} is already in use."
            if is_interactive && confirm "Take over frontend port ${ACA_FRONTEND_PORT} (stop existing process)?"; then
                takeover_port "${ACA_FRONTEND_PORT}" || return 1
            else
                log_error "Port ${ACA_FRONTEND_PORT} is already in use. Use --takeover-ports or change ACA_FRONTEND_PORT."
                return 1
            fi
        fi
    fi

    log_info "Starting frontend..."
    cd "$FRONTEND_DIR"
    nohup npm run dev -- --host "$ACA_FRONTEND_HOST" --port "$ACA_FRONTEND_PORT" --strictPort > "$FRONTEND_LOG" 2>&1 &
    echo $! > "$FRONTEND_PID_FILE"
    sleep 3

    if is_running "$FRONTEND_PID_FILE"; then
        log_info "Frontend started (PID: $(cat $FRONTEND_PID_FILE))"
        display_frontend_url
    else
        log_error "Failed to start frontend. Check $FRONTEND_LOG"
        return 1
    fi
}

# 停止后端
stop_backend() {
    if is_running "$BACKEND_PID_FILE"; then
        local pid=$(cat "$BACKEND_PID_FILE")
        log_info "Stopping backend (PID: $pid)..."
        kill "$pid" 2>/dev/null || true
        sleep 1
        kill -9 "$pid" 2>/dev/null || true
        rm -f "$BACKEND_PID_FILE"
        log_info "Backend stopped"
    else
        log_warn "Backend not running"
        # 清理可能残留的进程
        pkill -f "$BACKEND_DIR/ai-coding-assistant" 2>/dev/null || true
        pkill -f "go run .*${BACKEND_DIR}" 2>/dev/null || true
    fi
}

# 停止前端
stop_frontend() {
    if is_running "$FRONTEND_PID_FILE"; then
        local pid=$(cat "$FRONTEND_PID_FILE")
        log_info "Stopping frontend (PID: $pid)..."
        kill "$pid" 2>/dev/null || true
        sleep 1
        kill -9 "$pid" 2>/dev/null || true
        rm -f "$FRONTEND_PID_FILE"
        log_info "Frontend stopped"
    else
        log_warn "Frontend not running"
        # 清理可能残留的进程
        pkill -f "npm run dev" 2>/dev/null || true
    fi
}

# 显示状态
show_status() {
    echo "=== AI Coding Assistant Status ==="

    if is_running "$BACKEND_PID_FILE"; then
        echo -e "Backend:  ${GREEN}Running${NC} (PID: $(cat $BACKEND_PID_FILE))"
    else
        echo -e "Backend:  ${RED}Stopped${NC}"
    fi

    if is_running "$FRONTEND_PID_FILE"; then
        echo -e "Frontend: ${GREEN}Running${NC} (PID: $(cat $FRONTEND_PID_FILE))"
    else
        echo -e "Frontend: ${RED}Stopped${NC}"
    fi

    # 检查端口
    echo ""
    echo "=== Port Status ==="
    if command -v ss >/dev/null 2>&1; then
        ss -tlnp 2>/dev/null | grep -E "(:${ACA_FRONTEND_PORT}|:${SERVER_PORT})" || echo "No services listening on expected ports"
    else
        netstat -tlnp 2>/dev/null | grep -E "(:${ACA_FRONTEND_PORT}|:${SERVER_PORT})" || echo "No services listening on expected ports"
    fi
}

# 显示日志
show_logs() {
    local service=$1
    case $service in
        backend)
            tail -50 "$BACKEND_LOG" 2>/dev/null || echo "No backend logs: $BACKEND_LOG"
            ;;
        frontend)
            tail -50 "$FRONTEND_LOG" 2>/dev/null || echo "No frontend logs: $FRONTEND_LOG"
            ;;
        *)
            echo "=== Backend Logs (last 20 lines) ==="
            tail -20 "$BACKEND_LOG" 2>/dev/null || echo "No backend logs: $BACKEND_LOG"
            echo ""
            echo "=== Frontend Logs (last 20 lines) ==="
            tail -20 "$FRONTEND_LOG" 2>/dev/null || echo "No frontend logs: $FRONTEND_LOG"
            ;;
    esac
}

run_guide() {
    local step=0

    step=$((step + 1))
    echo "=== Step ${step}: Overview ==="
    echo "Project: ${PROJECT_ROOT}"
    echo "Backend mode: $(backend_mode)"
    display_backend_url
    echo ""
    display_frontend_url
    echo "Logs: ${LOG_DIR}"
    echo ""

    step=$((step + 1))
    echo "=== Step ${step}: Choose action ==="
    echo "  1) Start backend + frontend (recommended)"
    echo "  2) Start backend only"
    echo "  3) Start frontend only"
    echo "  4) Dev mode (backend go run + frontend dev)"
    echo "  5) Show status"
    echo "  6) Show logs"
    echo "  7) Stop services"
    echo ""

    local choice=""
    read -r -p "Select [1]: " choice || true
    case "${choice}" in
        ""|1)
            start_backend
            start_frontend
            show_status
            ;;
        2)
            start_backend
            show_status
            ;;
        3)
            start_frontend
            show_status
            ;;
        4)
            ACA_BACKEND_MODE="go-run" start_backend
            start_frontend
            show_status
            ;;
        5)
            show_status
            ;;
        6)
            echo ""
            echo "Which logs?"
            echo "  1) all"
            echo "  2) backend"
            echo "  3) frontend"
            local which=""
            read -r -p "Select [1]: " which || true
            case "${which}" in
                2) show_logs backend ;;
                3) show_logs frontend ;;
                *) show_logs all ;;
            esac
            ;;
        7)
            stop_backend
            stop_frontend
            ;;
        *)
            log_error "Invalid selection"
            return 1
            ;;
    esac

    echo ""
    echo "Done."
    echo "Tips:"
    echo "  - View logs: ./scripts/start.sh logs"
    echo "  - Stop:      ./scripts/start.sh stop"
    return 0
}

# 初始化开发环境变量（只加载一次，避免重复输出）
init_dev_env

# Parse args
COMMAND=""
if [[ $# -eq 0 ]]; then
    if is_interactive; then
        COMMAND="guide"
    else
        COMMAND="all"
    fi
else
    COMMAND="${1}"
    shift
fi

LOG_TARGET="all"
if [[ "${COMMAND}" == "logs" && $# -gt 0 && "${1}" != --* ]]; then
    LOG_TARGET="${1}"
    shift
fi

while [[ $# -gt 0 ]]; do
    case "${1}" in
        --takeover-ports)
            export ACA_TAKEOVER_PORTS=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown arg: ${1}"
            usage
            exit 1
            ;;
    esac
done

# 主命令处理
case "${COMMAND}" in
    guide)
        run_guide
        ;;
    dev)
        ACA_BACKEND_MODE="go-run" start_backend
        start_frontend
        show_status
        ;;
    backend)
        start_backend
        ;;
    backend-dev)
        ACA_BACKEND_MODE="go-run" start_backend
        ;;
    frontend)
        start_frontend
        ;;
    all)
        start_backend
        start_frontend
        show_status
        ;;
    restart)
        stop_backend
        stop_frontend
        sleep 2
        start_backend
        start_frontend
        show_status
        ;;
    stop)
        stop_backend
        stop_frontend
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs "${LOG_TARGET}"
        ;;
    *)
        usage
        exit 1
        ;;
esac
