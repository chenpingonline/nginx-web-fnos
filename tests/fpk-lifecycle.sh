#!/usr/bin/env bash
set -euo pipefail

FPK="${1:?用法: fpk-lifecycle.sh <file.fpk>}"
TEST="$(mktemp -d /tmp/fnproxy-fpk.XXXXXX)"
cleanup() {
  if [[ -x "$TEST/pkg/cmd/main" ]]; then
    "$TEST/pkg/cmd/main" stop >/dev/null 2>&1 || true
  fi
  rm -rf "$TEST"
}
trap cleanup EXIT

mkdir -p "$TEST/pkg" "$TEST/app" "$TEST/etc" "$TEST/var" "$TEST/tmp"
tar -xzf "$FPK" -C "$TEST/pkg"
tar -xzf "$TEST/pkg/app.tgz" -C "$TEST/app"
[[ -x "$TEST/app/bin/nginx-web-server" ]]
[[ ! -e "$TEST/app/bin/fnproxy-server" ]]
PORT="$(python3 - <<'PY'
import socket
s=socket.socket(); s.bind(('127.0.0.1',0)); print(s.getsockname()[1]); s.close()
PY
)"
cat > "$TEST/var/fnproxy.json" <<JSON
{
  "schema_version": 1,
  "settings": {"default_http_port": $PORT, "default_https_port": 19443, "revision_limit": 20},
  "rules": [],
  "certificates": [],
  "dirty": true,
  "updated_at": "2026-09-02T00:00:00Z"
}
JSON

export TRIM_APPDEST="$TEST/app"
export TRIM_PKGETC="$TEST/etc"
export TRIM_PKGVAR="$TEST/var"
export TRIM_PKGTMP="$TEST/tmp"
export TRIM_TEMP_LOGFILE="$TEST/fnos-error.log"

"$TEST/pkg/cmd/install_callback"
"$TEST/pkg/cmd/main" start
"$TEST/pkg/cmd/main" status
[[ -s "$TEST/var/run/nginx-web-server.pid" ]]
[[ -f "$TEST/var/logs/nginx-web-server.log" ]]
[[ -S "$TEST/app/app.sock" ]]
grep -q 'nginx-web' < <(curl -fsS --unix-socket "$TEST/app/app.sock" http://localhost/)
curl -fsS --unix-socket "$TEST/app/app.sock" -H 'X-Trim-Isadmin: true' http://localhost/api/overview | grep -q 'nginx_version'
[[ "$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/")" == "404" ]]
"$TEST/pkg/cmd/main" stop
set +e
"$TEST/pkg/cmd/main" status
status=$?
set -e
[[ "$status" == "3" ]]
echo "FPK lifecycle passed on port $PORT"
