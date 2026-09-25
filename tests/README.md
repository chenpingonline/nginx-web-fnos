# Tests

- `go test ./...`: validation and Nginx rendering unit tests.
- `integration.sh`: starts the real bundled Nginx, calls the management API through a Unix Socket, and verifies HTTP and HTTPS reverse proxying.
- `NGINX_TEST_BIN=/absolute/path/to/nginx go test ./internal/nginx ./internal/service`: additionally validates generated configuration with the required Nginx version, and runs `TestDashboardWithRealNginx` against real HTTP requests, combined/classified 4xx and 5xx rates, per-rule attribution, response rates, process uptime/worker counts, private status polling and log rotation. The binary must match the host OS/architecture.
- `go test -race ./internal/metrics ./internal/service ./internal/httpapi`: verifies collection, restart/counter reset, absent history, partial log lines, checkpoints, rotation, rule configuration state and in-use certificate warnings.

Integration tests select the native Linux architecture (x86_64 or ARM64). Set `INTEGRATION_SERVER_BIN` to an already built server to test the packaged executable without rebuilding the frontend or Go binary. Local test HTTP requests bypass environment proxies. Lifecycle checks require a native architecture: process identity checks are not reliable under user-mode CPU emulation.

- `low-ports.sh <native Linux server binary> <native Linux repair-app-data binary>`: isolated Linux root integration test. Build its server with `go build -tags full_ports`; the test stages the full-ports lifecycle helper in a temporary directory. Requires an existing non-root `nginx-web` user, bash, runuser, libcap tools, Python, curl and OpenSSL. Set the container sysctl `net.ipv4.ip_unprivileged_port_start=1024` and copy the repository/binary into the container (do not grant capabilities on a host-mounted checkout). Checks denied unprivileged bind, actual port 80 response, backend/Nginx UIDs and capabilities, HTTPS 443/TCP 81/UDP 53 traffic, rejected root-user/symlink targets, config testing/reload, upgrade and start repair after capability loss. `KEEP_TEST_ROOT=1` retains the instance for browser QA. fnOS device installation remains a separate check.

- `make test` checks both standard and full-ports backend validation. `make build-all` verifies the four mode/architecture packages. Standard builds must reject low listening ports while allowing low upstream ports.

- `bash tests/install-restore.sh` (non-root): checks copy failures, unreadable backups, initialization failure/retry and unwritable logging. Requires Bash; GNU coreutils is recommended for Linux parity. Verifies detailed errors, retained backup data, and no configuration contents in logs.

- `tests/repair-app-data.sh <helper>`: isolated Linux root test for retained private data with an old UID, ordinary-user log writes/restoration, symlink isolation and hard-link rejection. Build helper with `CGO_ENABLED=0 GOOS=linux GOARCH=<target> go build -o <helper> ./cmd/repair-app-data`; only full-ports packages ship it.
- Restore diagnostics also verify temporary-log fallback and that a failed new backup leaves the prior backup intact.

- Edition-switch regression: store tests load full-ports state with 80/443 defaults, inherited HTTP listeners and UDP 53, verify standard-mode pausing/persistence and rejection of re-enabling unsupported listeners, then reactivate after correcting the group port. Nginx tests ensure dormant low defaults do not generate a network listener.

- `NGINX_TEST_BIN=<nginx> go test ./internal/service -run 'TestImmediateToggle|TestToggleRequires'`: real HTTP/TCP immediate enable/disable, generated-config removal, unrelated draft isolation, persistence-failure rollback, and preservation of a stopped Nginx. Run in an isolated environment with ports 19989–19992 available.

- Automatic upgrade migration: `TestAutomaticRuntimeMigration` covers applied snapshots, history-only upgrades, clean legacy state, previously cleared activation timestamps, and full-ports to standard conversion. It checks draft isolation, one-time history, and immediate toggles after migration. `TestRuntimeMigrationFailureAndRetry` checks validation/persistence failure recovery; `TestPrepareNeverActivatesUnknownDraft` prevents installation from applying an unconfirmed draft. Set `NGINX_TEST_BIN` to run against real Nginx in both build modes; use an isolated Linux environment with ports 80, 53 and 19989–19992 available.
