# Tests

- `go test ./...`: validation and Nginx rendering unit tests.
- `integration.sh`: starts the real bundled Nginx, calls the management API through a Unix Socket, and verifies HTTP and HTTPS reverse proxying.
- `NGINX_TEST_BIN=/absolute/path/to/nginx go test ./internal/nginx ./internal/service`: additionally validates generated configuration with the required Nginx version, and runs `TestDashboardWithRealNginx` against real HTTP requests, combined/classified 4xx and 5xx rates, per-rule attribution, response rates, process uptime/worker counts, private status polling and log rotation. The binary must match the host OS/architecture.
- `go test -race ./internal/metrics ./internal/service ./internal/httpapi`: verifies collection, restart/counter reset, absent history, partial log lines, checkpoints, rotation, rule configuration state and in-use certificate warnings.

Integration tests select the native Linux architecture (x86_64 or ARM64). Set `INTEGRATION_SERVER_BIN` to an already built server to test the packaged executable without rebuilding the frontend or Go binary. Local test HTTP requests bypass environment proxies. Lifecycle checks require a native architecture: process identity checks are not reliable under user-mode CPU emulation.
