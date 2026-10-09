package platform

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

type Runtime struct {
	Standalone bool
	Listen     string
}

func LoadRuntime() (Runtime, error) {
	switch os.Getenv("FNPROXY_MODE") {
	case "", "fnos":
		if os.Getenv("FNPROXY_LISTEN") != "" {
			return Runtime{}, fmt.Errorf("FNPROXY_LISTEN 仅可用于 FNPROXY_MODE=standalone")
		}
		return Runtime{}, nil
	case "standalone":
		address := firstNonEmpty(os.Getenv("FNPROXY_LISTEN"), ":8080")
		_, port, err := net.SplitHostPort(address)
		number, parseErr := strconv.Atoi(port)
		if err != nil || parseErr != nil || number < 1 || number > 65535 {
			return Runtime{}, fmt.Errorf("FNPROXY_LISTEN 必须是 host:port，端口范围为 1–65535")
		}
		return Runtime{Standalone: true, Listen: address}, nil
	default:
		return Runtime{}, fmt.Errorf("FNPROXY_MODE 必须是 fnos 或 standalone")
	}
}
