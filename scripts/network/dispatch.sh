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
SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"
EFFECTIVE_CONFIG="$CONFIG_FILE"

case "$ACTION" in
  init)
    ;;
  up|reset|ready|logs|status)
    SESSION_FILE="$(require_runtime_session "$ACTION" "$SESSION_FILE")"
    EFFECTIVE_CONFIG="$SESSION_FILE"
    ;;
  down|resolve-backend)
    if [[ -f "$SESSION_FILE" ]]; then
      EFFECTIVE_CONFIG="$SESSION_FILE"
    fi
    ;;
  *)
    ;;
esac

if [[ -f "$SESSION_FILE" ]]; then
  SESSION_BACKEND="$(session_get "$SESSION_FILE" "runtime.backend" "")"
else
  SESSION_BACKEND=""
fi

BACKEND="native"

if [[ -n "$SESSION_BACKEND" && "$SESSION_BACKEND" != "native" ]]; then
  die "runtime session backend is unsupported: $SESSION_BACKEND (native only)"
fi

CONFIG_BACKEND="$(cfg_get "$EFFECTIVE_CONFIG" "runtime.backend" "native")"
if [[ -n "$CONFIG_BACKEND" && "$CONFIG_BACKEND" != "native" ]]; then
  die "config runtime.backend is unsupported: $CONFIG_BACKEND (native only)"
fi

if [[ "$ACTION" == "resolve-backend" ]]; then
  echo "$BACKEND"
  exit 0
fi

exec "$SCRIPT_DIR/native.sh" "$ACTION" "$CONFIG_FILE" "$SESSION_FILE"
