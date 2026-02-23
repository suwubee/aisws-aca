package api

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/buildmeta"
	"github.com/gofiber/fiber/v2"
)

type RuntimeVersionOptions struct {
	AppName    string
	ServerHost string
	ServerPort string
	BinaryPath string
	PID        int
	StartedAt  time.Time
}

type RuntimeVersionInfo struct {
	AppName           string    `json:"app_name"`
	Version           string    `json:"version"`
	GitBranch         string    `json:"git_branch"`
	GitCommit         string    `json:"git_commit"`
	GitDirty          bool      `json:"git_dirty"`
	BuildTime         string    `json:"build_time"`
	GoVersion         string    `json:"go_version"`
	BinaryPath        string    `json:"binary_path"`
	PID               int       `json:"pid"`
	StartedAt         time.Time `json:"started_at"`
	ServerHost        string    `json:"server_host"`
	ServerPort        string    `json:"server_port"`
	ServerAddr        string    `json:"server_addr"`
	StaticSource      string    `json:"static_source"`
	StaticIndexAssets []string  `json:"static_index_assets"`
}

type RuntimeVersionController struct {
	mu   sync.RWMutex
	info RuntimeVersionInfo
}

func NewRuntimeVersionController(opts RuntimeVersionOptions) *RuntimeVersionController {
	meta := buildmeta.Resolve()
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	host := strings.TrimSpace(opts.ServerHost)
	port := strings.TrimSpace(opts.ServerPort)
	if host == "" {
		host = "0.0.0.0"
	}
	info := RuntimeVersionInfo{
		AppName:    strings.TrimSpace(opts.AppName),
		Version:    meta.Version,
		GitBranch:  meta.GitBranch,
		GitCommit:  meta.GitCommit,
		GitDirty:   meta.GitDirty,
		BuildTime:  meta.BuildTime,
		GoVersion:  meta.GoVersion,
		BinaryPath: strings.TrimSpace(opts.BinaryPath),
		PID:        opts.PID,
		StartedAt:  startedAt,
		ServerHost: host,
		ServerPort: port,
		ServerAddr: buildServerAddr(host, port),
	}
	if info.AppName == "" {
		info.AppName = "ACA"
	}
	return &RuntimeVersionController{info: info}
}

func (ctrl *RuntimeVersionController) SetStaticDetails(source string, assets []string) {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	ctrl.info.StaticSource = strings.TrimSpace(source)
	ctrl.info.StaticIndexAssets = normalizeAssets(assets)
}

func (ctrl *RuntimeVersionController) Health(c *fiber.Ctx) error {
	info := ctrl.snapshot()
	return c.JSON(fiber.Map{
		"status":        "ok",
		"version":       info.Version,
		"git_branch":    info.GitBranch,
		"git_commit":    info.GitCommit,
		"build_time":    info.BuildTime,
		"started_at":    info.StartedAt,
		"static_source": info.StaticSource,
	})
}

func (ctrl *RuntimeVersionController) GetVersion(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"item": ctrl.snapshot(),
	})
}

func (ctrl *RuntimeVersionController) snapshot() RuntimeVersionInfo {
	ctrl.mu.RLock()
	defer ctrl.mu.RUnlock()

	cp := ctrl.info
	if len(ctrl.info.StaticIndexAssets) > 0 {
		cp.StaticIndexAssets = append([]string(nil), ctrl.info.StaticIndexAssets...)
	}
	return cp
}

func buildServerAddr(host, port string) string {
	if port == "" {
		return host
	}
	if host == "" {
		return ":" + port
	}
	return fmt.Sprintf("%s:%s", host, port)
}

func normalizeAssets(assets []string) []string {
	if len(assets) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(assets))
	out := make([]string, 0, len(assets))
	for _, raw := range assets {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if _, exists := set[item]; exists {
			continue
		}
		set[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}
