#!/bin/bash
# Standard package: fnOS starts lifecycle scripts as the application user.
readonly PACKAGE_PERMISSION_MODE=standard

enter_package_user() {
  if [ "$EUID" -eq 0 ]; then
    printf '%s\n' '标准版生命周期必须以 nginx-web 应用用户运行。' >&2
    exit 1
  fi
}
