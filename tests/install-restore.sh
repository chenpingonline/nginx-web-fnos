#!/usr/bin/env bash
# Run as an ordinary user; this exercises real filesystem access failures.
set -euo pipefail
export LC_ALL=C
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[[ $(id -u) != 0 ]] || { echo 'Run this test as a non-root user' >&2; exit 1; }
WORK=$(mktemp -d)
trap 'chmod -R u+rwX "$WORK"; rm -rf "$WORK"' EXIT
export TRIM_APPDEST="$WORK/app" TRIM_PKGVAR="$WORK/var" TRIM_PKGETC="$WORK/etc" TRIM_PKGHOME="$WORK/home"
export TRIM_TEMP_LOGFILE="$WORK/error.txt"
mkdir -p "$TRIM_APPDEST/bin" "$TRIM_PKGVAR/.uninstall-preserved/etc" "$TRIM_PKGETC" "$TRIM_PKGHOME"
printf '#!/bin/bash\n[ "${INIT_STATUS:-0}" = 0 ] || echo "initialization-test-detail" >&2\nexit "${INIT_STATUS:-0}"\n' > "$TRIM_APPDEST/bin/nginx-web-server"
chmod +x "$TRIM_APPDEST/bin/nginx-web-server"
printf 'private-content-must-not-be-logged\n' > "$TRIM_PKGVAR/.uninstall-preserved/etc/config"
chmod 600 "$TRIM_PKGVAR/.uninstall-preserved/etc/config"
run_install() { bash "$ROOT/packaging/fnos/cmd/install_callback" > "$WORK/output" 2>&1; }
expect_failure() {
  if run_install; then echo 'Expected restoration failure' >&2; exit 1; fi
  [[ -f "$TRIM_PKGVAR/.uninstall-preserved/etc/config" ]]
}
# Real copy failure includes source, destination, status and original stderr.
chmod 500 "$TRIM_PKGETC"
expect_failure
grep -q '退出码' "$TRIM_TEMP_LOGFILE"
grep -q 'Permission denied' "$TRIM_TEMP_LOGFILE"
grep -Fq "$TRIM_PKGETC" "$TRIM_TEMP_LOGFILE"
grep -q '执行用户=' "$TRIM_PKGVAR/logs/install.log"
! grep -q 'private-content-must-not-be-logged' "$TRIM_PKGVAR/logs/install.log"
chmod 700 "$TRIM_PKGETC"
# An inaccessible backup must never be mistaken for an empty backup and deleted.
chmod 000 "$TRIM_PKGVAR/.uninstall-preserved"
if run_install; then echo 'Unreadable backup accepted' >&2; exit 1; fi
[[ -d "$TRIM_PKGVAR/.uninstall-preserved" ]]
grep -q '保留目录不是' "$TRIM_TEMP_LOGFILE"
chmod 700 "$TRIM_PKGVAR/.uninstall-preserved"
# Initialization failure also keeps the backup available for a retry.
export INIT_STATUS=1
expect_failure
grep -q '初始化失败' "$TRIM_TEMP_LOGFILE"
grep -q 'initialization-test-detail' "$TRIM_TEMP_LOGFILE"
unset INIT_STATUS
run_install
[[ ! -e "$TRIM_PKGVAR/.uninstall-preserved" ]]
grep -q 'private-content-must-not-be-logged' "$TRIM_PKGETC/config"
grep -q '安装回调完成' "$TRIM_PKGVAR/logs/install.log"
# Log creation failure falls back without blocking restoration.
mkdir -p "$TRIM_PKGVAR/.uninstall-preserved/etc"
printf retained > "$TRIM_PKGVAR/.uninstall-preserved/etc/config"
mv "$TRIM_PKGVAR/logs" "$WORK/previous-logs"
printf blocked > "$TRIM_PKGVAR/logs"
export TRIM_PKGTMP="$WORK"
run_install
[[ ! -e "$TRIM_PKGVAR/.uninstall-preserved" ]]
grep -q '备用安装日志' "$WORK/output"
grep -q retained "$TRIM_PKGETC/config"
# Failed backup creation must not destroy the previous good copy.
rm "$TRIM_PKGVAR/logs"
mkdir -p "$TRIM_PKGVAR/.uninstall-preserved/etc"
printf previous > "$TRIM_PKGVAR/.uninstall-preserved/etc/config"
chmod 000 "$TRIM_PKGETC/config"
if bash "$ROOT/packaging/fnos/cmd/uninstall_init" > "$WORK/output" 2>&1; then
  echo 'Unreadable configuration accepted' >&2; exit 1
fi
grep -q previous "$TRIM_PKGVAR/.uninstall-preserved/etc/config"
chmod 600 "$TRIM_PKGETC/config"
bash "$ROOT/packaging/fnos/cmd/uninstall_init"
grep -q retained "$TRIM_PKGVAR/.uninstall-preserved/etc/config"
[[ ! -e "$TRIM_PKGVAR/.uninstall-preserved.previous" ]]
# Installation recovers a backup replacement interrupted between renames.
mv "$TRIM_PKGVAR/.uninstall-preserved" "$TRIM_PKGVAR/.uninstall-preserved.previous"
run_install
grep -q retained "$TRIM_PKGETC/config"
[[ ! -e "$TRIM_PKGVAR/.uninstall-preserved.previous" ]]
[[ ! -e "$TRIM_PKGVAR/.uninstall-preserved" ]]
echo 'Install restore diagnostics passed'
