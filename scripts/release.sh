#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(python3 "$ROOT/scripts/version.py")"
"$ROOT/scripts/build.sh" x86
"$ROOT/scripts/build.sh" arm64
"$ROOT/tests/integration.sh"
case "$(uname -m)" in
  aarch64|arm64) TEST_ARCH=arm64 ;;
  *) TEST_ARCH=x86 ;;
esac
"$ROOT/tests/fpk-lifecycle.sh" "$ROOT/dist/nginx-web-${VERSION}-${TEST_ARCH}.fpk"
(cd "$ROOT/dist"
 if command -v sha256sum >/dev/null 2>&1; then
   sha256sum "nginx-web-${VERSION}-x86.fpk" "nginx-web-${VERSION}-arm64.fpk" > SHA256SUMS.txt
 else
   shasum -a 256 "nginx-web-${VERSION}-x86.fpk" "nginx-web-${VERSION}-arm64.fpk" > SHA256SUMS.txt
 fi)
echo "nginx-web release artifacts created in $ROOT/dist"
