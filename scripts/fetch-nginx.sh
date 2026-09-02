#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCH="${1:?用法: fetch-nginx.sh <x86|arm64> [目标路径]}"
DEST="${2:-}"
VERSION="1.30.4"

case "$ARCH" in
  x86|x86_64|amd64)
    ARCH="amd64"
    ARCH_LABEL="AMD64"
    OUTPUT_NAME="nginx-${VERSION}-x86_64-linux"
    BIN="$ROOT/third_party/nginx/x86_64/nginx"
    ;;
  arm|arm64|aarch64)
    ARCH="arm64"
    ARCH_LABEL="ARM64"
    OUTPUT_NAME="nginx-${VERSION}-aarch64-linux"
    BIN="$ROOT/third_party/nginx/arm64/nginx"
    ;;
  *) echo "不支持的架构：$ARCH（应为 x86 或 arm64）" >&2; exit 1 ;;
esac

[[ -f "$BIN" ]] || {
  echo "缺少 $ARCH_LABEL Nginx 二进制：$BIN" >&2
  echo "请先在对应架构的 fnOS 上执行 scripts/build-nginx-on-fnos.sh，" >&2
  echo "再将生成的 $OUTPUT_NAME 复制到上述路径并命名为 nginx。" >&2
  exit 1
}

chmod 755 "$BIN"
if [[ -n "$DEST" ]]; then mkdir -p "$(dirname "$DEST")"; cp "$BIN" "$DEST"; chmod 755 "$DEST"; echo "$DEST";
else echo "$BIN"; fi
