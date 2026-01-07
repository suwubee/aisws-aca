package ssh

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	secretservice "github.com/ai-coding-assistant/service/secret"
	cryptossh "golang.org/x/crypto/ssh"
)

type dummyAddr string

func (a dummyAddr) Network() string { return "mem" }
func (a dummyAddr) String() string  { return string(a) }

type bufferedConn struct {
	readCh    chan []byte
	writeCh   chan []byte
	closeOnce sync.Once
	closeFn   func()

	mu     sync.Mutex
	closed bool
	buffer []byte

	localAddr  net.Addr
	remoteAddr net.Addr
}

func newBufferedConnPair(buffer int) (net.Conn, net.Conn) {
	aToB := make(chan []byte, buffer)
	bToA := make(chan []byte, buffer)

	connA := &bufferedConn{
		readCh:     bToA,
		writeCh:    aToB,
		closeFn:    func() { close(aToB) },
		localAddr:  dummyAddr("A"),
		remoteAddr: dummyAddr("B"),
	}

	connB := &bufferedConn{
		readCh:     aToB,
		writeCh:    bToA,
		closeFn:    func() { close(bToA) },
		localAddr:  dummyAddr("B"),
		remoteAddr: dummyAddr("A"),
	}

	return connA, connB
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	if len(c.buffer) > 0 {
		n := copy(p, c.buffer)
		c.buffer = c.buffer[n:]
		c.mu.Unlock()
		return n, nil
	}
	c.mu.Unlock()

	data, ok := <-c.readCh
	if !ok {
		return 0, io.EOF
	}

	c.mu.Lock()
	c.buffer = append(c.buffer, data...)
	n := copy(p, c.buffer)
	c.buffer = c.buffer[n:]
	c.mu.Unlock()
	return n, nil
}

func (c *bufferedConn) Write(p []byte) (n int, err error) {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return 0, io.ErrClosedPipe
	}

	data := make([]byte, len(p))
	copy(data, p)

	defer func() {
		if r := recover(); r != nil {
			n = 0
			err = io.ErrClosedPipe
		}
	}()

	c.writeCh <- data
	return len(p), nil
}

func (c *bufferedConn) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		if c.closeFn != nil {
			c.closeFn()
		}
	})
	return nil
}

func (c *bufferedConn) LocalAddr() net.Addr  { return c.localAddr }
func (c *bufferedConn) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *bufferedConn) SetDeadline(_ time.Time) error      { return nil }
func (c *bufferedConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *bufferedConn) SetWriteDeadline(_ time.Time) error { return nil }

func newHostSigner(t *testing.T) cryptossh.Signer {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := cryptossh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatalf("create host signer: %v", err)
	}
	return hostSigner
}

func newServerConfig(t *testing.T, username, password string, allowedPublicKey cryptossh.PublicKey) *cryptossh.ServerConfig {
	t.Helper()

	config := &cryptossh.ServerConfig{}

	if username != "" && password != "" {
		config.PasswordCallback = func(meta cryptossh.ConnMetadata, pass []byte) (*cryptossh.Permissions, error) {
			if meta.User() == username && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected")
		}
	}

	if username != "" && allowedPublicKey != nil {
		config.PublicKeyCallback = func(meta cryptossh.ConnMetadata, key cryptossh.PublicKey) (*cryptossh.Permissions, error) {
			if meta.User() == username && bytes.Equal(key.Marshal(), allowedPublicKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("public key rejected")
		}
	}

	config.AddHostKey(newHostSigner(t))

	return config
}

func pipeDialer(serverConfig *cryptossh.ServerConfig) func(network, addr string, config *cryptossh.ClientConfig) (*cryptossh.Client, error) {
	return func(_ string, addr string, clientConfig *cryptossh.ClientConfig) (*cryptossh.Client, error) {
		clientConn, serverConn := newBufferedConnPair(64)
		serverErrCh := make(chan error, 1)

		go func() {
			serverSSHConn, chans, reqs, err := cryptossh.NewServerConn(serverConn, serverConfig)
			if err != nil {
				_ = serverConn.Close()
				serverErrCh <- err
				return
			}

			go cryptossh.DiscardRequests(reqs)
			go handleServerChannels(chans)

			serverErrCh <- nil
			_ = serverSSHConn.Wait()
		}()

		clientSSHConn, chans, reqs, err := cryptossh.NewClientConn(clientConn, addr, clientConfig)
		if err != nil {
			_ = clientConn.Close()
			return nil, err
		}

		if err := <-serverErrCh; err != nil {
			_ = clientSSHConn.Close()
			return nil, err
		}

		return cryptossh.NewClient(clientSSHConn, chans, reqs), nil
	}
}

func handleServerChannels(chans <-chan cryptossh.NewChannel) {
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(cryptossh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}

		go func(ch cryptossh.Channel, in <-chan *cryptossh.Request) {
			for req := range in {
				switch req.Type {
				case "exec":
					req.Reply(true, nil)
					_, _ = ch.Write([]byte("ok"))
					_, _ = ch.SendRequest("exit-status", false, cryptossh.Marshal(&struct {
						Status uint32
					}{Status: 0}))
					_ = ch.Close()
					return
				default:
					req.Reply(false, nil)
				}
			}
		}(channel, requests)
	}
}

func TestSSHManager_PasswordAuth_ConnectReuseAndSession(t *testing.T) {
	serverConfig := newServerConfig(t, "tester", "p@ss", nil)

	dsn := fmt.Sprintf("file:ssh_manager_password_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	key := secretservice.DeriveKey("test-master-key")
	encryptedPassword, err := secretservice.EncryptAESGCMBase64(key, "p@ss")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}

	server := model.SSHServer{
		ID:         "srv-1",
		Name:       "srv",
		Host:       "example",
		Port:       22,
		Username:   "tester",
		AuthType:   "password",
		Password:   encryptedPassword,
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	manager := NewSSHManagerWithKey(key)
	manager.dialFunc = pipeDialer(serverConfig)
	t.Cleanup(manager.Close)

	if err := manager.TestConnection(&server); err != nil {
		t.Fatalf("TestConnection error: %v", err)
	}

	client1, err := manager.Connect(server.ID)
	if err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	client2, err := manager.Connect(server.ID)
	if err != nil {
		t.Fatalf("Connect (reuse) error: %v", err)
	}
	if client1 != client2 {
		t.Fatalf("expected client reuse")
	}

	session, err := manager.GetSession(server.ID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	output, err := session.CombinedOutput("echo ok")
	if err != nil {
		t.Fatalf("CombinedOutput error: %v", err)
	}
	if !bytes.Contains(output, []byte("ok")) {
		t.Fatalf("expected output to contain %q, got %q", "ok", string(output))
	}

	manager.Disconnect(server.ID)
	client3, err := manager.Connect(server.ID)
	if err != nil {
		t.Fatalf("Connect after Disconnect error: %v", err)
	}
	if client3 == client1 {
		t.Fatalf("expected new client after Disconnect")
	}
}

func TestSSHManager_KeyAuth_WithPassphrase(t *testing.T) {
	userKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate user key: %v", err)
	}

	userSigner, err := cryptossh.NewSignerFromKey(userKey)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	passphrase := "secret-pass"
	encryptedBlock, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(userKey), []byte(passphrase), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("EncryptPEMBlock: %v", err)
	}
	privateKeyPEM := string(pem.EncodeToMemory(encryptedBlock))

	serverConfig := newServerConfig(t, "tester", "", userSigner.PublicKey())

	dsn := fmt.Sprintf("file:ssh_manager_key_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	key := secretservice.DeriveKey("test-master-key")

	encryptedPrivateKey, err := secretservice.EncryptAESGCMBase64(key, privateKeyPEM)
	if err != nil {
		t.Fatalf("encrypt private key: %v", err)
	}
	encryptedPassphrase, err := secretservice.EncryptAESGCMBase64(key, passphrase)
	if err != nil {
		t.Fatalf("encrypt passphrase: %v", err)
	}

	server := model.SSHServer{
		ID:         "srv-2",
		Name:       "srv",
		Host:       "example",
		Port:       22,
		Username:   "tester",
		AuthType:   "key",
		PrivateKey: encryptedPrivateKey,
		Passphrase: encryptedPassphrase,
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	manager := NewSSHManagerWithKey(key)
	manager.dialFunc = pipeDialer(serverConfig)
	t.Cleanup(manager.Close)

	if err := manager.TestConnection(&server); err != nil {
		t.Fatalf("TestConnection error: %v", err)
	}

	session, err := manager.GetSession(server.ID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	output, err := session.CombinedOutput("echo ok")
	if err != nil {
		t.Fatalf("CombinedOutput error: %v", err)
	}
	if !bytes.Contains(output, []byte("ok")) {
		t.Fatalf("expected output to contain %q, got %q", "ok", string(output))
	}
}

func TestSSHManager_CleanupIdleConnections(t *testing.T) {
	serverConfig := newServerConfig(t, "tester", "p@ss", nil)

	dsn := fmt.Sprintf("file:ssh_manager_cleanup_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	key := secretservice.DeriveKey("test-master-key")
	encryptedPassword, err := secretservice.EncryptAESGCMBase64(key, "p@ss")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}

	server := model.SSHServer{
		ID:         "srv-3",
		Name:       "srv",
		Host:       "example",
		Port:       22,
		Username:   "tester",
		AuthType:   "password",
		Password:   encryptedPassword,
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	manager := NewSSHManagerWithKey(key)
	manager.dialFunc = pipeDialer(serverConfig)
	t.Cleanup(manager.Close)

	if _, err := manager.Connect(server.ID); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	manager.idleTimeout = 100 * time.Millisecond

	loaded, ok := manager.connections.Load(server.ID)
	if !ok {
		t.Fatalf("expected pooled client to exist")
	}
	entry := loaded.(*pooledClient)
	entry.mu.Lock()
	entry.lastUsed = time.Now().Add(-time.Second)
	entry.mu.Unlock()

	manager.cleanupIdleConnections(time.Now())

	if _, ok := manager.connections.Load(server.ID); ok {
		t.Fatalf("expected pooled client to be cleaned up")
	}
}

func TestSSHManager_ExecuteCommand(t *testing.T) {
	serverConfig := newServerConfig(t, "tester", "p@ss", nil)

	dsn := fmt.Sprintf("file:ssh_manager_exec_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	key := secretservice.DeriveKey("test-master-key")
	encryptedPassword, err := secretservice.EncryptAESGCMBase64(key, "p@ss")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}

	server := model.SSHServer{
		ID:         "srv-exec",
		Name:       "srv",
		Host:       "example",
		Port:       22,
		Username:   "tester",
		AuthType:   "password",
		Password:   encryptedPassword,
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	manager := NewSSHManagerWithKey(key)
	manager.dialFunc = pipeDialer(serverConfig)
	t.Cleanup(manager.Close)

	output, err := manager.ExecuteCommand(server.ID, "echo ok")
	if err != nil {
		t.Fatalf("ExecuteCommand error: %v", err)
	}
	if !strings.Contains(output, "ok") {
		t.Fatalf("expected output to contain %q, got %q", "ok", output)
	}

	if _, err := manager.ExecuteCommand("", "echo ok"); err == nil {
		t.Fatalf("expected error for empty serverID")
	}
	if _, err := manager.ExecuteCommand(server.ID, " "); err == nil {
		t.Fatalf("expected error for empty command")
	}
}

func TestSSHManager_BatchExecute(t *testing.T) {
	serverConfig := newServerConfig(t, "tester", "p@ss", nil)

	dsn := fmt.Sprintf("file:ssh_manager_batch_exec_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	key := secretservice.DeriveKey("test-master-key")
	encryptedPassword, err := secretservice.EncryptAESGCMBase64(key, "p@ss")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}

	servers := []model.SSHServer{
		{
			ID:         "srv-batch-1",
			Name:       "srv1",
			Host:       "example",
			Port:       22,
			Username:   "tester",
			AuthType:   "password",
			Password:   encryptedPassword,
			LastStatus: "unknown",
		},
		{
			ID:         "srv-batch-2",
			Name:       "srv2",
			Host:       "example",
			Port:       22,
			Username:   "tester",
			AuthType:   "password",
			Password:   encryptedPassword,
			LastStatus: "unknown",
		},
	}
	for _, server := range servers {
		if err := model.DB.Create(&server).Error; err != nil {
			t.Fatalf("create server %s: %v", server.ID, err)
		}
	}

	manager := NewSSHManagerWithKey(key)
	manager.dialFunc = pipeDialer(serverConfig)
	t.Cleanup(manager.Close)

	results := manager.BatchExecute([]string{"srv-batch-1", "srv-batch-2"}, "echo ok")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, id := range []string{"srv-batch-1", "srv-batch-2"} {
		res, ok := results[id]
		if !ok {
			t.Fatalf("missing result for %s", id)
		}
		if res.Error != "" {
			t.Fatalf("unexpected error for %s: %s", id, res.Error)
		}
		if !strings.Contains(res.Output, "ok") {
			t.Fatalf("expected output for %s to contain %q, got %q", id, "ok", res.Output)
		}
	}

	results = manager.BatchExecute([]string{"srv-batch-1"}, " ")
	if results["srv-batch-1"].Error == "" {
		t.Fatalf("expected error for empty command")
	}

	results = manager.BatchExecute([]string{"missing"}, "echo ok")
	if results["missing"].Error == "" {
		t.Fatalf("expected error for missing server")
	}
}
