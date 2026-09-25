#!/usr/bin/env bash
# Linux root-only integration check; run in an isolated container, never against an installed app.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVER="${1:?usage: low-ports.sh <native Linux server binary> <repair helper>}"
REPAIR="${2:?native Linux repair-app-data binary}"
[[ $(id -u) == 0 ]]
[[ $(cat /proc/sys/net/ipv4/ip_unprivileged_port_start) == 1024 ]]
id nginx-web >/dev/null
TEST=$(mktemp -d /tmp/nginx-lowports.XXXXXX)
export TRIM_APPDEST="$TEST/app" TRIM_PKGETC="$TEST/etc" TRIM_PKGVAR="$TEST/var" TRIM_PKGTMP="$TEST/tmp" TRIM_PKGHOME="$TEST/home" TRIM_USERNAME=nginx-web
export FNPROXY_DEV_ALLOW=1
cleanup() {
  "$TEST/cmd/main" stop >/dev/null 2>&1 || true
  [[ ${KEEP_TEST_ROOT:-0} == 1 ]] || rm -rf "$TEST"
}
trap cleanup EXIT
mkdir -p "$TEST/cmd"
cp "$ROOT/packaging/fnos/cmd/"* "$TEST/cmd/"
cp "$ROOT/packaging/fnos/variants/full-ports/privilege.sh" "$TEST/cmd/privilege.sh"
cp "$REPAIR" "$TEST/cmd/repair-app-data"
mkdir -p "$TEST/app/bin" "$TEST/app/etc" "$TEST/etc" "$TEST/var" "$TEST/tmp" "$TEST/home"
case $(uname -m) in aarch64) arch=arm64 ;; x86_64) arch=x86_64 ;; esac
cp "$SERVER" "$TEST/app/bin/nginx-web-server"
cp "$ROOT/third_party/nginx/$arch/nginx" "$TEST/app/bin/nginx"
cp "$ROOT/third_party/nginx/mime.types" "$TEST/app/etc/mime.types"
chmod 755 "$TEST" "$TEST/app/bin/"*
# Prove low ports really require privilege in this environment.
if runuser -u nginx-web -- python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",80))' 2>/dev/null; then
  echo 'Unprivileged bind unexpectedly succeeded' >&2; exit 1
fi
printf '%s' '{"schema_version":1,"settings":{"default_http_port":80,"default_https_port":443,"revision_limit":20},"rules":[],"certificates":[],"dirty":true}' > "$TEST/var/fnproxy.json"
chown nginx-web:nginx-web "$TEST/var/fnproxy.json"
# Reject an unexpected root runtime user and a symlink in place of the binary.
if TRIM_USERNAME=root "$TEST/cmd/install_callback" 2>/dev/null; then
  echo 'Root runtime user unexpectedly accepted' >&2; exit 1
fi
mv "$TEST/app/bin/nginx" "$TEST/app/bin/nginx.real"
ln -s nginx.real "$TEST/app/bin/nginx"
if "$TEST/cmd/install_callback" 2>/dev/null; then
  echo 'Symlink capability target unexpectedly accepted' >&2; exit 1
fi
rm "$TEST/app/bin/nginx"
mv "$TEST/app/bin/nginx.real" "$TEST/app/bin/nginx"
"$TEST/cmd/install_callback"
[[ $(getcap "$TEST/app/bin/nginx") == "$TEST/app/bin/nginx cap_net_bind_service=ep" ]]
[[ -z $(getcap "$TEST/app/bin/nginx-web-server") ]]

"$TEST/cmd/main" start
"$TEST/cmd/main" status
[[ $(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:80/) == 404 ]]
backend=$(cat "$TEST/var/run/nginx-web-server.pid")
nginx=$(cat "$TEST/var/nginx/run/nginx.pid")
uid=$(id -u nginx-web)
python3 - "$backend" "$nginx" "$uid" <<'PY'
import pathlib,sys
for pid,cap in [(sys.argv[1],0),(sys.argv[2],1024)]:
 d=dict(line.split(':',1) for line in pathlib.Path('/proc/'+pid+'/status').read_text().splitlines() if ':' in line)
 assert all(x==sys.argv[3] for x in d['Uid'].split()),d['Uid']
 assert int(d['CapEff'],16)==cap,d['CapEff']
 print('PID',pid,'UID',d['Uid'].strip(),'CapEff',d['CapEff'].strip())
PY
# Re-execution for config testing/reload must retain the narrow capability.
runuser -u nginx-web -- "$TEST/app/bin/nginx-web-server" nginx-test >/dev/null
runuser -u nginx-web -- "$TEST/app/bin/nginx-web-server" nginx-reload >/dev/null
[[ $(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:80/) == 404 ]]
"$TEST/cmd/main" stop
setcap -r "$TEST/app/bin/nginx"
"$TEST/cmd/upgrade_callback"
[[ $(getcap "$TEST/app/bin/nginx") == "$TEST/app/bin/nginx cap_net_bind_service=ep" ]]
# Capability loss is also repaired on start, without running the service as root.
setcap -r "$TEST/app/bin/nginx"
"$TEST/cmd/main" start
[[ $(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:80/) == 404 ]]
runuser -u nginx-web -- python3 "$ROOT/tests/low-ports-api.py"
echo "Low-port lifecycle passed: $TEST"
if [[ ${KEEP_TEST_ROOT:-0} == 1 ]]; then trap - EXIT; fi
