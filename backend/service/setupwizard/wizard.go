package setupwizard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type RunOptions struct {
	ProjectRoot string
	BindHost    string
	Port        int
}

type SetupConfig struct {
	BackendMode   string `json:"backend_mode"`   // "go-run" | "binary"
	FrontendMode  string `json:"frontend_mode"`  // "dev" | "embedded"
	InstallDeps   bool   `json:"install_deps"`   // npm ci
	BuildFrontend bool   `json:"build_frontend"` // npm run build (embedded mode)

	ServerHost string `json:"server_host"`
	ServerPort int    `json:"server_port"`

	FrontendPort int `json:"frontend_port"`

	DatabaseType string `json:"database_type"` // sqlite | postgres
	DatabaseDSN  string `json:"database_dsn"`  // sqlite: ./data/aca.db (relative to backend/), postgres: DSN string

	AdminUsername string `json:"admin_username"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
	JWTSecret     string `json:"jwt_secret"`
	DemoMode      bool   `json:"demo_mode"`

	TerminalDefaultLoginDir string `json:"terminal_default_login_dir"`

	SystemRule SystemRuleConfig `json:"system_rule"`
}

type SystemRuleConfig struct {
	Name          string   `json:"name"`
	ApprovalMode  string   `json:"approval_mode"`   // manual | auto_yes | smart
	AutoInputType string   `json:"auto_input_type"` // yes | y | enter | option1
	ContextLines  int      `json:"context_lines"`
	Whitelist     []string `json:"whitelist_patterns"`
	Blacklist     []string `json:"blacklist_patterns"`
	NotifyOnBlock bool     `json:"notify_on_block"`
	NotifyOnOK    bool     `json:"notify_on_approve"`
	DetectClaude  bool     `json:"detect_claude_code"`
	DetectCodex   bool     `json:"detect_codex"`
	DetectGemini  bool     `json:"detect_gemini"`
}

type SetupStatus struct {
	Started   bool   `json:"started"`
	Finished  bool   `json:"finished"`
	Phase     string `json:"phase"`
	Error     string `json:"error,omitempty"`
	Backend   string `json:"backend_url,omitempty"`
	Frontend  string `json:"frontend_url,omitempty"`
	EnvPath   string `json:"env_path,omitempty"`
	LogTail   []Log  `json:"log_tail,omitempty"`
	LogsTotal int    `json:"logs_total"`
}

type Log struct {
	Time  string `json:"time"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

type Wizard struct {
	token string

	preflight *Preflight

	mu        sync.Mutex
	status    SetupStatus
	logs      []Log
	running   bool
	subs      map[chan Log]struct{}
	shutdown  func()
	project   string
	bindHost  string
	listenURL string
}

func Run(opts RunOptions) error {
	bindHost := strings.TrimSpace(opts.BindHost)
	if bindHost == "" {
		bindHost = "0.0.0.0"
	}

	port := opts.Port
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	pre, err := CollectPreflight(opts.ProjectRoot)
	if err != nil {
		return err
	}

	token, err := randomToken(16)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindHost, port))
	if err != nil {
		if port != 0 {
			log.Printf("Port %d is in use, switching to a random available port...\n", port)
			ln, err = net.Listen("tcp", fmt.Sprintf("%s:%d", bindHost, 0))
		}
		if err != nil {
			return err
		}
	}

	actualPort := 0
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		actualPort = tcpAddr.Port
	}
	if actualPort == 0 {
		_ = ln.Close()
		return errors.New("failed to determine setup server port")
	}

	w := &Wizard{
		token:     token,
		preflight: pre,
		status: SetupStatus{
			Started:   false,
			Finished:  false,
			Phase:     "waiting",
			EnvPath:   pre.EnvPath,
			LogTail:   nil,
			LogsTotal: 0,
		},
		subs:     make(map[chan Log]struct{}),
		project:  pre.ProjectRoot,
		bindHost: bindHost,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", w.withToken(w.handleIndex))
	mux.HandleFunc("/api/preflight", w.withToken(w.handlePreflight))
	mux.HandleFunc("/api/status", w.withToken(w.handleStatus))
	mux.HandleFunc("/api/setup", w.withToken(w.handleSetup))
	mux.HandleFunc("/events", w.withToken(w.handleEvents))
	mux.HandleFunc("/api/quit", w.withToken(w.handleQuit))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	w.shutdown = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}

	urlHost := bindHost
	if urlHost == "0.0.0.0" || urlHost == "::" {
		urlHost = "localhost"
	}

	w.listenURL = fmt.Sprintf("http://%s/?token=%s", net.JoinHostPort(urlHost, fmt.Sprint(actualPort)), token)
	log.Printf("Setup wizard is running at: %s\n", w.listenURL)
	if bindHost == "0.0.0.0" || bindHost == "::" {
		log.Printf("If you are accessing from another machine, replace 'localhost' with your server IP.\n")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received %s, shutting down setup wizard...\n", sig.String())
		w.shutdown()
		<-errCh
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func randomToken(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		bytesLen = 16
	}
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (w *Wizard) withToken(next http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if w == nil {
			http.Error(rw, "wizard not initialized", http.StatusInternalServerError)
			return
		}

		if w.checkToken(rw, r) {
			next(rw, r)
			return
		}

		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte("Unauthorized. Use the setup URL printed in the terminal.\n"))
	}
}

func (w *Wizard) checkToken(rw http.ResponseWriter, r *http.Request) bool {
	if w == nil {
		return false
	}
	if c, err := r.Cookie("aca_setup_token"); err == nil && c != nil && c.Value == w.token {
		return true
	}

	if q := strings.TrimSpace(r.URL.Query().Get("token")); q != "" && q == w.token {
		http.SetCookie(rw, &http.Cookie{
			Name:     "aca_setup_token",
			Value:    w.token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   3600,
		})
		return true
	}
	return false
}

func (w *Wizard) handleIndex(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write([]byte(indexHTML))
}

func (w *Wizard) handlePreflight(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(rw, w.preflight)
}

func (w *Wizard) handleStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	const tailN = 200
	tail := w.logs
	if len(tail) > tailN {
		tail = tail[len(tail)-tailN:]
	}

	out := w.status
	out.LogTail = append([]Log(nil), tail...)
	out.LogsTotal = len(w.logs)
	writeJSON(rw, out)
}

func (w *Wizard) handleSetup(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(rw, "failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var cfg SetupConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		http.Error(rw, "setup is already running", http.StatusConflict)
		return
	}
	w.running = true
	w.status.Started = true
	w.status.Finished = false
	w.status.Error = ""
	w.status.Phase = "starting"
	w.mu.Unlock()

	w.logf("info", "Starting setup…")
	go w.runSetup(cfg)

	rw.WriteHeader(http.StatusAccepted)
	_, _ = rw.Write([]byte("ok"))
}

func (w *Wizard) handleEvents(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")

	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan Log, 256)

	const tailN = 200
	var initial []Log
	w.mu.Lock()
	initial = w.logs
	if len(initial) > tailN {
		initial = initial[len(initial)-tailN:]
	}
	initial = append([]Log(nil), initial...)
	w.subs[ch] = struct{}{}
	w.mu.Unlock()

	for _, entry := range initial {
		payload, _ := json.Marshal(entry)
		_, _ = fmt.Fprintf(rw, "data: %s\n\n", payload)
	}
	flusher.Flush()

	defer func() {
		w.mu.Lock()
		delete(w.subs, ch)
		w.mu.Unlock()
		close(ch)
	}()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			payload, _ := json.Marshal(entry)
			_, _ = fmt.Fprintf(rw, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

func (w *Wizard) handleQuit(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.logf("info", "Stopping setup wizard…")
	if w.shutdown != nil {
		go w.shutdown()
	}
	writeJSON(rw, map[string]any{"ok": true})
}

func (w *Wizard) logf(level string, msg string) {
	if w == nil {
		return
	}

	entry := Log{
		Time:  time.Now().Format("15:04:05"),
		Level: strings.TrimSpace(level),
		Text:  strings.TrimSpace(msg),
	}

	w.mu.Lock()
	w.logs = append(w.logs, entry)
	for ch := range w.subs {
		select {
		case ch <- entry:
		default:
		}
	}
	w.mu.Unlock()
}

func (w *Wizard) setError(err error) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.status.Error = strings.TrimSpace(err.Error())
	w.status.Phase = "error"
	w.status.Finished = true
	w.running = false
	w.mu.Unlock()
}

func (w *Wizard) setDone(backendURL, frontendURL string) {
	w.mu.Lock()
	w.status.Backend = strings.TrimSpace(backendURL)
	w.status.Frontend = strings.TrimSpace(frontendURL)
	w.status.Phase = "done"
	w.status.Finished = true
	w.running = false
	w.mu.Unlock()
}

func (w *Wizard) setPhase(phase string) {
	w.mu.Lock()
	w.status.Phase = strings.TrimSpace(phase)
	w.mu.Unlock()
}

func writeJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(rw)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (w *Wizard) runSetup(cfg SetupConfig) {
	backendURL, frontendURL, err := PerformSetup(context.Background(), w.preflight, cfg, w.logf, w.setPhase)
	if err != nil {
		w.logf("error", err.Error())
		w.setError(err)
		return
	}
	w.logf("success", "Setup completed.")
	w.setDone(backendURL, frontendURL)
}

const indexHTML = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>AI Coding Assistant · 初始化向导</title>
    <style>
      :root { color-scheme: dark; }
      body { margin: 0; font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, "Helvetica Neue", Arial; background: #141414; color: #e8e8e8; }
      a { color: #7dd3fc; }
      .wrap { max-width: 980px; margin: 0 auto; padding: 16px; }
      .card { background: #1f1f1f; border: 1px solid #2a2a2a; border-radius: 12px; padding: 14px; }
      .row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
      @media (max-width: 860px) { .row { grid-template-columns: 1fr; } }
      h1 { font-size: 18px; margin: 0 0 10px; }
      h2 { font-size: 14px; margin: 16px 0 8px; color: #cfcfcf; }
      .muted { color: #9aa0a6; font-size: 12px; }
      label { display: block; font-size: 12px; color: #cfcfcf; margin-bottom: 4px; }
      input, select, textarea { width: 100%; box-sizing: border-box; background: #101010; border: 1px solid #333; border-radius: 8px; padding: 8px 10px; color: #e8e8e8; }
      textarea { min-height: 72px; }
      .inline { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
      .btn { display: inline-flex; align-items: center; justify-content: center; gap: 8px; border: 0; border-radius: 10px; padding: 10px 12px; background: #2563eb; color: #fff; cursor: pointer; }
      .btn:disabled { opacity: 0.55; cursor: not-allowed; }
      .btn.secondary { background: #374151; }
      .pill { display: inline-flex; align-items: center; gap: 6px; padding: 4px 8px; border: 1px solid #333; border-radius: 999px; font-size: 12px; color: #cfcfcf; }
      .ok { color: #86efac; }
      .bad { color: #fca5a5; }
      .log { background: #0b0b0b; border: 1px solid #2a2a2a; border-radius: 10px; padding: 10px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 12px; height: 280px; overflow: auto; white-space: pre-wrap; }
      .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
      @media (max-width: 680px) { .grid2 { grid-template-columns: 1fr; } }
      .hr { height: 1px; background: #2a2a2a; margin: 14px 0; }
    </style>
  </head>
  <body>
    <div class="wrap">
      <div class="card">
        <h1>AI Coding Assistant · 初始化向导</h1>
        <div class="muted">首次启动会写入 <code>.env</code>，初始化 SQLite / PostgreSQL，设置默认审核规则与管理员账号，并启动后端/前端。</div>

        <h2>环境检测</h2>
        <div id="preflight" class="inline muted">加载中…</div>

        <div class="hr"></div>

        <h2>运行方式</h2>
        <div class="grid2">
          <div>
            <label>后端运行方式</label>
            <select id="backend_mode">
              <option value="binary">运行编译的 Go 二进制（推荐）</option>
              <option value="go-run">Go 开发模式（go run .）</option>
            </select>
            <div class="muted">选择 go-run 需要安装 Go；binary 模式会优先使用 <code>backend/ai-coding-assistant</code>。</div>
          </div>
          <div>
            <label>前端运行方式</label>
            <select id="frontend_mode">
              <option value="embedded">内置静态资源（无需 Node）</option>
              <option value="dev">Vite 开发模式（需要 Node）</option>
            </select>
            <div class="muted">embedded 模式默认直接由后端提供静态资源；dev 模式会启动 Vite 并代理 /api。</div>
          </div>
        </div>

        <h2>端口与数据库</h2>
        <div class="row">
          <div>
            <label>后端监听地址</label>
            <input id="server_host" placeholder="0.0.0.0" />
          </div>
          <div>
            <label>后端端口</label>
            <input id="server_port" type="number" min="1" max="65535" />
          </div>
          <div>
            <label>前端 Dev 端口（仅 dev 模式使用）</label>
            <input id="frontend_port" type="number" min="1" max="65535" />
          </div>
          <div>
            <label>数据库类型</label>
            <select id="database_type">
              <option value="sqlite">SQLite（本地文件）</option>
              <option value="postgres">PostgreSQL</option>
            </select>
          </div>
          <div>
            <label>数据库 DSN</label>
            <input id="database_dsn" placeholder="./data/aca.db" />
          </div>
        </div>

        <h2>管理员账号</h2>
        <div class="row">
          <div>
            <label>用户名</label>
            <input id="admin_username" placeholder="admin" />
          </div>
          <div>
            <label>Email（可选）</label>
            <input id="admin_email" placeholder="admin@example.com" />
          </div>
          <div>
            <label>密码</label>
            <input id="admin_password" type="password" placeholder="admin123" />
          </div>
          <div>
            <label>JWT_SECRET（留空自动生成）</label>
            <input id="jwt_secret" placeholder="auto" />
          </div>
        </div>

        <h2>默认审核规则（系统规则）</h2>
        <div class="row">
          <div>
            <label>审批模式</label>
            <select id="rule_approval_mode">
              <option value="manual">manual（手动）</option>
              <option value="auto_yes">auto_yes（全自动）</option>
              <option value="smart">smart（AI 辅助）</option>
            </select>
          </div>
          <div>
            <label>自动输入类型（auto_yes）</label>
            <select id="rule_auto_input_type">
              <option value="yes">yes</option>
              <option value="y">y</option>
              <option value="enter">enter</option>
              <option value="option1">option1</option>
            </select>
          </div>
          <div>
            <label>上下文行数</label>
            <input id="rule_context_lines" type="number" min="1" max="500" />
          </div>
          <div class="inline" style="align-items:flex-end">
            <label style="width:100%">检测</label>
            <span class="pill"><input id="rule_detect_claude" type="checkbox" checked />Claude</span>
            <span class="pill"><input id="rule_detect_codex" type="checkbox" checked />Codex</span>
            <span class="pill"><input id="rule_detect_gemini" type="checkbox" checked />Gemini</span>
          </div>
          <div class="inline" style="align-items:flex-end">
            <label style="width:100%">通知</label>
            <span class="pill"><input id="rule_notify_block" type="checkbox" checked />阻止时通知</span>
            <span class="pill"><input id="rule_notify_ok" type="checkbox" />允许时通知</span>
          </div>
          <div>
            <label>白名单（每行一条，正则/关键字）</label>
            <textarea id="rule_whitelist" placeholder=""></textarea>
          </div>
          <div>
            <label>黑名单（每行一条，正则/关键字）</label>
            <textarea id="rule_blacklist" placeholder=""></textarea>
          </div>
        </div>

        <h2>其他设置</h2>
        <div class="row">
          <div>
            <label>终端默认登入目录</label>
            <input id="terminal_default_login_dir" placeholder="~/" />
          </div>
          <div class="inline" style="align-items:flex-end">
            <label style="width:100%">选项</label>
            <span class="pill"><input id="demo_mode" type="checkbox" />演示模式（只读）</span>
            <span class="pill"><input id="install_deps" type="checkbox" checked />自动安装前端依赖（npm ci）</span>
            <span class="pill"><input id="build_frontend" type="checkbox" />重新构建前端（npm run build）</span>
          </div>
        </div>

        <div class="hr"></div>

        <div class="inline">
          <button id="start" class="btn">开始初始化并启动</button>
          <button id="quit" class="btn secondary">关闭向导</button>
          <span id="phase" class="pill">phase: waiting</span>
        </div>

        <h2>输出日志</h2>
        <div id="log" class="log"></div>

        <div id="result" style="margin-top: 12px;"></div>
      </div>
    </div>

    <script>
      const el = (id) => document.getElementById(id);
      const logEl = el('log');
      const phaseEl = el('phase');
      const resultEl = el('result');
      const startBtn = el('start');
      const quitBtn = el('quit');

      const writeLog = (line) => {
        logEl.textContent += line + "\\n";
        logEl.scrollTop = logEl.scrollHeight;
      };

      const setPhase = (p) => { phaseEl.textContent = 'phase: ' + p; };

      async function preflight() {
	        const res = await fetch('/api/preflight');
	        const data = await res.json();
	        const items = [];
	        items.push('<span class="pill">' + (data.platform || '') + '</span>');
	        items.push('<span class="pill">' + (data.has_go ? '<span class="ok">Go ✓</span>' : '<span class="bad">Go ✗</span>') + '</span>');
	        items.push('<span class="pill">' + (data.has_node ? '<span class="ok">Node ✓</span>' : '<span class="bad">Node ✗</span>') + '</span>');
	        items.push('<span class="pill">' + (data.has_npm ? '<span class="ok">npm ✓</span>' : '<span class="bad">npm ✗</span>') + '</span>');
	        items.push('<span class="pill">' + (data.has_backend_binary ? '<span class="ok">backend binary ✓</span>' : '<span class="bad">backend binary ✗</span>') + '</span>');
	        items.push('<span class="pill">' + (data.has_embedded_frontend ? '<span class="ok">embedded frontend ✓</span>' : '<span class="bad">embedded frontend ✗</span>') + '</span>');
	        items.push('<span class="pill">.env: ' + (data.has_env_file ? '<span class="ok">exists</span>' : '<span class="bad">missing</span>') + '</span>');
	        el('preflight').innerHTML = items.join(' ');

        const env = data.env_values || {};
        el('server_host').value = env.SERVER_HOST || data.default_host || '0.0.0.0';
        el('server_port').value = Number(env.SERVER_PORT || data.recommended_port || 34007);
        el('frontend_port').value = Number(env.ACA_FRONTEND_PORT || data.recommended_frontend_port || 34001);
        const dbType = String(env.DATABASE_TYPE || 'sqlite').toLowerCase();
        el('database_type').value = (dbType === 'postgresql' ? 'postgres' : dbType);
        el('database_dsn').value = env.DATABASE_DSN || './data/aca.db';
        el('admin_username').value = env.AUTH_USERNAME || 'admin';
        el('admin_password').value = env.AUTH_PASSWORD || 'admin123';
        el('jwt_secret').value = env.JWT_SECRET || '';
        el('terminal_default_login_dir').value = env.TERMINAL_DEFAULT_LOGIN_DIR || '~/';
        el('demo_mode').checked = (String(env.DEMO_MODE || 'false').toLowerCase() === 'true');

        refreshDatabasePlaceholder();
      }

      function refreshDatabasePlaceholder() {
        const t = (el('database_type').value || 'sqlite').toLowerCase();
        if (t === 'postgres') {
          el('database_dsn').placeholder = 'host=localhost user=aca password=secret dbname=aca port=5432 sslmode=disable';
          return;
        }
        el('database_dsn').placeholder = './data/aca.db';
      }

      function linesToArray(text) {
        return (text || '').split(/\\r?\\n/).map(s => s.trim()).filter(Boolean);
      }

      async function start() {
        resultEl.innerHTML = '';
        logEl.textContent = '';
        startBtn.disabled = true;

        const cfg = {
          backend_mode: el('backend_mode').value,
          frontend_mode: el('frontend_mode').value,
          install_deps: el('install_deps').checked,
          build_frontend: el('build_frontend').checked,
          server_host: el('server_host').value || '0.0.0.0',
          server_port: Number(el('server_port').value || 34007),
          frontend_port: Number(el('frontend_port').value || 34001),
          database_type: el('database_type').value || 'sqlite',
          database_dsn: el('database_dsn').value || './data/aca.db',
          admin_username: el('admin_username').value || 'admin',
          admin_email: el('admin_email').value || '',
          admin_password: el('admin_password').value || '',
          jwt_secret: el('jwt_secret').value || '',
          demo_mode: el('demo_mode').checked,
          terminal_default_login_dir: el('terminal_default_login_dir').value || '~/',
          system_rule: {
            name: '系统默认规则',
            approval_mode: el('rule_approval_mode').value,
            auto_input_type: el('rule_auto_input_type').value,
            context_lines: Number(el('rule_context_lines').value || 50),
            whitelist_patterns: linesToArray(el('rule_whitelist').value),
            blacklist_patterns: linesToArray(el('rule_blacklist').value),
            notify_on_block: el('rule_notify_block').checked,
            notify_on_approve: el('rule_notify_ok').checked,
            detect_claude_code: el('rule_detect_claude').checked,
            detect_codex: el('rule_detect_codex').checked,
            detect_gemini: el('rule_detect_gemini').checked
          }
        };

        const res = await fetch('/api/setup', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cfg) });
        if (!res.ok) {
          writeLog('[error] ' + await res.text());
          startBtn.disabled = false;
          return;
        }
      }

      async function pollStatus() {
        try {
          const res = await fetch('/api/status');
          const st = await res.json();
          setPhase(st.phase || 'unknown');
          if (st.finished) {
            startBtn.disabled = false;
            if (st.error) {
              resultEl.innerHTML = '<div class="pill"><span class="bad">失败</span> ' + (st.error || '') + '</div>';
            } else {
              const b = st.backend_url || '';
              const f = st.frontend_url || '';
              resultEl.innerHTML = '<div class="card" style="margin-top:10px">' +
                '<div><b>后端地址：</b> <a href="' + b + '" target="_blank" rel="noreferrer">' + b + '</a></div>' +
                '<div><b>前端地址：</b> <a href="' + f + '" target="_blank" rel="noreferrer">' + f + '</a></div>' +
                '<div class="muted" style="margin-top:6px">请使用管理员账号登录（用户名在上方设置中）。</div>' +
              '</div>';
            }
          }
        } catch {}
      }

      function connectEvents() {
        const es = new EventSource('/events');
        es.onmessage = (e) => {
          try {
            const obj = JSON.parse(e.data);
            const prefix = '[' + (obj.time || '') + ' ' + (obj.level || '') + '] ';
            writeLog(prefix + (obj.text || ''));
          } catch {}
        };
      }

      startBtn.addEventListener('click', start);
      el('database_type').addEventListener('change', refreshDatabasePlaceholder);
      quitBtn.addEventListener('click', async () => {
        await fetch('/api/quit', { method: 'POST' });
      });

      (async () => {
        await preflight();
        connectEvents();
        setInterval(pollStatus, 1000);
      })();
    </script>
  </body>
</html>
`
