#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/lib.sh"

ACTION="${1:-}"
CONFIG_ARG="${2:-}"

[[ -n "$ACTION" ]] || {
  usage_common
  die "missing action"
}

CONFIG_FILE="$(resolve_config_file "$CONFIG_ARG")"
BACKEND="${RUNTIME_BACKEND:-$(cfg_get "$CONFIG_FILE" "runtime.backend" "native")}"

case "$BACKEND" in
  docker)
    exec "$SCRIPT_DIR/docker.sh" "$ACTION" "$CONFIG_FILE"
    ;;
  native)
    exec "$SCRIPT_DIR/native.sh" "$ACTION" "$CONFIG_FILE"
    ;;
  *)
    usage_common
    die "unsupported backend: $BACKEND (expected: docker|native)"
    ;;
esac

