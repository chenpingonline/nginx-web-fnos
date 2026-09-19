package service

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestCountActiveTCPConnections(t *testing.T) {
	dir := t.TempDir()
	tcpPath := filepath.Join(dir, "tcp")
	tcp6Path := filepath.Join(dir, "tcp6")
	line := func(index int, address string, port int, state string) string {
		return fmt.Sprintf("%d: %s:%04X 00000000:0000 %s 00000000:00000000 00:00000000 00000000 0 0 0", index, address, port, state)
	}
	tcp := "sl local_address rem_address st\n" +
		line(0, "0100007F", 13308, "01") + "\n" +
		line(1, "0100007F", 13308, "06") + "\n" +
		line(2, "0100007F", 9443, "01") + "\n" +
		line(3, "0200007F", 9443, "01") + "\n"
	tcp6 := "sl local_address rem_address st\n" + line(0, "00000000000000000000000001000000", 9000, "01") + "\n"
	if err := os.WriteFile(tcpPath, []byte(tcp), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tcp6Path, []byte(tcp6), 0o600); err != nil {
		t.Fatal(err)
	}
	rules := []domain.StreamRule{
		{ID: "wildcard", Enabled: true, Protocol: "tcp", ListenAddress: "0.0.0.0", ListenPort: 13308},
		{ID: "exact", Enabled: true, Protocol: "tcp", ListenAddress: "127.0.0.1", ListenPort: 9443},
		{ID: "ipv6", Enabled: true, Protocol: "tcp", ListenAddress: "::", ListenPort: 9000},
		{ID: "udp", Enabled: true, Protocol: "udp", ListenAddress: "0.0.0.0", ListenPort: 13308},
	}
	if got := countActiveTCPConnections(rules, "", tcpPath, tcp6Path); got == nil || *got != 3 {
		t.Fatalf("all active TCP connections = %v, want 3", got)
	}
	if got := countActiveTCPConnections(rules, "wildcard", tcpPath, tcp6Path); got == nil || *got != 1 {
		t.Fatalf("selected active TCP connections = %v, want 1", got)
	}
	if got := countActiveTCPConnections(rules, "udp", tcpPath, tcp6Path); got == nil || *got != 0 {
		t.Fatalf("UDP selection must not report TCP connections: %v", got)
	}
	if got := countActiveTCPConnections(rules, "", filepath.Join(dir, "missing")); got != nil {
		t.Fatalf("missing proc data must be unavailable, got %v", *got)
	}
}

func TestParseProcTCPAddress(t *testing.T) {
	address, port, ok := parseProcTCPAddress("0100007F:3306")
	if !ok || address.String() != "127.0.0.1" || port != 13062 {
		t.Fatalf("unexpected IPv4 endpoint: %s:%d (%v)", address, port, ok)
	}
	address, port, ok = parseProcTCPAddress("00000000000000000000000001000000:2328")
	if !ok || address.String() != "::1" || port != 9000 {
		t.Fatalf("unexpected IPv6 endpoint: %s:%d (%v)", address, port, ok)
	}
}
