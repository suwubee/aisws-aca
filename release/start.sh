#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -x "$ROOT/scripts/start.sh" ]]; then
  exec "$ROOT/scripts/start.sh" "$@"
fi

echo "[ERROR] Missing script: $ROOT/scripts/start.sh" >&2
exit 1

