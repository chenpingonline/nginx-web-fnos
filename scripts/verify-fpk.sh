#!/usr/bin/env bash
set -euo pipefail
FPK="${1:?用法: verify-fpk.sh <file.fpk>}"; WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
tar -xzf "$FPK" -C "$WORK"
for f in app.tgz manifest cmd/main config/privilege config/resource ICON.PNG ICON_256.PNG; do [[ -e "$WORK/$f" ]] || { echo "FPK 缺少 $f" >&2; exit 1; }; done
EXPECTED_MD5="$(awk -F= '$1 ~ /^[[:space:]]*checksum[[:space:]]*$/ {gsub(/[[:space:]]/, "", $2); print $2}' "$WORK/manifest")"
if command -v md5sum >/dev/null 2>&1; then ACTUAL_MD5="$(md5sum "$WORK/app.tgz" | awk '{print $1}')"; else ACTUAL_MD5="$(md5 -q "$WORK/app.tgz")"; fi
[[ -n "$EXPECTED_MD5" && "$EXPECTED_MD5" == "$ACTUAL_MD5" ]] || { echo 'app.tgz MD5 不匹配' >&2; exit 1; }
PLATFORM="$(awk -F= '$1 ~ /^[[:space:]]*platform[[:space:]]*$/ {gsub(/[[:space:]]/, "", $2); print $2}' "$WORK/manifest")"
case "$PLATFORM" in
  x86) FILE_PATTERN='x86-64|x86_64' ;;
  arm) FILE_PATTERN='ARM aarch64|ARM64|aarch64' ;;
  *) echo "manifest platform 无效：$PLATFORM" >&2; exit 1 ;;
esac
[[ -f "$WORK/NGINX_BINARY_SHA256SUMS.txt" ]] || { echo 'FPK 缺少 NGINX_BINARY_SHA256SUMS.txt' >&2; exit 1; }
mkdir -p "$WORK/app"; tar -xzf "$WORK/app.tgz" -C "$WORK/app"
for f in bin/nginx-web-server bin/nginx etc/mime.types ui/config ui/images/icon_64.png ui/images/icon_256.png; do [[ -e "$WORK/app/$f" ]] || { echo "app.tgz 缺少 $f" >&2; exit 1; }; done
file "$WORK/app/bin/nginx-web-server" | grep -Eq "$FILE_PATTERN" || { echo '管理程序架构错误' >&2; exit 1; }
file "$WORK/app/bin/nginx" | grep -Eq "$FILE_PATTERN" || { echo 'Nginx 架构错误' >&2; exit 1; }
grep -aFq 'nginx-web 0.1.1' "$WORK/app/bin/nginx-web-server" || { echo '管理程序版本字符串不正确' >&2; exit 1; }
grep -aFq 'nginx version: nginx/1.30.4' "$WORK/app/bin/nginx" || { echo 'Nginx 版本不正确' >&2; exit 1; }
EXPECTED_SHA="$(awk 'NR == 1 {print $1}' "$WORK/NGINX_BINARY_SHA256SUMS.txt")"
if command -v sha256sum >/dev/null 2>&1; then ACTUAL_SHA="$(sha256sum "$WORK/app/bin/nginx" | awk '{print $1}')"; else ACTUAL_SHA="$(shasum -a 256 "$WORK/app/bin/nginx" | awk '{print $1}')"; fi
[[ -n "$EXPECTED_SHA" && "$ACTUAL_SHA" == "$EXPECTED_SHA" ]] || { echo 'Nginx 摘要不匹配' >&2; exit 1; }
file "$WORK/app/bin/nginx" | grep -Fq 'statically linked' || { echo 'Nginx 不是静态链接' >&2; exit 1; }
grep -aEq 'nginx-auth-jwt|nginx-keyval|echo-nginx-module|headers-more-nginx-module|set-misc-nginx-module' "$WORK/app/bin/nginx" && { echo 'Nginx 含非官方第三方模块' >&2; exit 1; }
python3 - "$WORK" <<'PY'
import json, pathlib, sys
root=pathlib.Path(sys.argv[1])
privilege=json.loads((root/'config/privilege').read_text())
json.loads((root/'config/resource').read_text())
json.loads((root/'app/ui/config').read_text())
if privilege.get('username') != 'nginx-web': raise SystemExit('运行用户名不正确')
if privilege.get('groupname') != 'nginx-web': raise SystemExit('运行组名不正确')
manifest=(root/'manifest').read_text()
for key in ('appname','version','display_name','platform','checksum'):
    if not any(line.split('=',1)[0].strip()==key for line in manifest.splitlines() if '=' in line): raise SystemExit(f'manifest 缺少 {key}')
values={line.split('=',1)[0].strip():line.split('=',1)[1].strip() for line in manifest.splitlines() if '=' in line}
if values.get('appname') != 'nginx-web': raise SystemExit('manifest appname 不正确')
if values.get('display_name') != 'nginx-web': raise SystemExit('manifest display_name 不正确')
PY
bash -n "$WORK/cmd/"*
echo "FPK 验证通过：$(basename "$FPK")"
