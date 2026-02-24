#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/../network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${1:-${TEST_ENV_CONFIG:-}}")"
DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
GENESIS_FILE="$DATA_DIR/genesis.json"
ENV_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.env_file" "./data/native/.env")")"
LOG_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.log_dir" "./data/native-logs")")"

CHAIN_ROOT="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "paths.chain_root" "../chain")")"
GETH_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "native.geth_binary" "")"
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
NETWORK_ID="$(cfg_get "$CONFIG_FILE" "native.network_id" "666666")"
STATE_SCHEME="$(cfg_get "$CONFIG_FILE" "native.state_scheme" "$(cfg_get "$CONFIG_FILE" "network.state_scheme" "hash")")"
HISTORY_STATE="$(cfg_get "$CONFIG_FILE" "native.history_state" "$(cfg_get "$CONFIG_FILE" "network.history_state" "")")"

V1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_http" "18545")"
V1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_ws" "18546")"
V1_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_engine" "18550")"
V1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_p2p" "30301")"

V2_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_http" "18547")"
V2_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_ws" "18548")"
V2_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_engine" "18552")"
V2_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_p2p" "30303")"

V3_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_http" "18549")"
V3_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_ws" "18553")"
V3_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_engine" "18554")"
V3_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_p2p" "30305")"

S1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.sync_http" "18551")"
S1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.sync_ws" "18555")"
S1_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.sync_engine" "18556")"
S1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.sync_p2p" "30307")"

[[ -x "$GETH_BINARY" ]] || die "geth binary not found. tried: ${GETH_CANDIDATES[*]}"
[[ -f "$GENESIS_FILE" ]] || die "missing genesis file: $GENESIS_FILE (run make init first)"

for i in 0 1 2 3; do
  [[ -d "$DATA_DIR/node$i" ]] || die "missing node data dir: $DATA_DIR/node$i (run make init first)"
  [[ -f "$DATA_DIR/node$i/nodekey" ]] || die "missing node key: $DATA_DIR/node$i/nodekey"
  if [[ $i -lt 3 ]]; then
    [[ -f "$DATA_DIR/node$i/validator.key" ]] || die "missing validator key: $DATA_DIR/node$i/validator.key"
    [[ -f "$DATA_DIR/node$i/validator.addr" ]] || die "missing validator addr: $DATA_DIR/node$i/validator.addr"
  fi
done

mkdir -p "$(dirname "$ENV_FILE")" "$LOG_DIR"

init_node() {
  local datadir="$1"
  local init_args=("--datadir" "$datadir")
  if [[ -n "$STATE_SCHEME" ]]; then
    init_args+=("--state.scheme=$STATE_SCHEME")
  fi
  if [[ ! -d "$datadir/geth/chaindata" ]]; then
    "$GETH_BINARY" "${init_args[@]}" init "$GENESIS_FILE" >/dev/null
  fi
}

import_validator_if_needed() {
  local idx="$1"
  local datadir="$DATA_DIR/node$idx"
  local keyfile="$datadir/validator.key"
  local password="$datadir/password.txt"
  local addr
  addr="$(tr -d '[:space:]' < "$datadir/validator.addr")"
  [[ -n "$addr" ]] || die "empty validator addr: $datadir/validator.addr"

  echo "123456" > "$password"
  if ! "$GETH_BINARY" account list --datadir "$datadir" 2>/dev/null | grep -qi "${addr#0x}"; then
    "$GETH_BINARY" account import --datadir "$datadir" --password "$password" "$keyfile" >/dev/null
  fi
}

for i in 0 1 2 3; do
  init_node "$DATA_DIR/node$i"
done
for i in 0 1 2; do
  import_validator_if_needed "$i"
done

VAL1_ADDR="$(tr -d '[:space:]' < "$DATA_DIR/node0/validator.addr")"
VAL2_ADDR="$(tr -d '[:space:]' < "$DATA_DIR/node1/validator.addr")"
VAL3_ADDR="$(tr -d '[:space:]' < "$DATA_DIR/node2/validator.addr")"

PUB0="$(tr -d '[:space:]' < "$DATA_DIR/node0/nodepub")"
PUB1="$(tr -d '[:space:]' < "$DATA_DIR/node1/nodepub")"
PUB2="$(tr -d '[:space:]' < "$DATA_DIR/node2/nodepub")"
PUB3="$(tr -d '[:space:]' < "$DATA_DIR/node3/nodepub")"

BOOTNODES="enode://${PUB0}@127.0.0.1:${V1_P2P},enode://${PUB1}@127.0.0.1:${V2_P2P},enode://${PUB2}@127.0.0.1:${V3_P2P},enode://${PUB3}@127.0.0.1:${S1_P2P}"

cat > "$ENV_FILE" <<EOF
GETH_BINARY=$GETH_BINARY
NETWORK_ID=$NETWORK_ID
BOOTNODES=$BOOTNODES
NATIVE_LOG_DIR=$LOG_DIR
STATE_SCHEME=$STATE_SCHEME
HISTORY_STATE=$HISTORY_STATE

NODE0_DATADIR=$DATA_DIR/node0
NODE1_DATADIR=$DATA_DIR/node1
NODE2_DATADIR=$DATA_DIR/node2
NODE3_DATADIR=$DATA_DIR/node3

NODE0_NODEKEY=$DATA_DIR/node0/nodekey
NODE1_NODEKEY=$DATA_DIR/node1/nodekey
NODE2_NODEKEY=$DATA_DIR/node2/nodekey
NODE3_NODEKEY=$DATA_DIR/node3/nodekey

VALIDATOR1_ADDRESS=$VAL1_ADDR
VALIDATOR2_ADDRESS=$VAL2_ADDR
VALIDATOR3_ADDRESS=$VAL3_ADDR

VALIDATOR1_PASSWORD=$DATA_DIR/node0/password.txt
VALIDATOR2_PASSWORD=$DATA_DIR/node1/password.txt
VALIDATOR3_PASSWORD=$DATA_DIR/node2/password.txt

VALIDATOR1_HTTP_PORT=$V1_HTTP
VALIDATOR1_WS_PORT=$V1_WS
VALIDATOR1_ENGINE_PORT=$V1_ENGINE
VALIDATOR1_P2P_PORT=$V1_P2P

VALIDATOR2_HTTP_PORT=$V2_HTTP
VALIDATOR2_WS_PORT=$V2_WS
VALIDATOR2_ENGINE_PORT=$V2_ENGINE
VALIDATOR2_P2P_PORT=$V2_P2P

VALIDATOR3_HTTP_PORT=$V3_HTTP
VALIDATOR3_WS_PORT=$V3_WS
VALIDATOR3_ENGINE_PORT=$V3_ENGINE
VALIDATOR3_P2P_PORT=$V3_P2P

SYNCNODE_HTTP_PORT=$S1_HTTP
SYNCNODE_WS_PORT=$S1_WS
SYNCNODE_ENGINE_PORT=$S1_ENGINE
SYNCNODE_P2P_PORT=$S1_P2P
EOF

log "native pm2 env generated: $ENV_FILE"
