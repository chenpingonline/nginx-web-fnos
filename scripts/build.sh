#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(python3 "$ROOT/scripts/version.py")"
DIST="${FNPROXY_DIST:-$ROOT/dist}"
MODE="standard"
if [[ "${1:-}" == --mode ]]; then MODE="${2:?缺少模式}"; shift 2; fi
case "$MODE" in
  standard) GO_TAGS="" ;;
  full-ports) GO_TAGS="full_ports" ;;
  all)
    "$0" --mode standard "$@"
    "$0" --mode full-ports "$@"
    exit 0 ;;
  *) echo '模式必须是 standard、full-ports 或 all' >&2; exit 1 ;;
esac
export FNPROXY_PERMISSION_MODE="$MODE"
export LC_ALL=C

if (( $# == 0 )); then ARCHES=(x86); else ARCHES=("$@"); fi
for arch in "${ARCHES[@]}"; do
  case "$arch" in
    x86|x86_64|amd64|arm|arm64|aarch64) ;;
    *) echo "不支持的架构：$arch（应为 x86 或 arm64）" >&2; exit 1 ;;
  esac
done
for cmd in go npm tar file python3; do command -v "$cmd" >/dev/null 2>&1 || { echo "缺少 $cmd" >&2; exit 1; }; done

create_archive() {
  local source_dir="$1" output_file="$2"
  if tar --version 2>/dev/null | grep -q 'GNU tar'; then
    tar --sort=name --mtime='UTC 2026-09-02 00:00:00' --owner=0 --group=0 --numeric-owner -czf "$output_file" -C "$source_dir" .
    return
  fi
  find "$source_dir" -exec touch -h -t 202609020000.00 {} +
  (cd "$source_dir" && find . -print | LC_ALL=C sort | tar --no-recursion --uid 0 --gid 0 --uname root --gname root --no-acls --no-fflags --no-xattrs --no-mac-metadata -czf "$output_file" -T -)
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}';
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

echo '[共享 1/2] 构建 Vue 管理页面'
if [[ ! -d "$ROOT/web/node_modules" ]]; then npm --prefix "$ROOT/web" ci; fi
npm --prefix "$ROOT/web" run build
echo '[共享 2/2] 运行 Go 测试'; (cd "$ROOT" && go test -tags "$GO_TAGS" ./...)

build_arch() {
  local ARCH="$1" GOARCH PLATFORM FILE_PATTERN OUTPUT_ARCH
  case "$ARCH" in
    x86|x86_64|amd64) ARCH="x86"; GOARCH="amd64"; PLATFORM="x86"; FILE_PATTERN='x86-64|x86_64'; OUTPUT_ARCH="x86_64" ;;
    arm|arm64|aarch64) ARCH="arm64"; GOARCH="arm64"; PLATFORM="arm"; FILE_PATTERN='ARM aarch64|ARM64|aarch64'; OUTPUT_ARCH="arm64" ;;
    *) echo "不支持的架构：$ARCH（应为 x86 或 arm64）" >&2; exit 1 ;;
  esac
  local WORK="$ROOT/.build/$MODE/$ARCH" STAGE="$ROOT/.build/$MODE/$ARCH/fpk" APP_STAGE="$ROOT/.build/$MODE/$ARCH/app"
  local FPK_NAME="nginx-web-${VERSION}-${MODE}-${OUTPUT_ARCH}.fpk" NGINX_SHA256 APP_MD5
  rm -rf "$WORK"; mkdir -p "$DIST" "$STAGE" "$APP_STAGE/bin"

  echo "[$ARCH 1/6] 构建 Linux $GOARCH 管理服务"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -tags "$GO_TAGS" -trimpath -buildvcs=false -ldflags='-s -w' -o "$APP_STAGE/bin/nginx-web-server" ./cmd/nginx-web)
  chmod 755 "$APP_STAGE/bin/nginx-web-server"
  echo "[$ARCH 2/6] 准备并校验 Nginx 1.30.4"
  "$ROOT/scripts/fetch-nginx.sh" "$ARCH" "$APP_STAGE/bin/nginx" >/dev/null
  file "$APP_STAGE/bin/nginx-web-server" | grep -Eq "$FILE_PATTERN" || { echo '管理服务架构不正确' >&2; exit 1; }
  file "$APP_STAGE/bin/nginx" | grep -Eq "$FILE_PATTERN" || { echo 'Nginx 架构不正确' >&2; exit 1; }
  file "$APP_STAGE/bin/nginx" | grep -Fq 'statically linked' || { echo 'Nginx 必须是静态链接二进制' >&2; exit 1; }
  grep -aFq 'nginx version: nginx/1.30.4' "$APP_STAGE/bin/nginx" || { echo '无法确认 Nginx 1.30.4 版本字符串' >&2; exit 1; }
  NGINX_SHA256="$(sha256_file "$APP_STAGE/bin/nginx")"
  echo "[$ARCH 3/6] 组装 app.tgz"
  mkdir -p "$APP_STAGE/etc" "$APP_STAGE/licenses"
  cp -a "$ROOT/packaging/fnos/app/ui" "$APP_STAGE/"
  cp "$ROOT/third_party/nginx/mime.types" "$APP_STAGE/etc/mime.types"
  cp "$ROOT/LICENSE" "$ROOT/NGINX_LICENSE" "$ROOT/NOTICE" "$ROOT/THIRD_PARTY_LICENSES.md" "$APP_STAGE/licenses/"
  create_archive "$APP_STAGE" "$STAGE/app.tgz"
  if command -v md5sum >/dev/null 2>&1; then APP_MD5="$(md5sum "$STAGE/app.tgz" | awk '{print $1}')"; else APP_MD5="$(md5 -q "$STAGE/app.tgz")"; fi
  echo "[$ARCH 4/6] 组装 FPK 元数据"
  cp -a "$ROOT/packaging/fnos/cmd" "$ROOT/packaging/fnos/config" "$ROOT/packaging/fnos/wizard" "$STAGE/"
  if [[ "$MODE" == full-ports ]]; then
    cp "$ROOT/packaging/fnos/variants/full-ports/privilege.sh" "$STAGE/cmd/privilege.sh"
    (cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -buildvcs=false -ldflags='-s -w' -o "$STAGE/cmd/repair-app-data" ./cmd/repair-app-data)
  fi
  python3 - "$STAGE" "$MODE" <<'PYMODE'
import json, pathlib, sys
stage, mode = pathlib.Path(sys.argv[1]), sys.argv[2]
p = stage / 'config/privilege'
config = json.loads(p.read_text())
config['defaults']['run-as'] = 'root' if mode == 'full-ports' else 'package'
p.write_text(json.dumps(config, indent=2) + '\n')
(stage / 'PERMISSION_MODE').write_text(mode + '\n')
PYMODE
  cp "$ROOT/packaging/fnos/ICON.PNG" "$ROOT/packaging/fnos/ICON_256.PNG" "$STAGE/"
  if [[ "$ARCH" == arm64 ]]; then cp "$ROOT/third_party/nginx/arm64/SOURCES.txt" "$STAGE/NGINX_ARM64_SOURCES.txt"; cp "$ROOT/third_party/nginx/arm64/SHA256SUMS.txt" "$STAGE/NGINX_ARM64_SHA256SUMS.txt";
  else cp "$ROOT/third_party/nginx/x86_64/SOURCES.txt" "$STAGE/NGINX_X86_64_SOURCES.txt"; cp "$ROOT/third_party/nginx/x86_64/SHA256SUMS.txt" "$STAGE/NGINX_X86_64_SHA256SUMS.txt"; fi
  printf '%s  nginx\n' "$NGINX_SHA256" > "$STAGE/NGINX_BINARY_SHA256SUMS.txt"
  sed -E "s/^platform[[:space:]]*=.*/platform                   = ${PLATFORM}/" "$ROOT/packaging/fnos/manifest" | grep -v '^[[:space:]]*checksum[[:space:]]*=' > "$STAGE/manifest"
  printf 'checksum                   = %s\n' "$APP_MD5" >> "$STAGE/manifest"; chmod 755 "$STAGE/cmd/"*
  echo "[$ARCH 5/6] 创建 $FPK_NAME"
  create_archive "$STAGE" "$DIST/$FPK_NAME"
  echo "[$ARCH 6/6] 验证 FPK"; "$ROOT/scripts/verify-fpk.sh" "$DIST/$FPK_NAME"
  if command -v sha256sum >/dev/null 2>&1; then (cd "$DIST" && sha256sum "$FPK_NAME" > "${FPK_NAME}.sha256"); else (cd "$DIST" && shasum -a 256 "$FPK_NAME" > "${FPK_NAME}.sha256"); fi
  echo "完成：$DIST/$FPK_NAME"
}

for arch in "${ARCHES[@]}"; do build_arch "$arch"; done
