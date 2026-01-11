package setupwizard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Preflight struct {
	ProjectRoot string `json:"project_root"`
	BackendDir  string `json:"backend_dir"`
	FrontendDir string `json:"frontend_dir"`
	RuntimeDir  string `json:"runtime_dir"`
	EnvPath     string `json:"env_path"`

	HasEnvFile bool              `json:"has_env_file"`
	EnvValues  map[string]string `json:"env_values,omitempty"`

	HasGo  bool   `json:"has_go"`
	GoInfo string `json:"go_info,omitempty"`

	HasNode  bool   `json:"has_node"`
	NodeInfo string `json:"node_info,omitempty"`

	HasNpm  bool   `json:"has_npm"`
	NpmInfo string `json:"npm_info,omitempty"`

	HasBackendBinary bool   `json:"has_backend_binary"`
	BackendBinary    string `json:"backend_binary"`

	HasEmbeddedFrontend bool   `json:"has_embedded_frontend"`
	EmbeddedIndexHTML   string `json:"embedded_index_html"`

	DefaultHost         string `json:"default_host"`
	RecommendedPort     int    `json:"recommended_port"`
	RecommendedFrontDev int    `json:"recommended_frontend_port"`

	Platform string `json:"platform"`
}

func CollectPreflight(projectRoot string) (*Preflight, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		root = detectProjectRoot()
	}
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}

	backendDir := filepath.Join(root, "backend")
	frontendDir := filepath.Join(root, "frontend")
	runtimeDir := filepath.Join(root, ".aca")
	envPath := filepath.Join(root, ".env")

	envValues := map[string]string{}
	hasEnv := false
	if _, err := os.Stat(envPath); err == nil {
		hasEnv = true
		if parsed, err := parseDotEnvFile(envPath); err == nil {
			envValues = parsed
		}
	}

	hasGo, goInfo := detectCommandVersion("go", "version")
	hasNode, nodeInfo := detectCommandVersion("node", "--version")
	hasNpm, npmInfo := detectCommandVersion("npm", "--version")

	backendBinary := filepath.Join(backendDir, "ai-coding-assistant")
	hasBackendBinary := false
	if info, err := os.Stat(backendBinary); err == nil && !info.IsDir() {
		if info.Mode()&0111 != 0 {
			hasBackendBinary = true
		}
	}

	embeddedIndex := filepath.Join(backendDir, "static", "index.html")
	hasEmbedded := false
	if info, err := os.Stat(embeddedIndex); err == nil && !info.IsDir() {
		hasEmbedded = true
	}

	defaultHost := envValues["SERVER_HOST"]
	if strings.TrimSpace(defaultHost) == "" {
		defaultHost = "0.0.0.0"
	}

	recommendedPort := parseEnvInt(envValues["SERVER_PORT"], 34007)
	if port, err := pickAvailableTCPPort(defaultHost, recommendedPort); err == nil {
		recommendedPort = port
	}

	recommendedFront := parseEnvInt(envValues["ACA_FRONTEND_PORT"], 34001)
	if port, err := pickAvailableTCPPort(defaultHost, recommendedFront); err == nil {
		recommendedFront = port
	}

	return &Preflight{
		ProjectRoot:          root,
		BackendDir:           backendDir,
		FrontendDir:          frontendDir,
		RuntimeDir:           runtimeDir,
		EnvPath:              envPath,
		HasEnvFile:           hasEnv,
		EnvValues:            envValues,
		HasGo:                hasGo,
		GoInfo:               goInfo,
		HasNode:              hasNode,
		NodeInfo:             nodeInfo,
		HasNpm:               hasNpm,
		NpmInfo:              npmInfo,
		HasBackendBinary:     hasBackendBinary,
		BackendBinary:        backendBinary,
		HasEmbeddedFrontend:  hasEmbedded,
		EmbeddedIndexHTML:    embeddedIndex,
		DefaultHost:          defaultHost,
		RecommendedPort:      recommendedPort,
		RecommendedFrontDev:  recommendedFront,
		Platform:             runtime.GOOS + "/" + runtime.GOARCH,
	}, nil
}

func detectCommandVersion(command string, versionArg string) (bool, string) {
	path, err := exec.LookPath(command)
	if err != nil {
		return false, ""
	}
	out, err := exec.Command(path, versionArg).CombinedOutput()
	if err != nil {
		return true, strings.TrimSpace(string(out))
	}
	return true, strings.TrimSpace(string(out))
}

func parseEnvInt(raw string, defaultValue int) int {
	text := strings.TrimSpace(raw)
	if text == "" {
		return defaultValue
	}
	var v int
	if err := fmtSscanfInt(text, &v); err != nil {
		return defaultValue
	}
	if v <= 0 || v > 65535 {
		return defaultValue
	}
	return v
}

// fmtSscanfInt is a tiny helper to avoid pulling fmt into this file's imports.
func fmtSscanfInt(text string, out *int) error {
	n := 0
	for _, r := range text {
		if r < '0' || r > '9' {
			return errors.New("not an integer")
		}
		n = n*10 + int(r-'0')
	}
	*out = n
	return nil
}

func detectProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for {
		if fileExists(filepath.Join(dir, "backend", "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
