package ssh

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	secretservice "github.com/ai-coding-assistant/service/secret"
	cryptossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type pooledClient struct {
	mu       sync.Mutex
	client   *cryptossh.Client
	lastUsed time.Time
}

type ExecuteResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// SSHManager SSH连接管理器
type SSHManager struct {
	connections sync.Map

	encryptionKey []byte
	dialTimeout   time.Duration
	idleTimeout   time.Duration
	dialFunc      func(network, addr string, config *cryptossh.ClientConfig) (*cryptossh.Client, error)

	cleanupInterval time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
}

const (
	defaultDialTimeout     = 10 * time.Second
	defaultIdleTimeout     = 5 * time.Minute
	defaultCleanupInterval = time.Minute
)

// NewSSHManager 创建SSH连接管理器
func NewSSHManager(masterKey string) *SSHManager {
	return NewSSHManagerWithKey(secretservice.DeriveKey(masterKey))
}

// NewSSHManagerWithKey 创建SSH连接管理器
func NewSSHManagerWithKey(encryptionKey []byte) *SSHManager {
	m := &SSHManager{
		encryptionKey:   encryptionKey,
		dialTimeout:     defaultDialTimeout,
		idleTimeout:     defaultIdleTimeout,
		cleanupInterval: defaultCleanupInterval,
		stopCh:          make(chan struct{}),
	}

	go m.reapIdleConnections()

	return m
}

// Close 停止后台清理并关闭所有连接
func (m *SSHManager) Close() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})

	m.connections.Range(func(key, value any) bool {
		entry, ok := value.(*pooledClient)
		if !ok {
			m.connections.Delete(key)
			return true
		}

		entry.mu.Lock()
		client := entry.client
		entry.client = nil
		entry.mu.Unlock()

		if client != nil {
			_ = client.Close()
		}

		m.connections.Delete(key)
		return true
	})
}

// Connect 建立SSH连接（支持连接复用）
func (m *SSHManager) Connect(serverID string) (*cryptossh.Client, error) {
	id := strings.TrimSpace(serverID)
	if id == "" {
		return nil, errors.New("serverID is required")
	}

	now := time.Now()

	loaded, _ := m.connections.LoadOrStore(id, &pooledClient{lastUsed: now})
	entry := loaded.(*pooledClient)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if entry.client != nil {
		if m.idleTimeout > 0 && now.Sub(entry.lastUsed) > m.idleTimeout {
			_ = entry.client.Close()
			entry.client = nil
		} else {
			entry.lastUsed = now
			return entry.client, nil
		}
	}

	server, err := m.loadServer(id)
	if err != nil {
		return nil, err
	}

	client, err := m.dial(server)
	if err != nil {
		return nil, err
	}

	entry.client = client
	entry.lastUsed = now

	return client, nil
}

// Disconnect 断开连接
func (m *SSHManager) Disconnect(serverID string) {
	id := strings.TrimSpace(serverID)
	if id == "" {
		return
	}

	loaded, ok := m.connections.Load(id)
	if !ok {
		return
	}

	entry, ok := loaded.(*pooledClient)
	if !ok {
		m.connections.Delete(id)
		return
	}

	entry.mu.Lock()
	client := entry.client
	entry.client = nil
	entry.mu.Unlock()

	if client != nil {
		_ = client.Close()
	}

	m.connections.Delete(id)
}

// TestConnection 测试连接
func (m *SSHManager) TestConnection(server *model.SSHServer) error {
	if server == nil {
		return errors.New("server is required")
	}

	client, err := m.dial(server)
	if err != nil {
		return err
	}
	_ = client.Close()
	return nil
}

// GetSession 获取会话
func (m *SSHManager) GetSession(serverID string) (*cryptossh.Session, error) {
	client, err := m.Connect(serverID)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err == nil {
		return session, nil
	}

	m.Disconnect(serverID)

	client, err = m.Connect(serverID)
	if err != nil {
		return nil, err
	}

	return client.NewSession()
}

func (m *SSHManager) reapIdleConnections() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupIdleConnections(time.Now())
		case <-m.stopCh:
			return
		}
	}
}

func (m *SSHManager) cleanupIdleConnections(now time.Time) {
	if m.idleTimeout <= 0 {
		return
	}

	m.connections.Range(func(key, value any) bool {
		entry, ok := value.(*pooledClient)
		if !ok {
			m.connections.Delete(key)
			return true
		}

		entry.mu.Lock()
		expired := entry.client != nil && now.Sub(entry.lastUsed) > m.idleTimeout
		client := entry.client
		if expired {
			entry.client = nil
		}
		entry.mu.Unlock()

		if expired {
			if client != nil {
				_ = client.Close()
			}
			m.connections.Delete(key)
		}

		return true
	})
}

func (m *SSHManager) loadServer(serverID string) (*model.SSHServer, error) {
	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("server not found")
		}
		return nil, err
	}

	return &server, nil
}

func (m *SSHManager) dial(server *model.SSHServer) (*cryptossh.Client, error) {
	cfg, addr, err := m.buildClientConfig(server)
	if err != nil {
		return nil, err
	}

	if m.dialFunc != nil {
		return m.dialFunc("tcp", addr, cfg)
	}

	client, err := cryptossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (m *SSHManager) buildClientConfig(server *model.SSHServer) (*cryptossh.ClientConfig, string, error) {
	host := strings.TrimSpace(server.Host)
	if host == "" {
		return nil, "", errors.New("missing host")
	}

	port := server.Port
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		return nil, "", errors.New("invalid port")
	}

	username := strings.TrimSpace(server.Username)
	if username == "" {
		return nil, "", errors.New("missing username")
	}

	authType := strings.ToLower(strings.TrimSpace(server.AuthType))
	if authType == "" {
		authType = "password"
	}

	var authMethods []cryptossh.AuthMethod

	switch authType {
	case "password":
		password, err := m.decryptSecret(server.Password)
		if err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(password) == "" {
			return nil, "", errors.New("missing password")
		}
		authMethods = []cryptossh.AuthMethod{cryptossh.Password(password)}
	case "key":
		privateKey, err := m.decryptSecret(server.PrivateKey)
		if err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(privateKey) == "" {
			return nil, "", errors.New("missing private key")
		}

		passphrase, err := m.decryptSecret(server.Passphrase)
		if err != nil {
			return nil, "", err
		}

		var signer cryptossh.Signer
		if strings.TrimSpace(passphrase) != "" {
			signer, err = cryptossh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
		} else {
			signer, err = cryptossh.ParsePrivateKey([]byte(privateKey))
		}
		if err != nil {
			return nil, "", errors.New("invalid private key")
		}

		authMethods = []cryptossh.AuthMethod{cryptossh.PublicKeys(signer)}
	default:
		return nil, "", errors.New("invalid auth type")
	}

	if len(authMethods) == 0 {
		return nil, "", errors.New("no auth method configured")
	}

	clientConfig := &cryptossh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(),
		Timeout:         m.dialTimeout,
	}

	return clientConfig, fmt.Sprintf("%s:%d", host, port), nil
}

func (m *SSHManager) decryptSecret(ciphertext string) (string, error) {
	trimmed := strings.TrimSpace(ciphertext)
	if trimmed == "" {
		return "", nil
	}
	if len(m.encryptionKey) == 0 {
		return "", errors.New("missing encryption key")
	}

	plaintext, err := secretservice.DecryptAESGCMBase64(m.encryptionKey, trimmed)
	if err != nil {
		return "", errors.New("failed to decrypt secret")
	}

	return plaintext, nil
}

func (m *SSHManager) ExecuteCommand(serverID, command string) (string, error) {
	if m == nil {
		return "", errors.New("ssh manager is nil")
	}

	id := strings.TrimSpace(serverID)
	if id == "" {
		return "", errors.New("serverID is required")
	}

	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", errors.New("command is required")
	}

	session, err := m.GetSession(id)
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

func (m *SSHManager) BatchExecute(serverIDs []string, command string) map[string]ExecuteResult {
	results := make(map[string]ExecuteResult)

	if m == nil {
		for _, serverID := range serverIDs {
			id := strings.TrimSpace(serverID)
			if id == "" {
				continue
			}
			results[id] = ExecuteResult{Error: "ssh manager is nil"}
		}
		return results
	}

	cmd := strings.TrimSpace(command)
	if cmd == "" {
		for _, serverID := range serverIDs {
			id := strings.TrimSpace(serverID)
			if id == "" {
				continue
			}
			results[id] = ExecuteResult{Error: "command is required"}
		}
		return results
	}

	seen := make(map[string]struct{}, len(serverIDs))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, serverID := range serverIDs {
		id := strings.TrimSpace(serverID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		wg.Add(1)
		go func(sid string) {
			defer wg.Done()
			output, err := m.ExecuteCommand(sid, cmd)
			result := ExecuteResult{Output: output}
			if err != nil {
				result.Error = err.Error()
			}

			mu.Lock()
			results[sid] = result
			mu.Unlock()
		}(id)
	}

	wg.Wait()

	return results
}
