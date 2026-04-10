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
NODE0_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node0.impl" "")"
NODE1_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node1.impl" "")"
NODE2_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node2.impl" "")"
NODE3_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node3.impl" "")"
NODE0_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node0.binary" "")"
NODE1_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node1.binary" "")"
NODE2_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node2.binary" "")"
NODE3_BINARY_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node3.binary" "")"
VALIDATOR_AUTH_MODE="$(cfg_get "$CONFIG_FILE" "validator_auth.mode" "auto")"
KEYSTORE_PASSWORD_FILE_CFG="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_file" "")"
KEYSTORE_PASSWORD_ENV_NAME="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_env" "")"

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
  local node_cfg="$2"
  case "$RUNTIME_IMPL_MODE" in
    single)
      normalize_impl "$DEFAULT_RUNTIME_IMPL"
      ;;
    mixed)
      [[ -n "$node_cfg" ]] || die "runtime_nodes.node${idx}.impl is required when runtime.impl_mode=mixed"
      normalize_impl "$node_cfg"
      ;;
    *)
      die "runtime.impl_mode must be single|mixed, got: $RUNTIME_IMPL_MODE"
      ;;
  esac
}

resolve_node_binary() {
  local impl="$1"
  local node_binary_cfg="$2"
  if [[ -n "$node_binary_cfg" ]]; then
    to_abs_path "$node_binary_cfg"
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

is_true() {
  case "$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

coverage_enabled() {
  is_true "${CHAIN_COVERAGE:-0}"
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
  auto|keystore) ;;
  *) die "validator_auth.mode must be auto|keystore, got: $VALIDATOR_AUTH_MODE" ;;
esac
if [[ -n "$UPGRADE_OVERRIDE_POSA_TIME" ]] && ! [[ "$UPGRADE_OVERRIDE_POSA_TIME" =~ ^[0-9]+$ ]]; then
  die "fork.override.posa_time must be an unsigned integer timestamp, got: $UPGRADE_OVERRIDE_POSA_TIME"
fi

UPGRADE_OVERRIDE_POSA_VALIDATORS="$(json_addresses_to_csv "$UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON")"
UPGRADE_OVERRIDE_POSA_SIGNERS="$(json_addresses_to_csv "$UPGRADE_OVERRIDE_POSA_SIGNERS_JSON")"
if [[ -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS" && -z "$UPGRADE_OVERRIDE_POSA_SIGNERS" ]] || [[ -z "$UPGRADE_OVERRIDE_POSA_VALIDATORS" && -n "$UPGRADE_OVERRIDE_POSA_SIGNERS" ]]; then
  die "fork.override.posa_validators and fork.override.posa_signers must be provided together"
fi

declare -a NODE_IMPLS=()
declare -a NODE_BINARIES=()
need_geth=false
need_reth=false
NODE_IMPL_CONFIGS=("$NODE0_IMPL_CFG" "$NODE1_IMPL_CFG" "$NODE2_IMPL_CFG" "$NODE3_IMPL_CFG")
NODE_BINARY_CONFIGS=("$NODE0_BINARY_CFG" "$NODE1_BINARY_CFG" "$NODE2_BINARY_CFG" "$NODE3_BINARY_CFG")
for ((i=0; i<NODE_COUNT; i++)); do
  impl_cfg=""
  binary_cfg=""
  if (( i < ${#NODE_IMPL_CONFIGS[@]} )); then
    impl_cfg="${NODE_IMPL_CONFIGS[$i]}"
  fi
  if (( i < ${#NODE_BINARY_CONFIGS[@]} )); then
    binary_cfg="${NODE_BINARY_CONFIGS[$i]}"
  fi
  impl="$(resolve_node_impl "$i" "$impl_cfg")"
  binary="$(resolve_node_binary "$impl" "$binary_cfg")"
  NODE_IMPLS+=("$impl")
  NODE_BINARIES+=("$binary")
  if [[ "$impl" == "geth" ]]; then
    need_geth=true
  else
    need_reth=true
  fi
done

if [[ -n "$UPGRADE_OVERRIDE_POSA_TIME" || -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS" || -n "$UPGRADE_OVERRIDE_POSA_SIGNERS" ]]; then
  for impl in "${NODE_IMPLS[@]}"; do
    if [[ "$impl" == "reth" ]]; then
      die "upgrade override currently supports geth runtime only; reth node detected"
    fi
  done
fi

if coverage_enabled && $need_reth; then
  die "CHAIN_COVERAGE=1 only supports native geth; reth or mixed runtime detected"
fi

GETH_BINARY=""
RETH_BINARY=""
if $need_geth; then
  coverage_geth_binary=""
  if coverage_enabled; then
    if [[ -n "${CHAIN_COVERAGE_GETH_BINARY:-}" ]]; then
      coverage_geth_binary="$(to_abs_path "$CHAIN_COVERAGE_GETH_BINARY")"
    else
      coverage_geth_binary="$("$SCRIPT_DIR/../coverage/prepare_chain_coverage.sh" --config "$CONFIG_FILE" --print-binary)"
    fi
  fi
  if ! GETH_BINARY="$(resolve_binary "geth" "$coverage_geth_binary" "$(to_abs_path "$GETH_BINARY_CFG")" "$CHAIN_ROOT/build/bin/geth")"; then
    GETH_BINARY=""
  fi
fi
if $need_reth; then
  if ! RETH_BINARY="$(resolve_binary "reth" "$(to_abs_path "$RETH_BINARY_CFG")" "$RETH_ROOT/target/release/congress-node" "$RETH_ROOT/target/debug/congress-node")"; then
    RETH_BINARY=""
  fi
fi

if $need_geth && [[ -z "$GETH_BINARY" ]]; then
  die "geth binary not found. tried: ${coverage_geth_binary:-<none>} $(to_abs_path "$GETH_BINARY_CFG") $CHAIN_ROOT/build/bin/geth"
fi
if $need_reth && [[ -z "$RETH_BINARY" ]]; then
  die "reth binary not found. tried: $(to_abs_path "$RETH_BINARY_CFG") $RETH_ROOT/target/release/congress-node $RETH_ROOT/target/debug/congress-node"
fi

for ((i=0; i<NODE_COUNT; i++)); do
  node_binary="${NODE_BINARIES[$i]}"
  [[ -n "$node_binary" ]] || die "resolved empty binary for node$i"
  [[ -x "$node_binary" ]] || die "node$i binary is not executable: $node_binary"
done

[[ -f "$GENESIS_FILE" ]] || die "missing genesis file: $GENESIS_FILE (run make init first)"

mkdir -p "$(dirname "$ENV_FILE")" "$LOG_DIR"

init_geth_node() {
  local binary="$1"
  local datadir="$2"
  local init_args=("--datadir" "$datadir")
  if [[ -n "$STATE_SCHEME" ]]; then
    init_args+=("--state.scheme=$STATE_SCHEME")
  fi
  if [[ ! -d "$datadir/geth/chaindata" ]]; then
    "$binary" "${init_args[@]}" init "$GENESIS_FILE" >/dev/null
  fi
}

init_reth_node() {
  local binary="$1"
  local datadir="$2"
  if [[ ! -d "$datadir/db" ]]; then
    CONGRESS_GENESIS="$GENESIS_FILE" \
    "$binary" init --chain "$GENESIS_FILE" --datadir "$datadir" \
      --log.file.directory "$LOG_DIR" >/dev/null
  fi
}

import_geth_validator_if_needed() {
  local idx="$1"
  local binary="$2"
  local datadir="$DATA_DIR/node$idx"
  local keyfile="$datadir/signer.key"
  if [[ ! -f "$keyfile" ]]; then
    keyfile="$datadir/validator.key"
  fi
  local password="$datadir/password.txt"
  local addr_file="$datadir/signer.addr"
  if [[ ! -f "$addr_file" ]]; then
    addr_file="$datadir/validator.addr"
  fi
  local addr
  addr="$(tr -d '[:space:]' < "$addr_file")"
  [[ -n "$addr" ]] || die "empty validator addr: $datadir/validator.addr"

  if [[ ! -f "$password" ]]; then
    printf '%s\n' "123456" > "$password"
  fi
  if ! "$binary" account list --datadir "$datadir" 2>/dev/null | grep -qi "${addr#0x}"; then
    "$binary" account import --datadir "$datadir" --password "$password" "$keyfile" >/dev/null
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
    geth) init_geth_node "${NODE_BINARIES[$i]}" "$datadir" ;;
    reth) init_reth_node "${NODE_BINARIES[$i]}" "$datadir" ;;
  esac

done

for ((i=0; i<VALIDATOR_COUNT; i++)); do
  datadir="$DATA_DIR/node$i"
  [[ -f "$datadir/validator.key" ]] || die "missing validator key: $datadir/validator.key"
  [[ -f "$datadir/validator.addr" ]] || die "missing validator addr: $datadir/validator.addr"
  [[ -d "$datadir/keystore" ]] || die "missing validator keystore dir: $datadir/keystore"
  if [[ "${NODE_IMPLS[$i]}" == "geth" ]]; then
    import_geth_validator_if_needed "$i" "${NODE_BINARIES[$i]}"
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

CHAIN_COVERAGE_ENABLED=0
CHAIN_COVERAGE_SCOPE_VALUE=""
COVERAGE_RAW_ROOT=""
if coverage_enabled; then
  CHAIN_COVERAGE_ENABLED=1
  CHAIN_COVERAGE_SCOPE_VALUE="${CHAIN_COVERAGE_SCOPE:-congress}"
  COVERAGE_RAW_ROOT="$("$SCRIPT_DIR/../coverage/prepare_chain_coverage.sh" --config "$CONFIG_FILE" --print-raw-root)"
  mkdir -p "$COVERAGE_RAW_ROOT"
  for ((i=0; i<NODE_COUNT; i++)); do
    mkdir -p "$COVERAGE_RAW_ROOT/node$i"
  done
fi

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
UPGRADE_OVERRIDE_POSA_TIME=$UPGRADE_OVERRIDE_POSA_TIME
UPGRADE_OVERRIDE_POSA_VALIDATORS=$UPGRADE_OVERRIDE_POSA_VALIDATORS
UPGRADE_OVERRIDE_POSA_SIGNERS=$UPGRADE_OVERRIDE_POSA_SIGNERS
RUNTIME_IMPL_MODE=$RUNTIME_IMPL_MODE
DEFAULT_RUNTIME_IMPL=$DEFAULT_RUNTIME_IMPL
VALIDATOR_AUTH_MODE=$VALIDATOR_AUTH_MODE
VALIDATOR_COUNT=$VALIDATOR_COUNT
NODE_COUNT=$NODE_COUNT
KEYSTORE_PASSWORD_ENV_NAME=$KEYSTORE_PASSWORD_ENV_NAME
CHAIN_COVERAGE_ENABLED=$CHAIN_COVERAGE_ENABLED
CHAIN_COVERAGE_SCOPE=$CHAIN_COVERAGE_SCOPE_VALUE

NODE0_IMPL=${NODE_IMPLS[0]:-}
NODE1_IMPL=${NODE_IMPLS[1]:-}
NODE2_IMPL=${NODE_IMPLS[2]:-}
NODE3_IMPL=${NODE_IMPLS[3]:-}
NODE0_BINARY=${NODE_BINARIES[0]:-}
NODE1_BINARY=${NODE_BINARIES[1]:-}
NODE2_BINARY=${NODE_BINARIES[2]:-}
NODE3_BINARY=${NODE_BINARIES[3]:-}

NODE0_DATADIR=$DATA_DIR/node0
NODE1_DATADIR=$DATA_DIR/node1
NODE2_DATADIR=$DATA_DIR/node2
NODE3_DATADIR=$DATA_DIR/node3

NODE0_NODEKEY=$DATA_DIR/node0/nodekey
NODE1_NODEKEY=$DATA_DIR/node1/nodekey
NODE2_NODEKEY=$DATA_DIR/node2/nodekey
NODE3_NODEKEY=$DATA_DIR/node3/nodekey
NODE0_GOCOVERDIR=${COVERAGE_RAW_ROOT:+$COVERAGE_RAW_ROOT/node0}
NODE1_GOCOVERDIR=${COVERAGE_RAW_ROOT:+$COVERAGE_RAW_ROOT/node1}
NODE2_GOCOVERDIR=${COVERAGE_RAW_ROOT:+$COVERAGE_RAW_ROOT/node2}
NODE3_GOCOVERDIR=${COVERAGE_RAW_ROOT:+$COVERAGE_RAW_ROOT/node3}

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
  signer_addr_file="$datadir/signer.addr"
  signer_key_file="$datadir/signer.key"
  if [[ ! -f "$signer_addr_file" ]]; then
    signer_addr_file="$datadir/validator.addr"
  fi
  if [[ ! -f "$signer_key_file" ]]; then
    signer_key_file="$datadir/validator.key"
  fi
  validator_addr="$(tr -d '[:space:]' < "$signer_addr_file")"
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
VALIDATOR${idx}_KEYSTORE_DIR=$datadir/keystore
VALIDATOR${idx}_KEYSTORE_PATH=$keystore_file
VALIDATOR${idx}_KEYSTORE_ADDRESS=$keystore_addr
EOF_ENV

done

log "native pm2 env generated: $ENV_FILE"
