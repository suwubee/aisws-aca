package setupwizard

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

func isTCPPortAvailable(host string, port int) (bool, error) {
	h := strings.TrimSpace(host)
	if h == "" {
		h = "0.0.0.0"
	}
	if port < 0 || port > 65535 {
		return false, fmt.Errorf("invalid port: %d", port)
	}
	addr := fmt.Sprintf("%s:%d", h, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Treat as "not available" for common cases.
		return false, nil
	}
	_ = ln.Close()
	return true, nil
}

func pickAvailableTCPPort(host string, preferred int) (int, error) {
	if preferred < 0 || preferred > 65535 {
		return 0, fmt.Errorf("invalid port: %d", preferred)
	}

	if preferred == 0 {
		return pickRandomTCPPort(host)
	}

	ok, err := isTCPPortAvailable(host, preferred)
	if err != nil {
		return 0, err
	}
	if ok {
		return preferred, nil
	}
	return pickRandomTCPPort(host)
}

func pickRandomTCPPort(host string) (int, error) {
	h := strings.TrimSpace(host)
	if h == "" {
		h = "0.0.0.0"
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", h, 0))
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr := ln.Addr().String()
	_, portStr, ok := strings.Cut(addr, ":")
	if !ok {
		return 0, errors.New("failed to parse listener address")
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if ok {
		return tcpAddr.Port, nil
	}

	// Fallback parse.
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		return 0, err
	}
	return port, nil
}

