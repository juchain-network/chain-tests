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
RETH_ROOT="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "paths.reth_root" "../rchain")")"

GETH_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "binaries.geth_native" "$(cfg_get "$CONFIG_FILE" "native.geth_binary" "")")"
RETH_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "binaries.reth_native" "$(cfg_get "$CONFIG_FILE" "native.reth_binary" "")")"

NETWORK_ID="$(cfg_get "$CONFIG_FILE" "native.network_id" "666666")"
STATE_SCHEME="$(cfg_get "$CONFIG_FILE" "native.state_scheme" "$(cfg_get "$CONFIG_FILE" "network.state_scheme" "hash")")"
HISTORY_STATE="$(cfg_get "$CONFIG_FILE" "native.history_state" "$(cfg_get "$CONFIG_FILE" "network.history_state" "")")"

RUNTIME_IMPL_MODE="$(cfg_get "$CONFIG_FILE" "runtime.impl_mode" "single")"
DEFAULT_RUNTIME_IMPL="$(cfg_get "$CONFIG_FILE" "runtime.impl" "geth")"
VALIDATOR_AUTH_MODE="$(cfg_get "$CONFIG_FILE" "validator_auth.mode" "auto")"
KEYSTORE_PASSWORD_FILE_CFG="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_file" "")"
KEYSTORE_PASSWORD_ENV_NAME="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_env" "")"

BLACKLIST_ENABLED="$(cfg_get "$CONFIG_FILE" "blacklist.enabled" "false")"
BLACKLIST_CONTRACT_ADDR="$(cfg_get "$CONFIG_FILE" "blacklist.contract_address" "0x1db0EDE439708A923431DC68fd3F646c0A4D4e6E")"
BLACKLIST_MODE="$(cfg_get "$CONFIG_FILE" "blacklist.mode" "mock")"
BLACKLIST_ALERT_FAIL_OPEN="$(cfg_get "$CONFIG_FILE" "blacklist.alert_fail_open" "true")"
BLACKLIST_REFRESH_SECONDS="$(cfg_get "$CONFIG_FILE" "blacklist.refresh_interval_seconds" "")"

NODE_COUNT="$(cfg_get "$CONFIG_FILE" "network.node_count" "4")"
VALIDATOR_COUNT="$(cfg_get "$CONFIG_FILE" "network.validator_count" "3")"

V1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_http" "18545")"
V1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_ws" "18546")"
V1_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_engine" "18550")"
V1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_p2p" "40401")"

V2_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_http" "18547")"
V2_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_ws" "18548")"
V2_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_engine" "18552")"
V2_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_p2p" "40403")"

V3_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_http" "18549")"
V3_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_ws" "18553")"
V3_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_engine" "18554")"
V3_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_p2p" "40405")"

S1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.sync_http" "18551")"
S1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.sync_ws" "18555")"
S1_ENGINE="$(cfg_get "$CONFIG_FILE" "native.ports.sync_engine" "18556")"
S1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.sync_p2p" "40407")"

normalize_impl() {
  local impl="${1:-}"
  case "$impl" in
    geth|reth) echo "$impl" ;;
    *) die "unsupported runtime implementation: $impl (expected geth|reth)" ;;
  esac
}

resolve_binary() {
  local name="$1"
  shift
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
  local idx="$1"
  local node_cfg="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node${idx}" "")"
  case "$RUNTIME_IMPL_MODE" in
    single)
      normalize_impl "$DEFAULT_RUNTIME_IMPL"
      ;;
    mixed)
      if [[ -n "$node_cfg" ]]; then
        normalize_impl "$node_cfg"
      else
        normalize_impl "$DEFAULT_RUNTIME_IMPL"
      fi
      ;;
    *)
      die "runtime.impl_mode must be single|mixed, got: $RUNTIME_IMPL_MODE"
      ;;
  esac
}

is_true() {
  case "$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

if ! [[ "$NODE_COUNT" =~ ^[0-9]+$ ]] || ! [[ "$VALIDATOR_COUNT" =~ ^[0-9]+$ ]]; then
  die "network.node_count and network.validator_count must be integers"
fi
if (( NODE_COUNT < 1 || VALIDATOR_COUNT < 1 || NODE_COUNT < VALIDATOR_COUNT )); then
  die "invalid node/validator count: node_count=$NODE_COUNT validator_count=$VALIDATOR_COUNT"
fi
if (( NODE_COUNT > 4 )); then
  die "native pm2 runtime currently supports up to 4 nodes, got $NODE_COUNT"
fi

DEFAULT_RUNTIME_IMPL="$(normalize_impl "$DEFAULT_RUNTIME_IMPL")"
case "$VALIDATOR_AUTH_MODE" in
  auto|private_key|keystore) ;;
  *) die "validator_auth.mode must be auto|private_key|keystore, got: $VALIDATOR_AUTH_MODE" ;;
esac

GETH_BINARY=""
RETH_BINARY=""
if ! GETH_BINARY="$(resolve_binary "geth" "$(to_abs_path "$GETH_BINARY_CFG")" "$CHAIN_ROOT/build/bin/geth")"; then
  GETH_BINARY=""
fi
if ! RETH_BINARY="$(resolve_binary "reth" "$(to_abs_path "$RETH_BINARY_CFG")" "$RETH_ROOT/target/release/congress-node" "$RETH_ROOT/target/debug/congress-node")"; then
  RETH_BINARY=""
fi

declare -a NODE_IMPLS=()
need_geth=false
need_reth=false
for ((i=0; i<NODE_COUNT; i++)); do
  impl="$(resolve_node_impl "$i")"
  NODE_IMPLS+=("$impl")
  if [[ "$impl" == "geth" ]]; then
    need_geth=true
  else
    need_reth=true
  fi
done

if $need_geth && [[ -z "$GETH_BINARY" ]]; then
  die "geth binary not found. tried: $(to_abs_path "$GETH_BINARY_CFG") $CHAIN_ROOT/build/bin/geth"
fi
if $need_reth && [[ -z "$RETH_BINARY" ]]; then
  die "reth binary not found. tried: $(to_abs_path "$RETH_BINARY_CFG") $RETH_ROOT/target/release/congress-node $RETH_ROOT/target/debug/congress-node"
fi

[[ -f "$GENESIS_FILE" ]] || die "missing genesis file: $GENESIS_FILE (run make init first)"

mkdir -p "$(dirname "$ENV_FILE")" "$LOG_DIR"

init_geth_node() {
  local datadir="$1"
  local init_args=("--datadir" "$datadir")
  if [[ -n "$STATE_SCHEME" ]]; then
    init_args+=("--state.scheme=$STATE_SCHEME")
  fi
  if [[ ! -d "$datadir/geth/chaindata" ]]; then
    "$GETH_BINARY" "${init_args[@]}" init "$GENESIS_FILE" >/dev/null
  fi
}

init_reth_node() {
  local datadir="$1"
  if [[ ! -d "$datadir/db" ]]; then
    CONGRESS_GENESIS="$GENESIS_FILE" \
    "$RETH_BINARY" init --chain "$GENESIS_FILE" --datadir "$datadir" \
      --log.file.directory "$LOG_DIR" >/dev/null
  fi
}

import_geth_validator_if_needed() {
  local idx="$1"
  local datadir="$DATA_DIR/node$idx"
  local keyfile="$datadir/validator.key"
  local password="$datadir/password.txt"
  local addr
  addr="$(tr -d '[:space:]' < "$datadir/validator.addr")"
  [[ -n "$addr" ]] || die "empty validator addr: $datadir/validator.addr"

  if [[ ! -f "$password" ]]; then
    printf '%s\n' "123456" > "$password"
  fi
  if ! "$GETH_BINARY" account list --datadir "$datadir" 2>/dev/null | grep -qi "${addr#0x}"; then
    "$GETH_BINARY" account import --datadir "$datadir" --password "$password" "$keyfile" >/dev/null
  fi
}

declare -a P2P_PORTS=("$V1_P2P" "$V2_P2P" "$V3_P2P" "$S1_P2P")
declare -a HTTP_PORTS=("$V1_HTTP" "$V2_HTTP" "$V3_HTTP" "$S1_HTTP")
declare -a WS_PORTS=("$V1_WS" "$V2_WS" "$V3_WS" "$S1_WS")
declare -a ENGINE_PORTS=("$V1_ENGINE" "$V2_ENGINE" "$V3_ENGINE" "$S1_ENGINE")

declare -a NODE_PUBS=()
for ((i=0; i<NODE_COUNT; i++)); do
  datadir="$DATA_DIR/node$i"
  [[ -d "$datadir" ]] || die "missing node data dir: $datadir (run make init first)"
  [[ -f "$datadir/nodekey" ]] || die "missing node key: $datadir/nodekey"
  [[ -f "$datadir/nodepub" ]] || die "missing node pub: $datadir/nodepub"
  NODE_PUBS+=("$(tr -d '[:space:]' < "$datadir/nodepub")")

  case "${NODE_IMPLS[$i]}" in
    geth) init_geth_node "$datadir" ;;
    reth) init_reth_node "$datadir" ;;
  esac

done

for ((i=0; i<VALIDATOR_COUNT; i++)); do
  datadir="$DATA_DIR/node$i"
  [[ -f "$datadir/validator.key" ]] || die "missing validator key: $datadir/validator.key"
  [[ -f "$datadir/validator.addr" ]] || die "missing validator addr: $datadir/validator.addr"
  [[ -d "$datadir/keystore" ]] || die "missing validator keystore dir: $datadir/keystore"
  if [[ "${NODE_IMPLS[$i]}" == "geth" ]]; then
    import_geth_validator_if_needed "$i"
  fi
done

BOOTNODES=""
for ((i=0; i<NODE_COUNT; i++)); do
  item="enode://${NODE_PUBS[$i]}@127.0.0.1:${P2P_PORTS[$i]}"
  if [[ -z "$BOOTNODES" ]]; then
    BOOTNODES="$item"
  else
    BOOTNODES+=",$item"
  fi
done

cat > "$ENV_FILE" <<EOF_ENV
GETH_BINARY=$GETH_BINARY
RETH_BINARY=$RETH_BINARY
GENESIS_FILE=$GENESIS_FILE
CONGRESS_GENESIS=$GENESIS_FILE
NETWORK_ID=$NETWORK_ID
BOOTNODES=$BOOTNODES
NATIVE_LOG_DIR=$LOG_DIR
STATE_SCHEME=$STATE_SCHEME
HISTORY_STATE=$HISTORY_STATE
BLACKLIST_ENABLED=$BLACKLIST_ENABLED
BLACKLIST_CONTRACT_ADDR=$BLACKLIST_CONTRACT_ADDR
BLACKLIST_MODE=$BLACKLIST_MODE
BLACKLIST_ALERT_FAIL_OPEN=$BLACKLIST_ALERT_FAIL_OPEN
BLACKLIST_REFRESH_INTERVAL_SECONDS=$BLACKLIST_REFRESH_SECONDS
RUNTIME_IMPL_MODE=$RUNTIME_IMPL_MODE
DEFAULT_RUNTIME_IMPL=$DEFAULT_RUNTIME_IMPL
VALIDATOR_AUTH_MODE=$VALIDATOR_AUTH_MODE
VALIDATOR_COUNT=$VALIDATOR_COUNT
NODE_COUNT=$NODE_COUNT
KEYSTORE_PASSWORD_ENV_NAME=$KEYSTORE_PASSWORD_ENV_NAME

NODE0_IMPL=${NODE_IMPLS[0]:-}
NODE1_IMPL=${NODE_IMPLS[1]:-}
NODE2_IMPL=${NODE_IMPLS[2]:-}
NODE3_IMPL=${NODE_IMPLS[3]:-}

NODE0_DATADIR=$DATA_DIR/node0
NODE1_DATADIR=$DATA_DIR/node1
NODE2_DATADIR=$DATA_DIR/node2
NODE3_DATADIR=$DATA_DIR/node3

NODE0_NODEKEY=$DATA_DIR/node0/nodekey
NODE1_NODEKEY=$DATA_DIR/node1/nodekey
NODE2_NODEKEY=$DATA_DIR/node2/nodekey
NODE3_NODEKEY=$DATA_DIR/node3/nodekey

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
EOF_ENV

for ((i=0; i<VALIDATOR_COUNT; i++)); do
  idx=$((i+1))
  datadir="$DATA_DIR/node$i"
  validator_addr="$(tr -d '[:space:]' < "$datadir/validator.addr")"
  keystore_file="$(find "$datadir/keystore" -type f | head -n 1 || true)"
  if [[ -z "$keystore_file" ]]; then
    die "missing keystore file for validator node$i in $datadir/keystore"
  fi
  keystore_addr=""
  if [[ -f "$datadir/keystore.addr" ]]; then
    keystore_addr="$(tr -d '[:space:]' < "$datadir/keystore.addr")"
  fi

  pass_file="$datadir/password.txt"
  if [[ -n "$KEYSTORE_PASSWORD_FILE_CFG" ]]; then
    pass_file="$(to_abs_path "$KEYSTORE_PASSWORD_FILE_CFG")"
    [[ -f "$pass_file" ]] || die "validator_auth.keystore.password_file not found: $pass_file"
  elif [[ ! -f "$pass_file" ]] && [[ -n "$KEYSTORE_PASSWORD_ENV_NAME" ]]; then
    env_pass="$(printenv "$KEYSTORE_PASSWORD_ENV_NAME" || true)"
    if [[ -n "$env_pass" ]]; then
      printf '%s\n' "$env_pass" > "$pass_file"
    fi
  fi

  cat >> "$ENV_FILE" <<EOF_ENV
VALIDATOR${idx}_ADDRESS=$validator_addr
VALIDATOR${idx}_PASSWORD=$pass_file
VALIDATOR${idx}_PRIVATE_KEY_FILE=$datadir/validator.key
VALIDATOR${idx}_KEYSTORE_DIR=$datadir/keystore
VALIDATOR${idx}_KEYSTORE_PATH=$keystore_file
VALIDATOR${idx}_KEYSTORE_ADDRESS=$keystore_addr
EOF_ENV

done

log "native pm2 env generated: $ENV_FILE"
