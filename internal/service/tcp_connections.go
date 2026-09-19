package service

import (
	"bufio"
	"encoding/hex"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

type tcpListener struct {
	address string
	port    int
}

func countActiveTCPConnections(rules []domain.StreamRule, selected string, paths ...string) *uint64 {
	listeners := make([]tcpListener, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled || rule.Protocol != "tcp" || (selected != "" && rule.ID != selected) {
			continue
		}
		listeners = append(listeners, tcpListener{address: strings.Trim(strings.TrimSpace(rule.ListenAddress), "[]"), port: rule.ListenPort})
	}
	var count uint64
	readable := false
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		readable = true
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 4 || fields[3] != "01" {
				continue
			}
			address, port, ok := parseProcTCPAddress(fields[1])
			if !ok {
				continue
			}
			for _, listener := range listeners {
				if listener.port == port && listener.matches(address) {
					count++
					break
				}
			}
		}
		_ = file.Close()
	}
	if !readable {
		return nil
	}
	return &count
}

func parseProcTCPAddress(value string) (net.IP, int, bool) {
	addressHex, portHex, ok := strings.Cut(value, ":")
	if !ok {
		return nil, 0, false
	}
	address, err := hex.DecodeString(addressHex)
	if err != nil || (len(address) != net.IPv4len && len(address) != net.IPv6len) {
		return nil, 0, false
	}
	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return nil, 0, false
	}
	if len(address) == net.IPv4len {
		for left, right := 0, len(address)-1; left < right; left, right = left+1, right-1 {
			address[left], address[right] = address[right], address[left]
		}
	} else {
		for offset := 0; offset < len(address); offset += 4 {
			address[offset], address[offset+3] = address[offset+3], address[offset]
			address[offset+1], address[offset+2] = address[offset+2], address[offset+1]
		}
	}
	return net.IP(address), int(port), true
}

func (listener tcpListener) matches(address net.IP) bool {
	configured := listener.address
	if configured == "" || configured == "*" || configured == "0.0.0.0" {
		return address.To4() != nil
	}
	if configured == "::" {
		return address.To4() == nil
	}
	parsed := net.ParseIP(configured)
	return parsed != nil && parsed.Equal(address)
}
