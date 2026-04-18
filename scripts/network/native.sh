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
TOPOLOGY="$(cfg_get "$SOURCE_FILE" "runtime.topology" "")"
PORTS_SOURCE_FILE="$SOURCE_FILE"
if [[ "$PORTS_SOURCE_FILE" != "$CONFIG_FILE" ]]; then
  if [[ -z "$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.validator1_http" "")" ]]; then
    PORTS_SOURCE_FILE="$CONFIG_FILE"
  fi
fi

[[ -f "$INIT_SCRIPT" ]] || die "init script not found: $INIT_SCRIPT"
[[ -f "$ECOSYSTEM_FILE" ]] || die "ecosystem file not found: $ECOSYSTEM_FILE"

command -v "$MANAGER" >/dev/null 2>&1 || die "$MANAGER is required for native backend"

if ! [[ "$NODE_COUNT" =~ ^[0-9]+$ ]] || ! [[ "$VALIDATOR_COUNT" =~ ^[0-9]+$ ]]; then
  die "network.node_count and network.validator_count must be integers"
fi

if [[ -z "$TOPOLOGY" ]]; then
  if [[ "$NODE_COUNT" == "1" && "$VALIDATOR_COUNT" == "1" ]]; then
    TOPOLOGY="single"
  else
    TOPOLOGY="multi"
  fi
fi

if [[ "$TOPOLOGY" == "single" ]]; then
  SINGLE_SCRIPT="$SCRIPT_DIR/native_single.sh"
  [[ -f "$SINGLE_SCRIPT" ]] || die "native single script not found: $SINGLE_SCRIPT"
  exec "$SINGLE_SCRIPT" "$ACTION" "$SOURCE_FILE"
fi

rpc_urls_for_topology() {
  local idx port
  for ((idx=1; idx<=VALIDATOR_COUNT; idx++)); do
    port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.validator${idx}_http" "")"
    [[ -n "$port" ]] || continue
    printf 'http://localhost:%s\n' "$port"
  done

  for ((idx=VALIDATOR_COUNT+1; idx<=NODE_COUNT; idx++)); do
    local sync_idx=$((idx-VALIDATOR_COUNT))
    if (( sync_idx == 1 )); then
      port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.sync_http" "")"
    else
      port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.sync${sync_idx}_http" "")"
    fi
    [[ -n "$port" ]] || continue
    printf 'http://localhost:%s\n' "$port"
  done
}

rpc_ports_for_topology() {
  local idx port
  for ((idx=1; idx<=VALIDATOR_COUNT; idx++)); do
    port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.validator${idx}_http" "")"
    [[ -n "$port" ]] || continue
    printf '%s\n' "$port"
  done

  for ((idx=VALIDATOR_COUNT+1; idx<=NODE_COUNT; idx++)); do
    local sync_idx=$((idx-VALIDATOR_COUNT))
    if (( sync_idx == 1 )); then
      port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.sync_http" "")"
    else
      port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.sync${sync_idx}_http" "")"
    fi
    [[ -n "$port" ]] || continue
    printf '%s\n' "$port"
  done
}

wait_for_all_rpcs_ready() {
  local timeout="${1:-$WAIT_TIMEOUT}"
  local rpc_url
  while IFS= read -r rpc_url; do
    [[ -n "$rpc_url" ]] || continue
    wait_for_rpc_ready "$rpc_url" "$timeout"
  done < <(rpc_urls_for_topology)
}

validator_rpc_urls() {
  local idx port
  for ((idx=1; idx<=VALIDATOR_COUNT; idx++)); do
    port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.validator${idx}_http" "")"
    [[ -n "$port" ]] || continue
    printf 'validator%d http://localhost:%s\n' "$idx" "$port"
  done
}

rpc_hex_result() {
  local rpc_url="$1"
  local method="$2"
  local response
  response="$(curl -s --max-time 3 \
    -H 'Content-Type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":[]}" \
    "$rpc_url" || true)"

  if [[ "$response" =~ \"result\":\"(0x[0-9a-fA-F]+)\" ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

hex_to_dec() {
  local raw="${1:-}"
  raw="${raw#0x}"
  if [[ -z "$raw" ]]; then
    echo 0
    return 0
  fi
  printf '%d\n' "$((16#$raw))"
}

wait_for_validator_topology_converged() {
  local timeout="${1:-$WAIT_TIMEOUT}"
  local required_rounds="${RETH_TOPOLOGY_STABLE_ROUNDS:-3}"
  local min_peers="${RETH_MIN_VALIDATOR_PEERS:-1}"
  local max_height_delta="${RETH_MAX_VALIDATOR_HEIGHT_DELTA:-2}"
  local deadline=$((SECONDS + timeout))
  local stable_rounds=0
  local last_summary=""
  local validator_entries=()

  mapfile -t validator_entries < <(validator_rpc_urls)
  if (( ${#validator_entries[@]} <= 1 )); then
    return 0
  fi

  log "waiting for validator topology convergence rounds=${required_rounds} timeout=${timeout}s"
  while (( SECONDS <= deadline )); do
    local all_ok=1
    local min_height=-1
    local max_height=-1
    local summary_parts=()
    local entry name rpc_url chain_id block_hex peer_hex block_dec peer_dec

    for entry in "${validator_entries[@]}"; do
      name="${entry%% *}"
      rpc_url="${entry#* }"
      chain_id="$(rpc_hex_result "$rpc_url" "eth_chainId" || true)"
      block_hex="$(rpc_hex_result "$rpc_url" "eth_blockNumber" || true)"
      peer_hex="$(rpc_hex_result "$rpc_url" "net_peerCount" || true)"
      if [[ -z "$chain_id" || -z "$block_hex" || -z "$peer_hex" ]]; then
        all_ok=0
        summary_parts+=("${name}:unready")
        continue
      fi

      block_dec="$(hex_to_dec "$block_hex")"
      peer_dec="$(hex_to_dec "$peer_hex")"
      summary_parts+=("${name}:block=${block_dec}:peers=${peer_dec}")
      if (( peer_dec < min_peers )); then
        all_ok=0
      fi
      if (( min_height < 0 || block_dec < min_height )); then
        min_height=$block_dec
      fi
      if (( max_height < 0 || block_dec > max_height )); then
        max_height=$block_dec
      fi
    done

    if (( min_height >= 0 && max_height >= 0 && max_height - min_height > max_height_delta )); then
      all_ok=0
      summary_parts+=("height_delta=$((max_height - min_height))")
    fi

    last_summary="$(IFS=' '; echo "${summary_parts[*]}")"
    if (( all_ok == 1 )); then
      stable_rounds=$((stable_rounds + 1))
      if (( stable_rounds >= required_rounds )); then
        log "validator topology converged ${last_summary}"
        return 0
      fi
    else
      stable_rounds=0
    fi

    sleep 1
  done

  die "validator topology did not converge after ${timeout}s: ${last_summary}"
}

wait_for_all_rpc_ports_released() {
  local timeout="${1:-30}"
  local deadline=$((SECONDS + timeout))
  local pending=()
  local port

  command -v lsof >/dev/null 2>&1 || return 0

  while (( SECONDS <= deadline )); do
    pending=()
    while IFS= read -r port; do
      [[ -n "$port" ]] || continue
      if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
        pending+=("$port")
      fi
    done < <(rpc_ports_for_topology)

    if [[ ${#pending[@]} -eq 0 ]]; then
      return 0
    fi
    sleep 1
  done

  log "RPC ports still listening after ${timeout}s: ${pending[*]}"
  return 1
}

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

env_value() {
  local key="$1"
  awk -F= -v key="$key" '$1==key {print $2; exit}' "$ENV_FILE" 2>/dev/null | tr -d '[:space:]'
}

coverage_enabled_runtime() {
  local raw="${CHAIN_COVERAGE:-$(env_value "CHAIN_COVERAGE_ENABLED")}"
  case "$(echo "${raw:-0}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

coverage_flush_timeout() {
  local raw="${CHAIN_COVERAGE_FLUSH_TIMEOUT:-20}"
  if [[ "$raw" =~ ^[0-9]+$ ]] && (( raw > 0 )); then
    echo "$raw"
  else
    echo 20
  fi
}

coverage_require_all_nodes() {
  local raw="${CHAIN_COVERAGE_REQUIRE_ALL_NODES:-0}"
  case "$(echo "${raw:-0}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

collect_coverage_dirs() {
  local idx impl dir
  for ((idx=0; idx<NODE_COUNT; idx++)); do
    impl="$(env_value "NODE${idx}_IMPL")"
    dir="$(env_value "NODE${idx}_GOCOVERDIR")"
    if [[ "$impl" == "geth" && -n "$dir" ]]; then
      printf '%s\n' "$dir"
    fi
  done
}

coverage_file_count() {
  local dir="$1"
  find "$dir" -type f 2>/dev/null | wc -l | tr -d '[:space:]'
}

wait_for_coverage_flush() {
  coverage_enabled_runtime || return 0

  local timeout require_all
  timeout="$(coverage_flush_timeout)"
  require_all=0
  if coverage_require_all_nodes; then
    require_all=1
  fi

  local -a dirs=()
  local dir count ready ready_count total
  while IFS= read -r dir; do
    [[ -n "$dir" ]] || continue
    dirs+=("$dir")
  done < <(collect_coverage_dirs)

  total="${#dirs[@]}"
  if (( total == 0 )); then
    log "coverage flush skipped: no geth coverage dirs configured"
    return 0
  fi

  log "waiting for coverage flush (dirs=$total timeout=${timeout}s require_all=${require_all})"
  local deadline=$((SECONDS + timeout))
  while (( SECONDS <= deadline )); do
    ready_count=0
    for dir in "${dirs[@]}"; do
      count="$(coverage_file_count "$dir")"
      if [[ "$count" =~ ^[0-9]+$ ]] && (( count > 0 )); then
        ready_count=$((ready_count + 1))
      fi
    done

    if (( require_all == 1 )); then
      ready=$(( ready_count == total ? 1 : 0 ))
    else
      ready=$(( ready_count > 0 ? 1 : 0 ))
    fi
    if (( ready == 1 )); then
      break
    fi
    sleep 1
  done

  for dir in "${dirs[@]}"; do
    count="$(coverage_file_count "$dir")"
    log "coverage dir: $dir files=${count:-0}"
  done

  if (( require_all == 1 )) && (( ready_count < total )); then
    log "coverage flush incomplete: ready=$ready_count/$total"
  elif (( require_all == 0 )) && (( ready_count == 0 )); then
    log "coverage flush produced no files before timeout"
  fi
}

pm2_stop_known_graceful() {
  local proc
  local -a existing=()
  for proc in "${PM2_PROCS[@]}"; do
    if "$MANAGER" describe "$proc" >/dev/null 2>&1; then
      existing+=("$proc")
    fi
  done
  if [[ ${#existing[@]} -eq 0 ]]; then
    return 0
  fi
  "$MANAGER" stop "${existing[@]}" >/dev/null 2>&1 || true
}

pm2_wait_all_stopped() {
  local timeout="${PROCS_STOP_TIMEOUT:-$(coverage_flush_timeout)}"
  local deadline=$((SECONDS + timeout))
  local pending=()
  local proc pid

  while (( SECONDS <= deadline )); do
    pending=()
    for proc in "${PM2_PROCS[@]}"; do
      pid="$("$MANAGER" pid "$proc" 2>/dev/null | tr -d '[:space:]' || true)"
      if [[ "$pid" =~ ^[0-9]+$ ]] && [[ "$pid" -ne 0 ]]; then
        pending+=("$proc")
      fi
    done
    if [[ ${#pending[@]} -eq 0 ]]; then
      return 0
    fi
    sleep 1
  done

  log "pm2 processes still online after ${timeout}s: ${pending[*]}"
  return 1
}

stop_with_optional_coverage_flush() {
  log "graceful pm2 stop before delete"
  pm2_stop_known_graceful
  pm2_wait_all_stopped || true

  if coverage_enabled_runtime && ! all_nodes_reth; then
    wait_for_coverage_flush
  fi

  pm2_delete_known
  pm2_wait_all_stopped || true
  wait_for_all_rpc_ports_released "${PORTS_STOP_TIMEOUT:-30}" || true
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
  log "pm2 start using $ECOSYSTEM_FILE (staged startup)"

  if [[ ${#PM2_PROCS[@]} -eq 0 ]]; then
    die "no pm2 processes configured"
  fi

  local startup_gap_seconds="${RETH_STARTUP_GAP_SECONDS:-}"
  if [[ -z "$startup_gap_seconds" ]]; then
    if all_nodes_reth; then
      startup_gap_seconds=10
    else
      startup_gap_seconds=1
    fi
  fi
  if ! [[ "$startup_gap_seconds" =~ ^[0-9]+$ ]]; then
    die "RETH_STARTUP_GAP_SECONDS must be an integer, got: $startup_gap_seconds"
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
    local node_http_port=""
    if (( idx < VALIDATOR_COUNT )); then
      node_http_port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.validator$((idx+1))_http" "")"
    else
      local sync_idx=$((idx + 1 - VALIDATOR_COUNT))
      if (( sync_idx == 1 )); then
        node_http_port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.sync_http" "")"
      else
        node_http_port="$(cfg_get "$PORTS_SOURCE_FILE" "native.ports.sync${sync_idx}_http" "")"
      fi
    fi
    if [[ -n "$node_http_port" ]]; then
      wait_for_rpc_ready "http://localhost:${node_http_port}" "$WAIT_TIMEOUT"
    fi
    if (( startup_gap_seconds > 0 && idx < ${#PM2_PROCS[@]} - 1 )); then
      sleep "$startup_gap_seconds"
    fi
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
    wait_for_all_rpcs_ready "$WAIT_TIMEOUT"
    if all_nodes_reth; then
      wait_for_validator_topology_converged "$WAIT_TIMEOUT"
    fi
    ;;
  down)
    stop_with_optional_coverage_flush
    ;;
  reset)
    stop_with_optional_coverage_flush
    [[ -f "$ENV_FILE" ]] || die "native env file not found: $ENV_FILE. Run 'make init' first."
    pm2_start_all
    pm2_wait_all_online
    wait_for_all_rpcs_ready "$WAIT_TIMEOUT"
    if all_nodes_reth; then
      wait_for_validator_topology_converged "$WAIT_TIMEOUT"
    fi
    ;;
  ready)
    wait_for_all_rpcs_ready "$WAIT_TIMEOUT"
    if all_nodes_reth; then
      wait_for_validator_topology_converged "$WAIT_TIMEOUT"
    fi
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
