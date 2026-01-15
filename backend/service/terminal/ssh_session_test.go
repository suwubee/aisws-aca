package terminal

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
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
		readCh:  bToA,
		writeCh: aToB,
		closeFn:    func() { close(aToB) },
		localAddr:  dummyAddr("A"),
		remoteAddr: dummyAddr("B"),
	}

	connB := &bufferedConn{
		readCh:  aToB,
		writeCh: bToA,
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

type sshTestServer struct {
	winCh chan ptyWindowChangeMsg
}

type ptyWindowChangeMsg struct {
	Columns uint32
	Rows    uint32
	Width   uint32
	Height  uint32
}

func (s *sshTestServer) handleChannels(chans <-chan cryptossh.NewChannel) {
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(cryptossh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}

		go s.handleSessionChannel(channel, requests)
	}
}

func (s *sshTestServer) handleSessionChannel(ch cryptossh.Channel, in <-chan *cryptossh.Request) {
	for req := range in {
		switch req.Type {
		case "pty-req":
			req.Reply(true, nil)
		case "window-change":
			var msg ptyWindowChangeMsg
			_ = cryptossh.Unmarshal(req.Payload, &msg)
			select {
			case s.winCh <- msg:
			default:
			}
		case "shell":
			req.Reply(true, nil)
			go func() {
				defer func() { _ = ch.Close() }()
				buf := make([]byte, 4096)
				for {
					n, err := ch.Read(buf)
					if n > 0 {
						_, _ = ch.Write(buf[:n])
					}
					if err != nil {
						_, _ = ch.SendRequest("exit-status", false, cryptossh.Marshal(&struct {
							Status uint32
						}{Status: 0}))
						return
					}
				}
			}()
		default:
			req.Reply(false, nil)
		}
	}
}

type clientBackedSSHSessionProvider struct {
	client *cryptossh.Client
}

func (p *clientBackedSSHSessionProvider) GetSession(_ string) (*cryptossh.Session, error) {
	if p.client == nil {
		return nil, errors.New("missing ssh client")
	}
	return p.client.NewSession()
}

func TestManager_CreateSSHSession_StreamsResizeAndExit(t *testing.T) {
	dsn := fmt.Sprintf("file:ssh_terminal_manager_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	winCh := make(chan ptyWindowChangeMsg, 1)
	srv := &sshTestServer{winCh: winCh}

	serverConn, clientConn := newBufferedConnPair(64)
	serverErrCh := make(chan error, 1)

	go func() {
		hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			serverErrCh <- err
			return
		}
		hostSigner, err := cryptossh.NewSignerFromKey(hostKey)
		if err != nil {
			serverErrCh <- err
			return
		}

		serverConfig := &cryptossh.ServerConfig{
			PasswordCallback: func(meta cryptossh.ConnMetadata, pass []byte) (*cryptossh.Permissions, error) {
				if meta.User() == "tester" && string(pass) == "p@ss" {
					return nil, nil
				}
				return nil, errors.New("password rejected")
			},
		}
		serverConfig.AddHostKey(hostSigner)

		conn, chans, reqs, err := cryptossh.NewServerConn(serverConn, serverConfig)
		if err != nil {
			_ = serverConn.Close()
			serverErrCh <- err
			return
		}

		go cryptossh.DiscardRequests(reqs)
		go srv.handleChannels(chans)

		serverErrCh <- nil
		_ = conn.Wait()
	}()

	clientConfig := &cryptossh.ClientConfig{
		User:            "tester",
		Auth:            []cryptossh.AuthMethod{cryptossh.Password("p@ss")},
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	clientSSHConn, chans, reqs, err := cryptossh.NewClientConn(clientConn, "example:22", clientConfig)
	if err != nil {
		t.Fatalf("NewClientConn error: %v", err)
	}
	t.Cleanup(func() { _ = clientSSHConn.Close() })

	if err := <-serverErrCh; err != nil {
		t.Fatalf("server handshake error: %v", err)
	}

	client := cryptossh.NewClient(clientSSHConn, chans, reqs)
	t.Cleanup(func() { _ = client.Close() })

	if err := model.DB.Create(&model.SSHServer{
		ID:   "srv-1",
		Name: "example",
	}).Error; err != nil {
		t.Fatalf("create SSHServer: %v", err)
	}

	mgr := &Manager{
		config: &config.TerminalConfig{
			DefaultShell:    "/bin/bash",
			ScrollbackBytes: 1024,
		},
		sshManager: &clientBackedSSHSessionProvider{client: client},
	}

	session, err := mgr.CreateSSHSession("srv-1")
	if err != nil {
		t.Fatalf("CreateSSHSession error: %v", err)
	}

	if session.Title() != "SSH: example" {
		t.Fatalf("expected title %q, got %q", "SSH: example", session.Title())
	}
	if mgr.GetSession(session.ID()) == nil {
		t.Fatalf("expected session to be stored in manager")
	}

	var dbSession model.TerminalSession
	if err := model.DB.First(&dbSession, "id = ?", session.ID()).Error; err != nil {
		t.Fatalf("query TerminalSession failed: %v", err)
	}
	if dbSession.ServerID == nil || *dbSession.ServerID != "srv-1" {
		t.Fatalf("expected db server_id %q, got %v", "srv-1", dbSession.ServerID)
	}

	subID, events := session.Subscribe()
	defer session.Unsubscribe(subID)

	if err := session.Write([]byte("hello")); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	dataEvent := waitForEvent(t, events, 2*time.Second, func(ev StreamEvent) bool {
		return ev.Type == StreamEventData && ev.Data != ""
	})

	decoded, err := base64.StdEncoding.DecodeString(dataEvent.Data)
	if err != nil {
		t.Fatalf("decode data event: %v", err)
	}
	if !bytes.Contains(decoded, []byte("hello")) {
		t.Fatalf("expected echoed output to contain %q, got %q", "hello", string(decoded))
	}

	if err := session.Resize(80, 24); err != nil {
		t.Fatalf("Resize error: %v", err)
	}

	select {
	case win := <-winCh:
		if win.Columns != 80 || win.Rows != 24 {
			t.Fatalf("expected window-change cols=80 rows=24, got cols=%d rows=%d", win.Columns, win.Rows)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected window-change request")
	}

	if err := session.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	waitForEvent(t, events, 2*time.Second, func(ev StreamEvent) bool {
		return ev.Type == StreamEventExit
	})

	if session.Status() != "exited" {
		t.Fatalf("expected status %q, got %q", "exited", session.Status())
	}

	if err := model.DB.First(&dbSession, "id = ?", session.ID()).Error; err != nil {
		t.Fatalf("query TerminalSession after close failed: %v", err)
	}
	if dbSession.Status != "exited" {
		t.Fatalf("expected db status %q, got %q", "exited", dbSession.Status)
	}
	if dbSession.ClosedAt == nil {
		t.Fatalf("expected db ClosedAt to be set")
	}
}

func waitForEvent(t *testing.T, ch <-chan StreamEvent, timeout time.Duration, predicate func(StreamEvent) bool) StreamEvent {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("event channel closed")
			}
			if predicate(ev) {
				return ev
			}
		case <-timer.C:
			t.Fatalf("timeout waiting for event")
		}
	}
}
