# Tests

- `go test ./...`: validation and Nginx rendering unit tests.
- `integration.sh`: starts the real bundled Nginx, calls the management API through a Unix Socket, and verifies HTTP and HTTPS reverse proxying.
- `NGINX_TEST_BIN=/absolute/path/to/nginx go test ./internal/nginx ./internal/service`: additionally validates generated configuration with the required Nginx version, and runs `TestDashboardWithRealNginx` against real HTTP requests, combined/classified 4xx and 5xx rates, per-rule attribution, response rates, process uptime/worker counts, private status polling and log rotation. The binary must match the host OS/architecture.
- `go test -race ./internal/metrics ./internal/service ./internal/httpapi`: verifies collection, restart/counter reset, absent history, partial log lines, checkpoints, rotation, rule configuration state and in-use certificate warnings.

Integration tests select the native Linux architecture (x86_64 or ARM64). Set `INTEGRATION_SERVER_BIN` to an already built server to test the packaged executable without rebuilding the frontend or Go binary. Local test HTTP requests bypass environment proxies. Lifecycle checks require a native architecture: process identity checks are not reliable under user-mode CPU emulation.

- `low-ports.sh <native Linux server binary>`: isolated Linux root integration test. Build its server with `go build -tags full_ports`; the test stages the full-ports lifecycle helper in a temporary directory. Requires an existing non-root `nginx-web` user, bash, runuser, libcap tools, Python, curl and OpenSSL. Set the container sysctl `net.ipv4.ip_unprivileged_port_start=1024` and copy the repository/binary into the container (do not grant capabilities on a host-mounted checkout). Checks denied unprivileged bind, actual port 80 response, backend/Nginx UIDs and capabilities, HTTPS 443/TCP 81/UDP 53 traffic, rejected root-user/symlink targets, config testing/reload, upgrade and start repair after capability loss. `KEEP_TEST_ROOT=1` retains the instance for browser QA. fnOS device installation remains a separate check.

- `make test` checks both standard and full-ports backend validation. `make build-all` verifies the four mode/architecture packages. Standard builds must reject low listening ports while allowing low upstream ports.
