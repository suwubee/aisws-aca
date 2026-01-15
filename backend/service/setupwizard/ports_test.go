package setupwizard

import (
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"
)

func skipIfSocketNotPermitted(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		_ = ln.Close()
		return
	}
	if errors.Is(err, syscall.EPERM) || strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
		t.Skip("socket operations are not permitted in this sandbox")
	}
	t.Fatalf("failed to listen on loopback: %v", err)
}

func TestPickAvailableTCPPort_PrefersRandomWhenZero(t *testing.T) {
	skipIfSocketNotPermitted(t)

	port, err := pickAvailableTCPPort("127.0.0.1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("port=%d, want 1..65535", port)
	}
}

func TestPickAvailableTCPPort_SwitchesWhenInUse(t *testing.T) {
	skipIfSocketNotPermitted(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || tcpAddr.Port == 0 {
		t.Fatalf("unexpected addr: %v", ln.Addr())
	}

	got, err := pickAvailableTCPPort("127.0.0.1", tcpAddr.Port)
	if err != nil {
		t.Fatal(err)
	}
	if got == tcpAddr.Port {
		t.Fatalf("expected different port when preferred is in use, got=%d", got)
	}
}

func TestPickAvailableTCPPort_InvalidPreferred(t *testing.T) {
	if _, err := pickAvailableTCPPort("127.0.0.1", -1); err == nil {
		t.Fatalf("expected error for invalid port")
	}
}
