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

DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
GENESIS_FILE="$DATA_DIR/genesis.json"
CHAIN_ROOT="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "paths.chain_root" "../chain")")"
RETH_ROOT="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "paths.reth_root" "../rchain")")"
GETH_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "binaries.geth_native" "$(cfg_get "$CONFIG_FILE" "native.geth_binary" "")")"
RETH_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "binaries.reth_native" "$(cfg_get "$CONFIG_FILE" "native.reth_binary" "")")"
NETWORK_ID="$(cfg_get "$CONFIG_FILE" "native.network_id" "666666")"
STATE_SCHEME="$(cfg_get "$CONFIG_FILE" "native.state_scheme" "$(cfg_get "$CONFIG_FILE" "network.state_scheme" "hash")")"
HISTORY_STATE="$(cfg_get "$CONFIG_FILE" "native.history_state" "$(cfg_get "$CONFIG_FILE" "network.history_state" "")")"
BLACKLIST_ENABLED="$(cfg_get "$CONFIG_FILE" "blacklist.enabled" "false")"
BLACKLIST_CONTRACT_ADDR="$(cfg_get "$CONFIG_FILE" "blacklist.contract_address" "0x1db0EDE439708A923431DC68fd3F646c0A4D4e6E")"
BLACKLIST_MODE="$(cfg_get "$CONFIG_FILE" "blacklist.mode" "mock")"
BLACKLIST_ALERT_FAIL_OPEN="$(cfg_get "$CONFIG_FILE" "blacklist.alert_fail_open" "true")"
BLACKLIST_REFRESH_SECONDS="$(cfg_get "$CONFIG_FILE" "blacklist.refresh_interval_seconds" "")"
UPGRADE_OVERRIDE_POSA_TIME="${UPGRADE_OVERRIDE_POSA_TIME:-$(cfg_get "$CONFIG_FILE" "fork.override.posa_time" "$(cfg_get "$CONFIG_FILE" "network.upgrade_override.posa_time" "")")}"
UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON="${UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON:-}"
UPGRADE_OVERRIDE_POSA_SIGNERS_JSON="${UPGRADE_OVERRIDE_POSA_SIGNERS_JSON:-}"
if [[ -z "$UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON" ]]; then
  if [[ -n "${UPGRADE_OVERRIDE_POSA_VALIDATORS:-}" ]]; then
    UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON="$(python3 - "${UPGRADE_OVERRIDE_POSA_VALIDATORS:-}" <<'PY'
import json
import sys
raw = sys.argv[1].strip()
items = [item.strip() for item in raw.split(",") if item.strip()]
print(json.dumps(items))
PY
)"
  else
    UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON="$(cfg_get_json "$CONFIG_FILE" "fork.override.posa_validators" "$(cfg_get_json "$CONFIG_FILE" "network.upgrade_override.posa_validators" "[]")")"
  fi
fi
if [[ -z "$UPGRADE_OVERRIDE_POSA_SIGNERS_JSON" ]]; then
  if [[ -n "${UPGRADE_OVERRIDE_POSA_SIGNERS:-}" ]]; then
    UPGRADE_OVERRIDE_POSA_SIGNERS_JSON="$(python3 - "${UPGRADE_OVERRIDE_POSA_SIGNERS:-}" <<'PY'
import json
import sys
raw = sys.argv[1].strip()
items = [item.strip() for item in raw.split(",") if item.strip()]
print(json.dumps(items))
PY
)"
  else
    UPGRADE_OVERRIDE_POSA_SIGNERS_JSON="$(cfg_get_json "$CONFIG_FILE" "fork.override.posa_signers" "$(cfg_get_json "$CONFIG_FILE" "network.upgrade_override.posa_signers" "[]")")"
  fi
fi
VALIDATOR_AUTH_MODE="$(cfg_get "$CONFIG_FILE" "validator_auth.mode" "auto")"
KEYSTORE_PASSWORD_FILE_CFG="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_file" "")"
KEYSTORE_PASSWORD_ENV_NAME="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_env" "")"
RUNTIME_IMPL_MODE="$(cfg_get "$CONFIG_FILE" "runtime.impl_mode" "single")"
DEFAULT_RUNTIME_IMPL="$(cfg_get "$CONFIG_FILE" "runtime.impl" "geth")"
NODE0_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node0.impl" "")"
NODE0_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node0.binary" "")"

RPC_HOST="0.0.0.0"
RPC_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_http" "18545")"
WS_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_ws" "18546")"
ENGINE_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_engine" "18550")"
P2P_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_p2p" "40401")"
RPC_URL="http://127.0.0.1:${RPC_PORT}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120}"

SINGLE_DIR="$DATA_DIR/native-single"
PID_FILE="$SINGLE_DIR/node.pid"
LOG_FILE="$SINGLE_DIR/node.log"
PASSWORD_FILE="$DATA_DIR/node0/password.txt"
NODE_DATADIR="$DATA_DIR/node0"
NODE_KEY="$DATA_DIR/node0/nodekey"
VALIDATOR_KEY="$DATA_DIR/node0/signer.key"
VALIDATOR_ADDR_FILE="$DATA_DIR/node0/signer.addr"
[[ -f "$VALIDATOR_KEY" ]] || VALIDATOR_KEY="$DATA_DIR/node0/validator.key"
[[ -f "$VALIDATOR_ADDR_FILE" ]] || VALIDATOR_ADDR_FILE="$DATA_DIR/node0/validator.addr"
KEYSTORE_DIR="$DATA_DIR/node0/keystore"
KEYSTORE_ADDR_FILE="$DATA_DIR/node0/keystore.addr"

normalize_impl() {
  local impl="${1:-}"
  case "$impl" in
    geth|reth) echo "$impl" ;;
    *) die "unsupported runtime implementation: $impl (expected geth|reth)" ;;
  esac
}

resolve_binary() {
  local candidate
  for candidate in "$@"; do
    [[ -n "$candidate" ]] || continue
    if [[ -x "$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

resolve_node_impl() {
  case "$RUNTIME_IMPL_MODE" in
    single)
      normalize_impl "$DEFAULT_RUNTIME_IMPL"
      ;;
    mixed)
      [[ -n "$NODE0_IMPL_CFG" ]] || die "runtime_nodes.node0.impl is required when runtime.impl_mode=mixed"
      normalize_impl "$NODE0_IMPL_CFG"
      ;;
    *)
      die "runtime.impl_mode must be single|mixed, got: $RUNTIME_IMPL_MODE"
      ;;
  esac
}

resolve_node_binary() {
  local impl="$1"
  if [[ -n "$NODE0_BINARY_CFG" ]]; then
    to_abs_path "$NODE0_BINARY_CFG"
    return 0
  fi

  case "$impl" in
    geth)
      if [[ -n "$GETH_BINARY_CFG" ]]; then
        to_abs_path "$GETH_BINARY_CFG"
      else
        echo "$CHAIN_ROOT/build/bin/geth"
      fi
      ;;
    reth)
      if [[ -n "$RETH_BINARY_CFG" ]]; then
        to_abs_path "$RETH_BINARY_CFG"
      else
        echo "$RETH_ROOT/target/release/congress-node"
      fi
      ;;
    *)
      die "unsupported runtime implementation for binary resolution: $impl"
      ;;
  esac
}

coverage_enabled() {
  case "$(echo "${CHAIN_COVERAGE:-0}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

coverage_session_file() {
  echo "$ROOT_DIR/reports/.coverage_state/session.env"
}

coverage_enabled_effective() {
  if coverage_enabled; then
    return 0
  fi
  [[ -f "$(coverage_session_file)" ]]
}

coverage_out_dir_effective() {
  if [[ -n "${CHAIN_COVERAGE_OUT_DIR:-}" ]]; then
    to_abs_path "$CHAIN_COVERAGE_OUT_DIR"
    return 0
  fi
  local session_file
  session_file="$(coverage_session_file)"
  if [[ -f "$session_file" ]]; then
    awk -F= '$1=="SESSION_CHAIN_COVERAGE_OUT_DIR" {print $2; exit}' "$session_file" 2>/dev/null
  fi
}

coverage_node_dir_effective() {
  local out_dir
  out_dir="$(coverage_out_dir_effective)"
  [[ -n "$out_dir" ]] || return 1
  echo "$out_dir/raw/node0"
}

coverage_flush_timeout() {
  local raw="${CHAIN_COVERAGE_FLUSH_TIMEOUT:-20}"
  if [[ "$raw" =~ ^[0-9]+$ ]] && (( raw > 0 )); then
    echo "$raw"
  else
    echo 20
  fi
}

graceful_stop_wait() {
  local pid="$1"
  local timeout="$2"
  local deadline=$((SECONDS + timeout))
  while (( SECONDS <= deadline )); do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_single_coverage_flush() {
  coverage_enabled_effective || return 0
  [[ "$NODE_IMPL" == "geth" ]] || return 0

  local dir timeout count
  dir="$(coverage_node_dir_effective || true)"
  [[ -n "$dir" ]] || {
    log "coverage flush skipped: no single-node coverage dir configured"
    return 0
  }
  mkdir -p "$dir"

  timeout="$(coverage_flush_timeout)"
  log "waiting for single-node coverage flush: $dir (timeout=${timeout}s)"
  local deadline=$((SECONDS + timeout))
  while (( SECONDS <= deadline )); do
    count="$(find "$dir" -type f 2>/dev/null | wc -l | tr -d '[:space:]')"
    if [[ "$count" =~ ^[0-9]+$ ]] && (( count > 0 )); then
      log "coverage dir: $dir files=$count"
      return 0
    fi
    sleep 1
  done
  count="$(find "$dir" -type f 2>/dev/null | wc -l | tr -d '[:space:]')"
  log "coverage dir: $dir files=${count:-0}"
  log "coverage flush produced no files before timeout"
}

resolve_validator_auth_mode() {
  case "$VALIDATOR_AUTH_MODE" in
    auto|private_key|keystore) echo "$VALIDATOR_AUTH_MODE" ;;
    *) die "validator_auth.mode must be auto|private_key|keystore, got: $VALIDATOR_AUTH_MODE" ;;
  esac
}

json_addresses_to_csv() {
  local raw_json="${1:-[]}"
  python3 - "$raw_json" <<'PY'
import json
import sys

try:
    items = json.loads(sys.argv[1] or "[]")
except Exception:
    items = []

if not isinstance(items, list):
    items = []

print(",".join(str(item).strip() for item in items if str(item).strip()))
PY
}

is_running() {
  if [[ ! -f "$PID_FILE" ]]; then
    return 1
  fi
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" >/dev/null 2>&1
}

signer_addr="$(tr -d '[:space:]' < "$VALIDATOR_ADDR_FILE" 2>/dev/null || true)"
if [[ -z "$signer_addr" && -f "$VALIDATOR_ADDR_FILE" ]]; then
  signer_addr="$(tr -d '[:space:]' < "$VALIDATOR_ADDR_FILE")"
fi

NODE_IMPL="$(resolve_node_impl)"
NODE_BINARY_CFG_RESOLVED="$(resolve_node_binary "$NODE_IMPL")"
AUTH_MODE="$(resolve_validator_auth_mode)"
UPGRADE_OVERRIDE_POSA_VALIDATORS="$(json_addresses_to_csv "$UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON")"
UPGRADE_OVERRIDE_POSA_SIGNERS="$(json_addresses_to_csv "$UPGRADE_OVERRIDE_POSA_SIGNERS_JSON")"

if coverage_enabled && [[ "$NODE_IMPL" != "geth" ]]; then
  die "CHAIN_COVERAGE=1 only supports native geth"
fi

coverage_geth_binary=""
if [[ "$NODE_IMPL" == "geth" ]] && coverage_enabled; then
  if [[ -n "${CHAIN_COVERAGE_GETH_BINARY:-}" ]]; then
    coverage_geth_binary="$(to_abs_path "$CHAIN_COVERAGE_GETH_BINARY")"
  else
    coverage_geth_binary="$("$ROOT_DIR/scripts/coverage/prepare_chain_coverage.sh" --config "$CONFIG_FILE" --print-binary)"
  fi
fi
if ! GETH_BINARY="$(resolve_binary "$coverage_geth_binary" "$NODE_BINARY_CFG_RESOLVED" "$CHAIN_ROOT/build/bin/geth")"; then
  GETH_BINARY=""
fi
if ! RETH_BINARY="$(resolve_binary "$NODE_BINARY_CFG_RESOLVED" "$RETH_ROOT/target/release/congress-node" "$RETH_ROOT/target/debug/congress-node")"; then
  RETH_BINARY=""
fi

if [[ -n "$UPGRADE_OVERRIDE_POSA_TIME" ]] && ! [[ "$UPGRADE_OVERRIDE_POSA_TIME" =~ ^[0-9]+$ ]]; then
  die "fork.override.posa_time must be an unsigned integer timestamp, got: $UPGRADE_OVERRIDE_POSA_TIME"
fi
if [[ -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS" && -z "$UPGRADE_OVERRIDE_POSA_SIGNERS" ]] || [[ -z "$UPGRADE_OVERRIDE_POSA_VALIDATORS" && -n "$UPGRADE_OVERRIDE_POSA_SIGNERS" ]]; then
  die "fork.override.posa_validators and fork.override.posa_signers must be provided together"
fi
if [[ "$NODE_IMPL" == "reth" ]] && [[ -n "$UPGRADE_OVERRIDE_POSA_TIME" || -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS" || -n "$UPGRADE_OVERRIDE_POSA_SIGNERS" ]]; then
  die "upgrade override currently supports geth runtime only"
fi

[[ -f "$GENESIS_FILE" ]] || die "missing genesis file: $GENESIS_FILE (run make init first)"
[[ -d "$NODE_DATADIR" ]] || die "missing node data dir: $NODE_DATADIR (run make init first)"
[[ -f "$NODE_KEY" ]] || die "missing node key: $NODE_KEY"
[[ -f "$VALIDATOR_KEY" ]] || die "missing validator key: $VALIDATOR_KEY"
[[ -f "$VALIDATOR_ADDR_FILE" ]] || die "missing validator addr: $VALIDATOR_ADDR_FILE"
[[ -n "$signer_addr" ]] || die "empty signer address in $VALIDATOR_ADDR_FILE"

mkdir -p "$SINGLE_DIR"

init_geth() {
  [[ -n "$GETH_BINARY" ]] || die "geth binary not found"
  local init_args=("--datadir" "$NODE_DATADIR")
  if [[ -n "$STATE_SCHEME" ]]; then
    init_args+=("--state.scheme=$STATE_SCHEME")
  fi
  if [[ ! -d "$NODE_DATADIR/geth/chaindata" ]]; then
    "$GETH_BINARY" "${init_args[@]}" init "$GENESIS_FILE" >/dev/null
  fi

  if [[ ! -f "$PASSWORD_FILE" ]]; then
    printf '%s\n' "123456" > "$PASSWORD_FILE"
  fi
  if ! "$GETH_BINARY" account list --datadir "$NODE_DATADIR" 2>/dev/null | grep -qi "${signer_addr#0x}"; then
    "$GETH_BINARY" account import --datadir "$NODE_DATADIR" --password "$PASSWORD_FILE" "$VALIDATOR_KEY" >/dev/null
  fi
}

init_reth() {
  [[ -n "$RETH_BINARY" ]] || die "reth binary not found"
  if [[ ! -d "$NODE_DATADIR/db" ]]; then
    CONGRESS_GENESIS="$GENESIS_FILE" \
    "$RETH_BINARY" init --chain "$GENESIS_FILE" --datadir "$NODE_DATADIR" \
      --log.file.directory "$SINGLE_DIR" >/dev/null
  fi
}

resolve_reth_auth_args() {
  local mode="$1"
  local key_hex
  key_hex="$(tr -d '[:space:]' < "$VALIDATOR_KEY")"
  key_hex="${key_hex#0x}"
  local private_key="0x${key_hex}"
  local pass_file="$PASSWORD_FILE"
  local env_pass=""

  if [[ -n "$KEYSTORE_PASSWORD_FILE_CFG" ]]; then
    pass_file="$(to_abs_path "$KEYSTORE_PASSWORD_FILE_CFG")"
  elif [[ ! -f "$pass_file" && -n "$KEYSTORE_PASSWORD_ENV_NAME" ]]; then
    env_pass="$(printenv "$KEYSTORE_PASSWORD_ENV_NAME" || true)"
    if [[ -n "$env_pass" ]]; then
      printf '%s\n' "$env_pass" > "$pass_file"
    fi
  fi

  local keystore_file=""
  if [[ -d "$KEYSTORE_DIR" ]]; then
    keystore_file="$(find "$KEYSTORE_DIR" -type f | head -n 1 || true)"
  fi
  local keystore_addr=""
  if [[ -f "$KEYSTORE_ADDR_FILE" ]]; then
    keystore_addr="$(tr -d '[:space:]' < "$KEYSTORE_ADDR_FILE")"
  fi

  if [[ "$mode" == "private_key" ]]; then
    echo "--validator-private-key $private_key"
    return 0
  fi

  local use_keystore=false
  if [[ "$mode" == "keystore" ]]; then
    use_keystore=true
  elif [[ "$mode" == "auto" && -n "$keystore_file" ]]; then
    use_keystore=true
  fi

  if $use_keystore; then
    local out=""
    if [[ -n "$keystore_file" ]]; then
      out+="--keystore-path $keystore_file"
    else
      out+="--keystore-dir $KEYSTORE_DIR"
    fi
    if [[ -f "$pass_file" ]]; then
      out+=" --keystore-pass-file $pass_file"
    elif [[ -n "$KEYSTORE_PASSWORD_ENV_NAME" ]]; then
      out+=" --keystore-pass env:$KEYSTORE_PASSWORD_ENV_NAME"
    fi
    if [[ -n "$keystore_addr" ]]; then
      out+=" --keystore-address $keystore_addr"
    fi
    echo "$out"
    return 0
  fi

  echo "--validator-private-key $private_key"
}

start_geth() {
  local cover_dir=""
  if coverage_enabled; then
    cover_dir="$("$ROOT_DIR/scripts/coverage/prepare_chain_coverage.sh" --config "$CONFIG_FILE" --print-node-dir node0)"
    mkdir -p "$cover_dir"
  fi
  local args=(
    "--networkid" "$NETWORK_ID"
    "--datadir" "$NODE_DATADIR"
    "--nodekey" "$NODE_KEY"
    "--syncmode=full"
    "--gcmode=full"
    "--mine"
    "--miner.etherbase" "$signer_addr"
    "--miner.gasprice" "0"
    "--unlock" "$signer_addr"
    "--password" "$PASSWORD_FILE"
    "--allow-insecure-unlock"
    "--http"
    "--http.addr=$RPC_HOST"
    "--http.port=$RPC_PORT"
    "--http.vhosts=*"
    "--http.corsdomain=*"
    "--http.api=web3,debug,eth,txpool,net,personal,admin,miner,congress"
    "--ws"
    "--ws.addr=$RPC_HOST"
    "--ws.port=$WS_PORT"
    "--ws.origins=*"
    "--ws.api=debug,eth,txpool,net,engine,personal,admin,miner,congress"
    "--authrpc.port=$ENGINE_PORT"
    "--port=$P2P_PORT"
    "--nodiscover"
    "--maxpeers=0"
    "--verbosity=3"
    "--cache=1024"
  )

  if [[ -n "$STATE_SCHEME" ]]; then
    args+=("--state.scheme=$STATE_SCHEME")
  fi
  if [[ -n "$HISTORY_STATE" ]]; then
    args+=("--history.state=$HISTORY_STATE")
  fi
  if [[ -n "$UPGRADE_OVERRIDE_POSA_TIME" ]]; then
    args+=("--override.posaTime=$UPGRADE_OVERRIDE_POSA_TIME")
  fi
  if [[ -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS" ]]; then
    args+=("--override.posaValidators=$UPGRADE_OVERRIDE_POSA_VALIDATORS")
    args+=("--override.posaSigners=$UPGRADE_OVERRIDE_POSA_SIGNERS")
  fi

  log "starting native single geth signer=$signer_addr overrideValidators=${UPGRADE_OVERRIDE_POSA_VALIDATORS:-<none>} overrideSigners=${UPGRADE_OVERRIDE_POSA_SIGNERS:-<none>}"

  if [[ -n "$cover_dir" ]]; then
    GOCOVERDIR="$cover_dir" \
    BLACKLIST_ENABLED="$BLACKLIST_ENABLED" \
    BLACKLIST_CONTRACT_ADDR="$BLACKLIST_CONTRACT_ADDR" \
    BLACKLIST_MODE="$BLACKLIST_MODE" \
    BLACKLIST_ALERT_FAIL_OPEN="$BLACKLIST_ALERT_FAIL_OPEN" \
    BLACKLIST_REFRESH_INTERVAL="$BLACKLIST_REFRESH_SECONDS" \
    nohup "$GETH_BINARY" "${args[@]}" >"$LOG_FILE" 2>&1 &
  else
    BLACKLIST_ENABLED="$BLACKLIST_ENABLED" \
    BLACKLIST_CONTRACT_ADDR="$BLACKLIST_CONTRACT_ADDR" \
    BLACKLIST_MODE="$BLACKLIST_MODE" \
    BLACKLIST_ALERT_FAIL_OPEN="$BLACKLIST_ALERT_FAIL_OPEN" \
    BLACKLIST_REFRESH_INTERVAL="$BLACKLIST_REFRESH_SECONDS" \
    nohup "$GETH_BINARY" "${args[@]}" >"$LOG_FILE" 2>&1 &
  fi
  echo "$!" > "$PID_FILE"
}

start_reth() {
  local args=(
    node
    --chain "$GENESIS_FILE"
    --datadir "$NODE_DATADIR"
    --http
    --http.addr "$RPC_HOST"
    --http.port "$RPC_PORT"
    --http.api all
    --ws
    --ws.addr "$RPC_HOST"
    --ws.port "$WS_PORT"
    --ws.api all
    --authrpc.port "$ENGINE_PORT"
    --port "$P2P_PORT"
    --discovery.port "$P2P_PORT"
    --p2p-secret-key "$NODE_KEY"
    --log.file.directory "$SINGLE_DIR"
  )

  auth_extra="$(resolve_reth_auth_args "$AUTH_MODE")"
  if [[ -n "$auth_extra" ]]; then
    # shellcheck disable=SC2206
    auth_args=( $auth_extra )
    args+=("${auth_args[@]}")
  fi

  BLACKLIST_ENABLED="$BLACKLIST_ENABLED" \
  BLACKLIST_CONTRACT_ADDR="$BLACKLIST_CONTRACT_ADDR" \
  BLACKLIST_MODE="$BLACKLIST_MODE" \
  BLACKLIST_ALERT_FAIL_OPEN="$BLACKLIST_ALERT_FAIL_OPEN" \
  BLACKLIST_REFRESH_INTERVAL="$BLACKLIST_REFRESH_SECONDS" \
  CONGRESS_GENESIS="$GENESIS_FILE" \
  nohup "$RETH_BINARY" "${args[@]}" >"$LOG_FILE" 2>&1 &
  echo "$!" > "$PID_FILE"
}

start_node() {
  if is_running; then
    log "single node already running (pid=$(cat "$PID_FILE"))"
    return 0
  fi

  case "$NODE_IMPL" in
    geth)
      init_geth
      start_geth
      ;;
    reth)
      init_reth
      start_reth
      ;;
    *)
      die "unsupported node impl: $NODE_IMPL"
      ;;
  esac

  sleep 1
  if ! is_running; then
    die "failed to start native single node (see $LOG_FILE)"
  fi
}

stop_node() {
  if is_running; then
    local pid
    pid="$(cat "$PID_FILE")"
    if coverage_enabled_effective && [[ "$NODE_IMPL" == "geth" ]]; then
      local timeout
      timeout="$(coverage_flush_timeout)"
      log "coverage-enabled single-node stop: graceful SIGINT before force kill"
      kill -INT "$pid" >/dev/null 2>&1 || true
      if ! graceful_stop_wait "$pid" "$timeout"; then
        kill -TERM "$pid" >/dev/null 2>&1 || true
        graceful_stop_wait "$pid" 5 || true
      fi
      if kill -0 "$pid" >/dev/null 2>&1; then
        kill -9 "$pid" >/dev/null 2>&1 || true
      fi
      wait_for_single_coverage_flush
    else
      kill "$pid" >/dev/null 2>&1 || true
      sleep 1
      if kill -0 "$pid" >/dev/null 2>&1; then
        kill -9 "$pid" >/dev/null 2>&1 || true
      fi
    fi
  fi
  rm -f "$PID_FILE"
}

show_status() {
  if is_running; then
    local pid
    pid="$(cat "$PID_FILE")"
    log "native single node running pid=$pid impl=$NODE_IMPL rpc=$RPC_URL"
  else
    log "native single node is stopped"
  fi
}

show_logs() {
  if [[ ! -f "$LOG_FILE" ]]; then
    die "single node log not found: $LOG_FILE"
  fi
  if [[ -n "${TAIL_LINES:-}" ]]; then
    tail -n "$TAIL_LINES" "$LOG_FILE"
  else
    tail -f "$LOG_FILE"
  fi
}

case "$ACTION" in
  init)
    if [[ "$NODE_IMPL" == "geth" ]]; then
      init_geth
    else
      init_reth
    fi
    ;;
  up)
    start_node
    ;;
  down)
    stop_node
    ;;
  reset)
    stop_node
    start_node
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
    ;;
  ready)
    wait_for_rpc_ready "$RPC_URL" "$WAIT_TIMEOUT"
    ;;
  status)
    show_status
    ;;
  logs)
    show_logs
    ;;
  *)
    usage_common
    die "unsupported action: $ACTION"
    ;;
esac
