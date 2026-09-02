#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCH="${1:?用法: fetch-nginx.sh <x86|arm64> [目标路径]}"
DEST="${2:-}"
VERSION="1.30.4"
CACHE="$ROOT/.cache/nginx/$ARCH"
mkdir -p "$CACHE"

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}';
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}
verify_sha256() {
  local file="$1" expected="$2" actual
  actual="$(sha256_file "$file")"
  [[ "$actual" == "$expected" ]] || {
    echo "SHA-256 校验失败：$file" >&2
    echo "期望：$expected" >&2
    echo "实际：$actual" >&2
    exit 1
  }
}
download() {
  local url="$1" output="$2"
  command -v curl >/dev/null 2>&1 || { echo "缺少 curl" >&2; exit 1; }
  curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 "$url" -o "$output.tmp"
  mv "$output.tmp" "$output"
}

case "$ARCH" in
  x86|x86_64|amd64)
    ARCH="x86"; CACHE="$ROOT/.cache/nginx/$ARCH"; mkdir -p "$CACHE"
    BIN="$CACHE/nginx"
    BIN_SHA="90b9e538d29a481f071a53877cb9f957b0a5fd38416fea84d5c89eba78e60cb4"
    ARCHIVE="$CACHE/nginx-${VERSION}-acme-debian12.tar.gz"
    ARCHIVE_SHA="802e5f416fd2377d9770dd201585b67fc6d8c8b538a15c124dc3402f3042d33d"
    URL="https://github.com/hzbd/nginx-acme-build/releases/download/nginx-${VERSION}/nginx-${VERSION}-acme-debian12.tar.gz"
    if [[ -n "${NGINX_BINARY:-}" ]]; then
      cp "$NGINX_BINARY" "$BIN"
    elif [[ ! -f "$BIN" ]]; then
      [[ -f "$ARCHIVE" ]] || download "$URL" "$ARCHIVE"
      verify_sha256 "$ARCHIVE" "$ARCHIVE_SHA"
      TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
      tar -xzf "$ARCHIVE" -C "$TMP" nginx_acme/sbin/nginx
      cp "$TMP/nginx_acme/sbin/nginx" "$BIN"
      command -v strip >/dev/null 2>&1 || { echo "缺少 strip（请安装 binutils）" >&2; exit 1; }
      strip "$BIN"
    fi
    verify_sha256 "$BIN" "$BIN_SHA"
    ;;
  arm|arm64|aarch64)
    ARCH="arm64"; CACHE="$ROOT/.cache/nginx/$ARCH"; mkdir -p "$CACHE"
    BIN="$CACHE/nginx-official"
    if [[ -n "${NGINX_BINARY:-}" ]]; then cp "$NGINX_BINARY" "$BIN";
    elif [[ ! -f "$BIN" ]]; then "$ROOT/scripts/build-nginx.sh" arm64 "$BIN" >/dev/null; fi
    ;;
  *) echo "不支持的架构：$ARCH（应为 x86 或 arm64）" >&2; exit 1 ;;
esac

chmod 755 "$BIN"
if [[ -n "$DEST" ]]; then mkdir -p "$(dirname "$DEST")"; cp "$BIN" "$DEST"; chmod 755 "$DEST"; echo "$DEST";
else echo "$BIN"; fi
