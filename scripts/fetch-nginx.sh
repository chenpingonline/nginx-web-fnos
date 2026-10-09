#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORE="$(python3 "$ROOT/scripts/resolve-core.py")"
exec "$CORE/scripts/fetch-nginx.sh" "$@"
