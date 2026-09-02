#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST="${TEST_ROOT:-$(mktemp -d /tmp/fnproxy-integration.XXXXXX)}"
KEEP_TEST_ROOT="${KEEP_TEST_ROOT:-0}"

read -r DEFAULT_PORT HTTP_PORT HTTPS_PORT UPSTREAM_PORT < <(python3 - <<'PY'
import socket
ports=[]
for _ in range(4):
    s=socket.socket()
    s.bind(('127.0.0.1', 0))
    ports.append(s.getsockname()[1])
    s.close()
print(*ports)
PY
)

mkdir -p "$TEST/app/bin" "$TEST/app/etc" "$TEST/etc" "$TEST/var" "$TEST/tmp" "$TEST/upstream"
(cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags='-s -w' -o "$TEST/app/bin/nginx-web-server" ./cmd/nginx-web)
"$ROOT/scripts/fetch-nginx.sh" x86 "$TEST/app/bin/nginx" >/dev/null
cp "$ROOT/third_party/nginx/mime.types" "$TEST/app/etc/mime.types"
chmod 755 "$TEST/app/bin/"*

export FNPROXY_APPDEST="$TEST/app"
export FNPROXY_ETC="$TEST/etc"
export FNPROXY_VAR="$TEST/var"
export FNPROXY_TMP="$TEST/tmp"
export FNPROXY_SOCKET="$TEST/app/app.sock"
export FNPROXY_DEV_ALLOW=1

cleanup() {
  "$TEST/app/bin/nginx-web-server" nginx-stop >/dev/null 2>&1 || true
  [[ -f "$TEST/backend.pid" ]] && kill "$(cat "$TEST/backend.pid")" 2>/dev/null || true
  [[ -f "$TEST/upstream.pid" ]] && kill "$(cat "$TEST/upstream.pid")" 2>/dev/null || true
  if [[ "$KEEP_TEST_ROOT" != "1" ]]; then rm -rf "$TEST"; else echo "保留测试目录：$TEST"; fi
}
trap cleanup EXIT

cat > "$TEST/var/fnproxy.json" <<JSON
{
  "schema_version": 1,
  "settings": {"default_http_port": $DEFAULT_PORT, "default_https_port": $HTTPS_PORT, "revision_limit": 20},
  "rules": [],
  "certificates": [],
  "dirty": true,
  "updated_at": "2026-09-02T00:00:00Z"
}
JSON

"$TEST/app/bin/nginx-web-server" init > "$TEST/init.json"
"$TEST/app/bin/nginx-web-server" serve > "$TEST/var/logs/nginx-web-server.log" 2>&1 & echo $! > "$TEST/backend.pid"
for _ in $(seq 1 80); do [[ -S "$TEST/app/app.sock" ]] && break; sleep .1; done
[[ -S "$TEST/app/app.sock" ]]

curl -fsS --unix-socket "$TEST/app/app.sock" http://localhost/api/overview > "$TEST/overview.json"
"$TEST/app/bin/nginx-web-server" nginx-start > "$TEST/start.json"
[[ "$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$DEFAULT_PORT/")" == "404" ]]

echo 'FNPROXY_INTEGRATION_OK' > "$TEST/upstream/index.html"
python3 -m http.server "$UPSTREAM_PORT" --bind 127.0.0.1 --directory "$TEST/upstream" > "$TEST/upstream.log" 2>&1 & echo $! > "$TEST/upstream.pid"
for _ in $(seq 1 50); do curl -fsS "http://127.0.0.1:$UPSTREAM_PORT/" >/dev/null 2>&1 && break; sleep .1; done

openssl req -x509 -newkey rsa:2048 -nodes -days 3 -subj '/CN=secure.test' -addext 'subjectAltName=DNS:secure.test' \
  -keyout "$TEST/key.pem" -out "$TEST/cert.pem" >/dev/null 2>&1
python3 - "$TEST" <<'PY'
import json, pathlib, sys
root=pathlib.Path(sys.argv[1])
payload={"name":"Integration certificate","certificate":(root/'cert.pem').read_text(),"private_key":(root/'key.pem').read_text()}
(root/'cert-request.json').write_text(json.dumps(payload))
PY
curl -fsS --unix-socket "$TEST/app/app.sock" -H 'Content-Type: application/json' -H 'X-FnProxy-Request: 1' \
  --data-binary @"$TEST/cert-request.json" http://localhost/api/certificates > "$TEST/certificate.json"
CERT_ID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["id"])' "$TEST/certificate.json")"

cat > "$TEST/http-rule.json" <<JSON
{
  "name":"HTTP integration","enabled":true,"listen_port":$HTTP_PORT,"domains":["proxy.test"],
  "tls":false,"http2":false,"upstream_scheme":"http","upstream_host":"127.0.0.1","upstream_port":$UPSTREAM_PORT,
  "preserve_host":true,"websocket":true,"streaming":true,"verify_upstream_tls":false,
  "connect_timeout_seconds":10,"read_timeout_seconds":60,"send_timeout_seconds":60,"client_max_body_mb":0
}
JSON
cat > "$TEST/https-rule.json" <<JSON
{
  "name":"HTTPS integration","enabled":true,"listen_port":$HTTPS_PORT,"domains":["secure.test"],
  "tls":true,"http2":true,"certificate_id":"$CERT_ID","upstream_scheme":"http","upstream_host":"127.0.0.1","upstream_port":$UPSTREAM_PORT,
  "preserve_host":true,"websocket":true,"streaming":true,"verify_upstream_tls":false,
  "connect_timeout_seconds":10,"read_timeout_seconds":60,"send_timeout_seconds":60,"client_max_body_mb":0
}
JSON
for rule in "$TEST/http-rule.json" "$TEST/https-rule.json"; do
  curl -fsS --unix-socket "$TEST/app/app.sock" -H 'Content-Type: application/json' -H 'X-FnProxy-Request: 1' \
    --data-binary @"$rule" http://localhost/api/rules >/dev/null
done
curl -fsS --unix-socket "$TEST/app/app.sock" -H 'Content-Type: application/json' -H 'X-FnProxy-Request: 1' \
  --data '{"summary":"integration test"}' http://localhost/api/apply > "$TEST/apply.json"

[[ "$(curl -fsS -H 'Host: proxy.test' "http://127.0.0.1:$HTTP_PORT/")" == "FNPROXY_INTEGRATION_OK" ]]
[[ "$(curl -kfsS --resolve "secure.test:$HTTPS_PORT:127.0.0.1" "https://secure.test:$HTTPS_PORT/")" == "FNPROXY_INTEGRATION_OK" ]]
"$TEST/app/bin/nginx-web-server" nginx-test > "$TEST/nginx-test.json"
curl -fsS --unix-socket "$TEST/app/app.sock" http://localhost/api/config > "$TEST/config.json"
curl -fsS --unix-socket "$TEST/app/app.sock" http://localhost/api/revisions > "$TEST/revisions.json"
python3 - "$TEST" <<'PY'
import json, pathlib, sys
root=pathlib.Path(sys.argv[1])
assert len(json.loads((root/'revisions.json').read_text())) == 1
config=json.loads((root/'config.json').read_text())
joined=config['master']+'\n'+'\n'.join(config['files'].values())
assert 'proxy.test' in joined and 'secure.test' in joined and 'http2 on;' in joined
PY

printf 'Integration passed: HTTP=%s HTTPS=%s upstream=%s\n' "$HTTP_PORT" "$HTTPS_PORT" "$UPSTREAM_PORT"
