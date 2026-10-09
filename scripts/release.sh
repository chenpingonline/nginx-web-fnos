#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORE="$(python3 "$ROOT/scripts/resolve-core.py")"
python3 - "$ROOT/core.lock" "$CORE" <<'PYRELEASE'
import json,pathlib,subprocess,sys
lock=json.loads(pathlib.Path(sys.argv[1]).read_text()); core=pathlib.Path(sys.argv[2])
head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=core,text=True).strip()
status=subprocess.check_output(['git','status','--porcelain'],cwd=core,text=True).strip()
if head!=lock['commit'] or status: sys.exit('Release requires the clean source commit pinned in core.lock')
PYRELEASE
VERSION="$(python3 "$ROOT/scripts/version.py")"
"$ROOT/scripts/build.sh" --mode all x86 arm64
"$ROOT/tests/integration.sh"
case "$(uname -m)" in
  aarch64|arm64) TEST_ARCH=arm64 ;;
  *) TEST_ARCH=x86_64 ;;
esac
for mode in standard full-ports; do
  "$ROOT/tests/fpk-lifecycle.sh" "$ROOT/dist/nginx-web-${VERSION}-${mode}-${TEST_ARCH}.fpk"
done
(cd "$ROOT/dist"
 if command -v sha256sum >/dev/null 2>&1; then
   sha256sum nginx-web-${VERSION}-{standard,full-ports}-{x86_64,arm64}.fpk > SHA256SUMS.txt
 else
   shasum -a 256 nginx-web-${VERSION}-{standard,full-ports}-{x86_64,arm64}.fpk > SHA256SUMS.txt
 fi)
echo "nginx-web release artifacts created in $ROOT/dist"
