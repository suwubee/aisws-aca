package setupwizard

import (
	"net"
	"testing"
)

func TestFormatURL_IPv6(t *testing.T) {
	got := formatURL("2001:db8::1", 34007)
	if got != "http://[2001:db8::1]:34007" {
		t.Fatalf("got %q", got)
	}
}

func TestLocalProbeHost(t *testing.T) {
	if got := localProbeHost("0.0.0.0"); got != "127.0.0.1" {
		t.Fatalf("0.0.0.0 -> %q", got)
	}
	if got := localProbeHost("::"); got != "::1" {
		t.Fatalf(":: -> %q", got)
	}
	if got := localProbeHost("192.0.2.10"); got != "192.0.2.10" {
		t.Fatalf("192.0.2.10 -> %q", got)
	}
}

func TestValidateAndNormalize_SwitchesBackendPortWhenInUse(t *testing.T) {
	skipIfSocketNotPermitted(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	occupied := ln.Addr().(*net.TCPAddr).Port
	if occupied == 0 {
		t.Fatalf("expected occupied port > 0")
	}

	pre := &Preflight{
		DefaultHost:     "127.0.0.1",
		RecommendedPort: occupied,
	}

	cfg := &SetupConfig{
		BackendMode:   "binary",
		FrontendMode:  "embedded",
		ServerHost:    "127.0.0.1",
		ServerPort:    occupied,
		DatabaseDSN:   "./data/aca.db",
		AdminUsername: "admin",
		AdminPassword: "admin123",
		JWTSecret:     "auto",
		SystemRule: SystemRuleConfig{
			Name:         "x",
			ApprovalMode: "manual",
		},
	}

	if err := validateAndNormalize(cfg, pre, func(level, msg string) {}); err != nil {
		t.Fatal(err)
	}
	if cfg.ServerPort == occupied {
		t.Fatalf("expected backend port to change when occupied, got %d", cfg.ServerPort)
	}
}
