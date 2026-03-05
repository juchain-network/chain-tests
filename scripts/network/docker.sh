#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/lib.sh"

ACTION="${1:-}"
CONFIG_FILE="${2:-}"
SESSION_FILE="${3:-}"

[[ -n "$ACTION" ]] || {
  usage_common
  die "missing action"
}
[[ -n "$CONFIG_FILE" ]] || die "missing config file"

SOURCE_FILE="$CONFIG_FILE"
if [[ -n "$SESSION_FILE" && -f "$SESSION_FILE" && "$ACTION" != "init" ]]; then
  SOURCE_FILE="$SESSION_FILE"
fi

RUNTIME_COMPOSE="$(cfg_get "$SOURCE_FILE" "docker.runtime_compose_file" "./data/docker-compose.runtime.yml")"
BASE_COMPOSE="$(cfg_get "$SOURCE_FILE" "docker.compose_file" "./docker/docker-compose.yml")"
PROJECT_NAME="$(cfg_get "$SOURCE_FILE" "docker.project_name" "juchain-it")"
RPC_URL="$(cfg_get "$SOURCE_FILE" "docker.external_rpc" "$(cfg_get "$SOURCE_FILE" "network.external_rpc" "http://localhost:18545")")"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120}"

RUNTIME_COMPOSE_ABS="$(to_abs_path "$RUNTIME_COMPOSE")"
BASE_COMPOSE_ABS="$(to_abs_path "$BASE_COMPOSE")"

if [[ -f "$RUNTIME_COMPOSE_ABS" ]]; then
  ACTIVE_COMPOSE_FILE="$RUNTIME_COMPOSE_ABS"
else
  ACTIVE_COMPOSE_FILE="$BASE_COMPOSE_ABS"
fi

[[ -f "$ACTIVE_COMPOSE_FILE" ]] || die "compose file not found: $ACTIVE_COMPOSE_FILE"

if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD=(docker-compose)
else
  die "docker compose is required"
fi

compose() {
  "${COMPOSE_CMD[@]}" -f "$ACTIVE_COMPOSE_FILE" -p "$PROJECT_NAME" "$@"
}

do_up() {
  log "docker up using $ACTIVE_COMPOSE_FILE"
  compose up -d
}

do_down() {
  log "docker down using $ACTIVE_COMPOSE_FILE"
  compose down --remove-orphans
}

do_logs() {
  if [[ -n "${NODE:-}" ]]; then
    compose logs -f "$NODE"
  else
    compose logs -f
  fi
}

do_status() {
  compose ps
}

case "$ACTION" in
  up)
    do_up
    ;;
  down)
    do_down
    ;;
  reset)
    do_down
    do_up
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
    ;;
  ready)
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
    ;;
  logs)
    do_logs
    ;;
  status)
    do_status
    ;;
  init)
    log "docker backend has no separate init action"
    ;;
  *)
    usage_common
    die "unsupported action: $ACTION"
    ;;
esac
