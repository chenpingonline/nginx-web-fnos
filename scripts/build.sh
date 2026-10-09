#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORE="$(python3 "$ROOT/scripts/resolve-core.py")"
VERSION="$(python3 "$CORE/scripts/version.py")"
DIST="${FNPROXY_DIST:-$ROOT/dist}"
MODE=standard
if [[ "${1:-}" == --mode ]]; then MODE="${2:?missing mode}"; shift 2; fi
case "$MODE" in
  standard|full-ports) ;;
  all) "$0" --mode standard "$@"; "$0" --mode full-ports "$@"; exit 0 ;;
  *) echo 'Use standard, full-ports or all' >&2; exit 1 ;;
esac
if (( $# == 0 )); then set -- x86; fi
for arch in "$@"; do case "$arch" in x86|x86_64|amd64|arm|arm64|aarch64) ;; *) exit 1 ;; esac; done
export LC_ALL=C
source "$CORE/scripts/package-lib.sh"
make --no-print-directory -C "$CORE" FRONTEND_MODE=fnos PERMISSION_MODE="$MODE" frontend-build
(cd "$CORE" && go test -tags "$([[ $MODE == full-ports ]] && echo full_ports || true)" ./...)
mkdir -p "$DIST"
for arch in "$@"; do
  case "$arch" in
    x86|x86_64|amd64) ARCH=x86; GOARCH=amd64; PLATFORM=x86; OUTPUT_ARCH=x86_64 ;;
    arm|arm64|aarch64) ARCH=arm64; GOARCH=arm64; PLATFORM=arm; OUTPUT_ARCH=arm64 ;;
  esac
  WORK="$ROOT/.build/$MODE/$ARCH"; STAGE="$WORK/fpk"; APP_STAGE="$WORK/app"
  rm -rf "$WORK"; mkdir -p "$STAGE"
  "$CORE/scripts/stage-app.sh" "$GOARCH" "$MODE" "$APP_STAGE"
  cp -a "$ROOT/packaging/fnos/app/ui" "$APP_STAGE/"
  NGINX_SHA256="$(sha256_file "$APP_STAGE/bin/nginx")"
  create_archive "$APP_STAGE" "$STAGE/app.tgz"
  if command -v md5sum >/dev/null 2>&1; then APP_MD5="$(md5sum "$STAGE/app.tgz" | awk '{print $1}')";
  else APP_MD5="$(md5 -q "$STAGE/app.tgz")"; fi
  cp -a "$ROOT/packaging/fnos/"{cmd,config,wizard} "$STAGE/"
  if [[ $MODE == full-ports ]]; then
    cp "$ROOT/packaging/fnos/variants/full-ports/privilege.sh" "$STAGE/cmd/privilege.sh"
    (cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -buildvcs=false -ldflags='-s -w' -o "$STAGE/cmd/repair-app-data" ./cmd/repair-app-data)
  fi
  python3 - "$ROOT" "$CORE" "$STAGE" "$VERSION" "$MODE" "$PLATFORM" "$APP_MD5" <<'PY'
import json,pathlib,re,subprocess,sys
root,core,stage=map(pathlib.Path,sys.argv[1:4])
version,mode,platform,checksum=sys.argv[4:]
p=stage/'config/privilege'; privilege=json.loads(p.read_text())
privilege['defaults']['run-as']='root' if mode=='full-ports' else 'package'
p.write_text(json.dumps(privilege,indent=2)+'\n')
(stage/'PERMISSION_MODE').write_text(mode+'\n')
text=(root/'packaging/fnos/manifest.template').read_text()
if text.count('@CORE_VERSION@')!=1: sys.exit('Manifest template needs exactly one @CORE_VERSION@')
text=text.replace('@CORE_VERSION@',version)
text=re.sub(r'^platform\s*=.*$',f'platform                   = {platform}',text,flags=re.M)
text=re.sub(r'^checksum\s*=.*\n?','',text,flags=re.M)
(stage/'manifest').write_text(text+f'checksum                   = {checksum}\n')
lock=json.loads((root/'core.lock').read_text())
def git(*args): return subprocess.check_output(['git',*args],cwd=core,text=True).strip()
provenance={'repository':lock['repository'],'commit':git('rev-parse','HEAD'),'version':version,'dirty':bool(git('status','--porcelain'))}
(stage/'CORE_SOURCE.json').write_text(json.dumps(provenance,indent=2)+'\n')
PY
  cp "$ROOT/packaging/fnos/"{ICON.PNG,ICON_256.PNG} "$STAGE/"
  cp "$CORE/third_party/nginx/$OUTPUT_ARCH/SOURCES.txt" "$STAGE/NGINX_SOURCES.txt"
  cp "$CORE/third_party/nginx/$OUTPUT_ARCH/SHA256SUMS.txt" "$STAGE/NGINX_SHA256SUMS.txt"
  printf '%s  nginx\n' "$NGINX_SHA256" > "$STAGE/NGINX_BINARY_SHA256SUMS.txt"
  chmod 755 "$STAGE/cmd/"*
  FPK_NAME="nginx-web-${VERSION}-${MODE}-${OUTPUT_ARCH}.fpk"
  create_archive "$STAGE" "$DIST/$FPK_NAME"
  "$ROOT/scripts/verify-fpk.sh" "$DIST/$FPK_NAME"
  printf '%s  %s\n' "$(sha256_file "$DIST/$FPK_NAME")" "$FPK_NAME" > "$DIST/${FPK_NAME}.sha256"
  echo "Created $DIST/$FPK_NAME"
done
