#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/lib.sh"

ACTION="${1:-}"
CONFIG_FILE="${2:-}"

[[ -n "$ACTION" ]] || {
  usage_common
  die "missing action"
}
[[ -n "$CONFIG_FILE" ]] || die "missing config file"

MANAGER="$(cfg_get "$CONFIG_FILE" "native.manager" "pm2")"
INIT_SCRIPT="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.init_script" "./scripts/native/pm2_init.sh")")"
ECOSYSTEM_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.ecosystem_file" "./native/ecosystem.config.js")")"
ENV_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.env_file" "./data/native/.env")")"
PM2_NAMESPACE="$(cfg_get "$CONFIG_FILE" "native.pm2_namespace" "ju-chain")"
RPC_URL="$(cfg_get "$CONFIG_FILE" "native.external_rpc" "$(cfg_get "$CONFIG_FILE" "network.external_rpc" "http://localhost:18545")")"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120}"

[[ -f "$INIT_SCRIPT" ]] || die "init script not found: $INIT_SCRIPT"
[[ -f "$ECOSYSTEM_FILE" ]] || die "ecosystem file not found: $ECOSYSTEM_FILE"

command -v "$MANAGER" >/dev/null 2>&1 || die "$MANAGER is required for native backend"

PM2_PROCS=(
  "${PM2_NAMESPACE}-validator1"
  "${PM2_NAMESPACE}-validator2"
  "${PM2_NAMESPACE}-validator3"
  "${PM2_NAMESPACE}-syncnode"
)

pm2_delete_known() {
  local proc
  for proc in "${PM2_PROCS[@]}"; do
    if "$MANAGER" describe "$proc" >/dev/null 2>&1; then
      "$MANAGER" delete "$proc" >/dev/null 2>&1 || true
    fi
  done
}

pm2_start_all() {
  log "pm2 start using $ECOSYSTEM_FILE (staged startup)"

  # Start primary validator first to avoid early clique fork races.
  PM2_NAMESPACE="$PM2_NAMESPACE" NATIVE_ENV_FILE="$ENV_FILE" "$MANAGER" start "$ECOSYSTEM_FILE" --only "${PM2_PROCS[0]}" --update-env >/dev/null

  log "waiting for primary validator bootstrap block: $RPC_URL"
  # In staged startup only validator1 is online at this point; on Congress it may
  # produce block #1 then pause until peers join. So gate on "block >= 1" instead
  # of continuous head increments to avoid startup deadlock.
  if ! RETRIES=60 MIN_BLOCK=1 INCREMENTS_REQUIRED=0 "$ROOT_DIR/scripts/wait_for_node.sh" "$RPC_URL" >/dev/null 2>&1; then
    # Fallback keeps behavior backward-compatible if block-progress probe is unavailable.
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
  fi

  # Bring up remaining validators/sync node after chain head is moving.
  PM2_NAMESPACE="$PM2_NAMESPACE" NATIVE_ENV_FILE="$ENV_FILE" "$MANAGER" start "$ECOSYSTEM_FILE" --only "${PM2_PROCS[1]}" --update-env >/dev/null
  sleep 1
  PM2_NAMESPACE="$PM2_NAMESPACE" NATIVE_ENV_FILE="$ENV_FILE" "$MANAGER" start "$ECOSYSTEM_FILE" --only "${PM2_PROCS[2]}" --update-env >/dev/null
  sleep 1
  PM2_NAMESPACE="$PM2_NAMESPACE" NATIVE_ENV_FILE="$ENV_FILE" "$MANAGER" start "$ECOSYSTEM_FILE" --only "${PM2_PROCS[3]}" --update-env >/dev/null
}

pm2_status() {
  "$MANAGER" status
}

pm2_logs() {
  if [[ -n "${NODE:-}" ]]; then
    "$MANAGER" logs "$NODE"
    return 0
  fi
  "$MANAGER" logs "${PM2_PROCS[@]}"
}

run_init() {
  log "running native init script: $INIT_SCRIPT"
  if [[ -x "$INIT_SCRIPT" ]]; then
    TEST_ENV_CONFIG="$CONFIG_FILE" "$INIT_SCRIPT" "$CONFIG_FILE"
  else
    TEST_ENV_CONFIG="$CONFIG_FILE" bash "$INIT_SCRIPT" "$CONFIG_FILE"
  fi
}

case "$ACTION" in
  up)
    if [[ ! -f "$ENV_FILE" ]]; then
      run_init
    fi
    pm2_start_all
    ;;
  down)
    pm2_delete_known
    ;;
  reset)
    pm2_delete_known
    run_init
    pm2_start_all
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
    ;;
  ready)
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
    ;;
  logs)
    pm2_logs
    ;;
  status)
    pm2_status
    ;;
  init)
    run_init
    ;;
  *)
    usage_common
    die "unsupported action: $ACTION"
    ;;
esac
