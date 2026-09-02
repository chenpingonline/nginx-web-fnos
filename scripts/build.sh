#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCH="${1:-x86}"
VERSION="${VERSION:-0.1.0}"
DIST="$ROOT/dist"
case "$ARCH" in
  x86|x86_64|amd64) ARCH="x86"; GOARCH="amd64"; PLATFORM="x86"; FILE_PATTERN='x86-64|x86_64'; OUTPUT_ARCH="x86" ;;
  arm|arm64|aarch64) ARCH="arm64"; GOARCH="arm64"; PLATFORM="arm"; FILE_PATTERN='ARM aarch64|ARM64|aarch64'; OUTPUT_ARCH="arm64" ;;
  *) echo "不支持的架构：$ARCH（应为 x86 或 arm64）" >&2; exit 1 ;;
esac
WORK="$ROOT/.build/$ARCH"; STAGE="$WORK/fpk"; APP_STAGE="$WORK/app"; FPK_NAME="nginx-web-${VERSION}-${OUTPUT_ARCH}.fpk"
for cmd in go tar file python3; do command -v "$cmd" >/dev/null 2>&1 || { echo "缺少 $cmd" >&2; exit 1; }; done
rm -rf "$WORK"; mkdir -p "$DIST" "$STAGE" "$APP_STAGE/bin"

create_archive() {
  local source_dir="$1" output_file="$2"
  if tar --version 2>/dev/null | grep -q 'GNU tar'; then
    tar --sort=name --mtime='UTC 2026-09-02 00:00:00' --owner=0 --group=0 --numeric-owner -czf "$output_file" -C "$source_dir" .
    return
  fi
  find "$source_dir" -exec touch -h -t 202609020000.00 {} +
  (cd "$source_dir" && find . -print | LC_ALL=C sort | tar --no-recursion --uid 0 --gid 0 --uname root --gname root --no-acls --no-fflags --no-xattrs --no-mac-metadata -czf "$output_file" -T -)
}

echo '[1/7] 运行 Go 测试'; (cd "$ROOT" && go test ./...)
echo "[2/7] 构建 Linux $GOARCH 管理服务"
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -buildvcs=false -ldflags='-s -w' -o "$APP_STAGE/bin/fnproxy-server" "$ROOT"
chmod 755 "$APP_STAGE/bin/fnproxy-server"
echo '[3/7] 准备并校验 Nginx 1.30.4'
"$ROOT/scripts/fetch-nginx.sh" "$ARCH" "$APP_STAGE/bin/nginx" >/dev/null
file "$APP_STAGE/bin/fnproxy-server" | grep -Eq "$FILE_PATTERN" || { echo '管理服务架构不正确' >&2; exit 1; }
file "$APP_STAGE/bin/nginx" | grep -Eq "$FILE_PATTERN" || { echo 'Nginx 架构不正确' >&2; exit 1; }
grep -aFq 'nginx version: nginx/1.30.4' "$APP_STAGE/bin/nginx" || { echo '无法确认 Nginx 1.30.4 版本字符串' >&2; exit 1; }
echo '[4/7] 组装 app.tgz'
cp -a "$ROOT/fnos/app/etc" "$ROOT/fnos/app/ui" "$APP_STAGE/"
create_archive "$APP_STAGE" "$STAGE/app.tgz"
if command -v md5sum >/dev/null 2>&1; then APP_MD5="$(md5sum "$STAGE/app.tgz" | awk '{print $1}')"; else APP_MD5="$(md5 -q "$STAGE/app.tgz")"; fi
echo '[5/7] 组装 FPK 元数据'
cp -a "$ROOT/fnos/cmd" "$ROOT/fnos/config" "$ROOT/fnos/wizard" "$STAGE/"
cp "$ROOT/fnos/ICON.PNG" "$ROOT/fnos/ICON_256.PNG" "$STAGE/"
cp "$ROOT/LICENSE" "$ROOT/NGINX_LICENSE" "$ROOT/NOTICE" "$ROOT/THIRD_PARTY_LICENSES.md" "$STAGE/"
if [[ "$ARCH" == arm64 ]]; then cp "$ROOT/third_party/nginx/arm64/SOURCES.txt" "$STAGE/NGINX_ARM64_SOURCES.txt"; cp "$ROOT/third_party/nginx/arm64/SHA256SUMS.txt" "$STAGE/NGINX_ARM64_SHA256SUMS.txt";
else cp "$ROOT/third_party/nginx/x86_64/SHA256SUMS.txt" "$STAGE/NGINX_X86_64_SHA256SUMS.txt"; fi
sed -E "s/^platform[[:space:]]*=.*/platform                   = ${PLATFORM}/" "$ROOT/fnos/manifest" | grep -v '^[[:space:]]*checksum[[:space:]]*=' > "$STAGE/manifest"
printf 'checksum                   = %s\n' "$APP_MD5" >> "$STAGE/manifest"; chmod 755 "$STAGE/cmd/"*
echo "[6/7] 创建 $FPK_NAME"
create_archive "$STAGE" "$DIST/$FPK_NAME"
echo '[7/7] 验证 FPK'; "$ROOT/scripts/verify-fpk.sh" "$DIST/$FPK_NAME"
if command -v sha256sum >/dev/null 2>&1; then sha256sum "$DIST/$FPK_NAME" > "$DIST/${FPK_NAME}.sha256"; else shasum -a 256 "$DIST/$FPK_NAME" > "$DIST/${FPK_NAME}.sha256"; fi
echo "完成：$DIST/$FPK_NAME"
