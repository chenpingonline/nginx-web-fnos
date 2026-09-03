#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; VERSION="${VERSION:-0.1.5}"
"$ROOT/scripts/build.sh" x86
"$ROOT/scripts/build.sh" arm64
"$ROOT/tests/integration.sh"
"$ROOT/tests/fpk-lifecycle.sh" "$ROOT/dist/nginx-web-${VERSION}-x86.fpk"
(cd "$ROOT/dist" && sha256sum "nginx-web-${VERSION}-x86.fpk" "nginx-web-${VERSION}-arm64.fpk" > SHA256SUMS.txt)
echo "nginx-web release artifacts created in $ROOT/dist"
