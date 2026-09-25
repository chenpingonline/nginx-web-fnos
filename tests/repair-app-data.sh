#!/usr/bin/env bash
# Isolated Linux root fixture only. Pass the compiled helper as argument.
set -euo pipefail
helper="${1:?helper binary}"
work=$(mktemp -d)
chmod 755 "$work"
trap 'rm -rf "$work"' EXIT
export TRIM_PKGETC="$work/etc" TRIM_PKGVAR="$work/var" TRIM_PKGHOME="$work/home"
mkdir -p "$TRIM_PKGETC" "$TRIM_PKGHOME" "$TRIM_PKGVAR/logs" "$TRIM_PKGVAR/.uninstall-preserved/etc"
printf retained > "$TRIM_PKGVAR/.uninstall-preserved/etc/config"
printf old > "$TRIM_PKGVAR/logs/install.log"
printf external > "$work/external"
chmod 600 "$work/external"
ln -s "$work/external" "$TRIM_PKGVAR/external-link"
chown -R 12345:12345 "$TRIM_PKGETC" "$TRIM_PKGHOME" "$TRIM_PKGVAR"
chmod 700 "$TRIM_PKGVAR/logs" "$TRIM_PKGVAR/.uninstall-preserved"
chmod 600 "$TRIM_PKGVAR/logs/install.log"
"$helper"
runuser -u nginx-web -- touch "$TRIM_PKGVAR/logs/install.log"
runuser -u nginx-web -- cp -a "$TRIM_PKGVAR/.uninstall-preserved/etc/." "$TRIM_PKGETC/"
[[ $(stat -c %u "$work/external") == 0 ]]
ln "$work/external" "$TRIM_PKGVAR/hard-link"
if "$helper"; then echo 'Hard link accepted' >&2; exit 1; fi
[[ $(stat -c %u "$work/external") == 0 ]]
echo 'Retained ownership migration and external-link protection passed'
