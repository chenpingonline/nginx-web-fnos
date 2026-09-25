#!/bin/bash
# Sourced by lifecycle entrypoints. Never execute application code as root.
readonly PACKAGE_PERMISSION_MODE=full-ports

privilege_error() {
  printf '%s\n' "$1" >&2
  # Let fnOS capture stderr; do not open a package-user-controlled log as root.
  exit 1
}

prepare_low_ports() {
  local nginx="${TRIM_APPDEST:?}/bin/nginx" tool
  for tool in setcap getcap; do
    command -v "$tool" >/dev/null 2>&1 || privilege_error "缺少 $tool，无法配置 Nginx 低位端口权限。"
  done
  [ ! -L "${TRIM_APPDEST}/bin" ] && [ -f "$nginx" ] && [ ! -L "$nginx" ] && [ -x "$nginx" ] || privilege_error '内置 Nginx 不存在或不是普通可执行文件，bin 目录和程序不能为符号链接。'
  # Replace, rather than append, capabilities. No setuid and no capability on the backend.
  setcap cap_net_bind_service=ep "$nginx" || privilege_error '设置 Nginx 低位端口权限失败，请检查文件系统是否支持 security.capability。'
  [ "$(getcap "$nginx")" = "$nginx cap_net_bind_service=ep" ] || privilege_error 'Nginx 低位端口权限校验失败。'
}

prepare_package_directories() {
  local directory resolved
  # Root lifecycle mode may create root-owned top-level directories on first
  # install. Only adjust fnOS-provided application roots, never recurse into
  # user-controlled data, certificates, or authorized external directories.
  for directory in "${TRIM_APPDEST:?}" "${TRIM_PKGETC:?}" "${TRIM_PKGVAR:?}" "${TRIM_PKGTMP:?}" "${TRIM_PKGHOME:?}"; do
    resolved="$(readlink -f "$directory")" || privilege_error '无法定位应用目录。'
    case "$resolved" in ""|/|/etc|/var|/tmp|/home|/usr|/opt) privilege_error '拒绝修改系统目录所有者。' ;; esac
    [ -d "$resolved" ] || privilege_error "应用目录不存在：$directory"
    chown -h nginx-web:nginx-web "$resolved" || privilege_error "无法设置应用目录所有者：$directory"
  done
}

enter_package_user() {
  [ "$EUID" -eq 0 ] || return 0
  # Resolve privileged commands only from system directories, never package data.
  export PATH=/usr/sbin:/usr/bin:/sbin:/bin
  local user="${TRIM_USERNAME:-nginx-web}" uid entry
  [ "$user" = nginx-web ] || privilege_error '应用运行用户必须为 nginx-web。'
  uid="$(id -u "$user")" || privilege_error '找不到 nginx-web 应用用户。'
  [ "$uid" -ne 0 ] || privilege_error '拒绝以 root 运行管理服务和 Nginx。'
  command -v runuser >/dev/null 2>&1 || privilege_error '缺少 runuser，无法降权启动应用。'
  entry="$(basename "$0")"
  case "$entry:${1:-}" in
    install_callback:*|upgrade_callback:*|main:start|main:restart)
      prepare_low_ports
      prepare_package_directories
      ;;
  esac
  case "$entry" in
    install_callback|upgrade_callback)
      local repair="$(dirname "${BASH_SOURCE[0]}")/repair-app-data"
      [ -f "$repair" ] && [ ! -L "$repair" ] && [ -x "$repair" ] || privilege_error '缺少应用数据权限修复程序。'
      "$repair" || privilege_error '应用数据权限修复失败，未执行恢复。'
      ;;
  esac
  # Re-enter the same entrypoint before any init, data restoration, or cleanup.
  # runuser retains the fnOS environment and initializes supplementary groups.
  exec runuser -u "$user" -- /bin/bash "$0" "$@"
  privilege_error '切换应用用户失败。'
}
