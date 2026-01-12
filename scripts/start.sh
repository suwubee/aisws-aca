#!/usr/bin/env bash
# AI Coding Assistant - 启动脚本
# 用法: ./scripts/start.sh {dev|backend|backend-dev|frontend|all|restart|stop|status|logs}

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

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        log_error "Missing required command: $1"
        return 1
    fi
    return 0
}

display_backend_url() {
    local host="${SERVER_HOST:-}"
    local port="${SERVER_PORT:-}"
    local show_host="$host"

    if [[ -z "$show_host" || "$show_host" == "0.0.0.0" || "$show_host" == "::" ]]; then
        show_host="localhost"
    fi

    log_info "Backend bind: ${host}:${port}"
    log_info "Backend open: http://${show_host}:${port}"
    if [[ "$show_host" == "localhost" ]]; then
        log_info "If you are accessing from another machine, replace 'localhost' with your server IP."
    fi
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

    log_info "Starting frontend..."
    cd "$FRONTEND_DIR"
    nohup npm run dev -- --host 0.0.0.0 --port "$ACA_FRONTEND_PORT" --strictPort > "$FRONTEND_LOG" 2>&1 &
    echo $! > "$FRONTEND_PID_FILE"
    sleep 3

    if is_running "$FRONTEND_PID_FILE"; then
        log_info "Frontend started (PID: $(cat $FRONTEND_PID_FILE))"
        log_info "Frontend URL: http://localhost:${ACA_FRONTEND_PORT}/"
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

# 初始化开发环境变量（只加载一次，避免重复输出）
init_dev_env

# 主命令处理
case "${1:-all}" in
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
        show_logs "${2:-all}"
        ;;
    *)
        echo "Usage: $0 {dev|backend|backend-dev|frontend|all|restart|stop|status|logs [backend|frontend]}"
        echo ""
        echo "Commands:"
        echo "  dev       - Start backend(go run) + frontend(dev)"
        echo "  backend   - Start backend only"
        echo "  backend-dev - Start backend with go run (dev)"
        echo "  frontend  - Start frontend only"
        echo "  all       - Start both (default)"
        echo "  restart   - Restart both services"
        echo "  stop      - Stop both services"
        echo "  status    - Show service status"
        echo "  logs      - Show logs (optionally specify backend/frontend)"
        exit 1
        ;;
esac
