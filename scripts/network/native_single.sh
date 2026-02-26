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
GETH_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "native.geth_binary" "")"
NETWORK_ID="$(cfg_get "$CONFIG_FILE" "native.network_id" "666666")"
STATE_SCHEME="$(cfg_get "$CONFIG_FILE" "native.state_scheme" "$(cfg_get "$CONFIG_FILE" "network.state_scheme" "hash")")"
HISTORY_STATE="$(cfg_get "$CONFIG_FILE" "native.history_state" "$(cfg_get "$CONFIG_FILE" "network.history_state" "")")"
RPC_HOST="0.0.0.0"
RPC_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_http" "18545")"
WS_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_ws" "18546")"
ENGINE_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_engine" "18550")"
P2P_PORT="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_p2p" "30301")"
RPC_URL="http://127.0.0.1:${RPC_PORT}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120}"

SINGLE_DIR="$DATA_DIR/native-single"
PID_FILE="$SINGLE_DIR/geth.pid"
LOG_FILE="$SINGLE_DIR/geth.log"
PASSWORD_FILE="$DATA_DIR/node0/password.txt"
NODE_DATADIR="$DATA_DIR/node0"
NODE_KEY="$DATA_DIR/node0/nodekey"
VALIDATOR_KEY="$DATA_DIR/node0/validator.key"
VALIDATOR_ADDR_FILE="$DATA_DIR/node0/validator.addr"

GETH_CANDIDATES=()
if [[ -n "$GETH_BINARY_CFG" ]]; then
  GETH_CANDIDATES+=("$(to_abs_path "$GETH_BINARY_CFG")")
fi
GETH_CANDIDATES+=("$CHAIN_ROOT/build/bin/geth")

GETH_BINARY=""
for candidate in "${GETH_CANDIDATES[@]}"; do
  if [[ -x "$candidate" ]]; then
    GETH_BINARY="$candidate"
    break
  fi
done

[[ -x "$GETH_BINARY" ]] || die "geth binary not found. tried: ${GETH_CANDIDATES[*]}"
[[ -f "$GENESIS_FILE" ]] || die "missing genesis file: $GENESIS_FILE (run make init first)"
[[ -d "$NODE_DATADIR" ]] || die "missing node data dir: $NODE_DATADIR (run make init first)"
[[ -f "$NODE_KEY" ]] || die "missing node key: $NODE_KEY"
[[ -f "$VALIDATOR_KEY" ]] || die "missing validator key: $VALIDATOR_KEY"
[[ -f "$VALIDATOR_ADDR_FILE" ]] || die "missing validator addr: $VALIDATOR_ADDR_FILE"

mkdir -p "$SINGLE_DIR"

validator_addr="$(tr -d '[:space:]' < "$VALIDATOR_ADDR_FILE")"
[[ -n "$validator_addr" ]] || die "empty validator address in $VALIDATOR_ADDR_FILE"

is_running() {
  if [[ ! -f "$PID_FILE" ]]; then
    return 1
  fi
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" >/dev/null 2>&1
}

init_node() {
  local init_args=("--datadir" "$NODE_DATADIR")
  if [[ -n "$STATE_SCHEME" ]]; then
    init_args+=("--state.scheme=$STATE_SCHEME")
  fi
  if [[ ! -d "$NODE_DATADIR/geth/chaindata" ]]; then
    "$GETH_BINARY" "${init_args[@]}" init "$GENESIS_FILE" >/dev/null
  fi

  echo "123456" > "$PASSWORD_FILE"
  if ! "$GETH_BINARY" account list --datadir "$NODE_DATADIR" 2>/dev/null | grep -qi "${validator_addr#0x}"; then
    "$GETH_BINARY" account import --datadir "$NODE_DATADIR" --password "$PASSWORD_FILE" "$VALIDATOR_KEY" >/dev/null
  fi
}

start_node() {
  if is_running; then
    log "single node already running (pid=$(cat "$PID_FILE"))"
    return 0
  fi

  local args=(
    "--networkid" "$NETWORK_ID"
    "--datadir" "$NODE_DATADIR"
    "--nodekey" "$NODE_KEY"
    "--syncmode=full"
    "--gcmode=full"
    "--mine"
    "--miner.etherbase" "$validator_addr"
    "--miner.gasprice" "0"
    "--unlock" "$validator_addr"
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

  nohup "$GETH_BINARY" "${args[@]}" >"$LOG_FILE" 2>&1 &
  echo "$!" > "$PID_FILE"
  sleep 1
  if ! is_running; then
    die "failed to start native single node (see $LOG_FILE)"
  fi
}

stop_node() {
  if is_running; then
    local pid
    pid="$(cat "$PID_FILE")"
    kill "$pid" >/dev/null 2>&1 || true
    sleep 1
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
  fi
  rm -f "$PID_FILE"
}

show_status() {
  if is_running; then
    local pid
    pid="$(cat "$PID_FILE")"
    log "native single node running pid=$pid rpc=$RPC_URL"
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
    init_node
    ;;
  up)
    init_node
    start_node
    ;;
  down)
    stop_node
    ;;
  reset)
    stop_node
    init_node
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
