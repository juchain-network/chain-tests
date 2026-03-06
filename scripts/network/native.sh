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

MANAGER="$(cfg_get "$SOURCE_FILE" "native.manager" "pm2")"
INIT_SCRIPT="$(to_abs_path "$(cfg_get "$SOURCE_FILE" "native.init_script" "./scripts/native/pm2_init.sh")")"
ECOSYSTEM_FILE="$(to_abs_path "$(cfg_get "$SOURCE_FILE" "native.ecosystem_file" "./native/ecosystem.config.js")")"
ENV_FILE="$(to_abs_path "$(cfg_get "$SOURCE_FILE" "native.env_file" "./data/native/.env")")"
PM2_NAMESPACE="$(cfg_get "$SOURCE_FILE" "native.pm2_namespace" "ju-chain")"
RPC_URL="$(cfg_get "$SOURCE_FILE" "native.external_rpc" "$(cfg_get "$SOURCE_FILE" "network.external_rpc" "http://localhost:18545")")"
LOG_DIR="$(to_abs_path "$(cfg_get "$SOURCE_FILE" "native.log_dir" "./data/native-logs")")"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120}"
NODE_COUNT="$(cfg_get "$SOURCE_FILE" "network.node_count" "4")"
VALIDATOR_COUNT="$(cfg_get "$SOURCE_FILE" "network.validator_count" "3")"

[[ -f "$INIT_SCRIPT" ]] || die "init script not found: $INIT_SCRIPT"
[[ -f "$ECOSYSTEM_FILE" ]] || die "ecosystem file not found: $ECOSYSTEM_FILE"

command -v "$MANAGER" >/dev/null 2>&1 || die "$MANAGER is required for native backend"

if ! [[ "$NODE_COUNT" =~ ^[0-9]+$ ]] || ! [[ "$VALIDATOR_COUNT" =~ ^[0-9]+$ ]]; then
  die "network.node_count and network.validator_count must be integers"
fi

PM2_PROCS=()
for ((i=1; i<=VALIDATOR_COUNT; i++)); do
  PM2_PROCS+=("${PM2_NAMESPACE}-validator${i}")
done
for ((i=VALIDATOR_COUNT+1; i<=NODE_COUNT; i++)); do
  sync_idx=$((i-VALIDATOR_COUNT))
  if (( sync_idx == 1 )); then
    PM2_PROCS+=("${PM2_NAMESPACE}-syncnode")
  else
    PM2_PROCS+=("${PM2_NAMESPACE}-syncnode${sync_idx}")
  fi
done

pm2_delete_known() {
  local proc
  for proc in "${PM2_PROCS[@]}"; do
    if "$MANAGER" describe "$proc" >/dev/null 2>&1; then
      "$MANAGER" delete "$proc" >/dev/null 2>&1 || true
    fi
  done
}

all_nodes_reth() {
  local idx impl
  for ((idx=0; idx<NODE_COUNT; idx++)); do
    impl="$(awk -F= -v key="NODE${idx}_IMPL" '$1==key {print $2; exit}' "$ENV_FILE" 2>/dev/null | tr -d '[:space:]')"
    if [[ -z "$impl" || "$impl" != "reth" ]]; then
      return 1
    fi
  done
  return 0
}

pm2_start_all() {
  if all_nodes_reth; then
    log "pm2 start using $ECOSYSTEM_FILE (all-reth parallel startup)"
    PM2_NAMESPACE="$PM2_NAMESPACE" NATIVE_ENV_FILE="$ENV_FILE" "$MANAGER" start "$ECOSYSTEM_FILE" --update-env >/dev/null
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
    return 0
  fi

  log "pm2 start using $ECOSYSTEM_FILE (staged startup)"

  if [[ ${#PM2_PROCS[@]} -eq 0 ]]; then
    die "no pm2 processes configured"
  fi

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

  # Bring up remaining validators/sync nodes after primary head is available.
  local idx
  for ((idx=1; idx<${#PM2_PROCS[@]}; idx++)); do
    PM2_NAMESPACE="$PM2_NAMESPACE" NATIVE_ENV_FILE="$ENV_FILE" "$MANAGER" start "$ECOSYSTEM_FILE" --only "${PM2_PROCS[$idx]}" --update-env >/dev/null
    sleep 1
  done
}

pm2_wait_all_online() {
  local timeout="${PROCS_WAIT_TIMEOUT:-$WAIT_TIMEOUT}"
  local deadline=$((SECONDS + timeout))
  local pending=()
  local proc pid

  while (( SECONDS <= deadline )); do
    pending=()
    for proc in "${PM2_PROCS[@]}"; do
      pid="$("$MANAGER" pid "$proc" 2>/dev/null | tr -d '[:space:]' || true)"
      if [[ ! "$pid" =~ ^[0-9]+$ ]] || [[ "$pid" -eq 0 ]]; then
        pending+=("$proc")
      fi
    done

    if [[ ${#pending[@]} -eq 0 ]]; then
      return 0
    fi
    sleep 1
  done

  log "pm2 processes not online after ${timeout}s: ${pending[*]}"
  "$MANAGER" status || true
  for proc in "${pending[@]}"; do
    local log_key="${proc#${PM2_NAMESPACE}-}"
    local err_log="${LOG_DIR}/${log_key}-error.log"
    if [[ -f "$err_log" ]]; then
      log "recent errors for $proc ($err_log):"
      tail -n 20 "$err_log" | sed 's/^/[network]   /'
    fi
  done
  die "native backend startup incomplete"
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
    [[ -f "$ENV_FILE" ]] || die "native env file not found: $ENV_FILE. Run 'make init' first."
    pm2_start_all
    pm2_wait_all_online
    ;;
  down)
    pm2_delete_known
    ;;
  reset)
    pm2_delete_known
    [[ -f "$ENV_FILE" ]] || die "native env file not found: $ENV_FILE. Run 'make init' first."
    pm2_start_all
    pm2_wait_all_online
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
