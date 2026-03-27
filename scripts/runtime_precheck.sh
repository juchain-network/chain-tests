#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/network/lib.sh"

logp() {
  printf '[runtime-precheck] %s\n' "$*"
}

diep() {
  printf '[runtime-precheck] ERROR: %s\n' "$*" >&2
  exit 1
}

file_mtime() {
  local file="$1"
  if stat -f %m "$file" >/dev/null 2>&1; then
    stat -f %m "$file"
  else
    stat -c %Y "$file"
  fi
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

normalize_impl() {
  local impl="${1:-}"
  case "$impl" in
    geth|reth) echo "$impl" ;;
    *) diep "unsupported runtime impl: $impl (expected geth|reth)" ;;
  esac
}

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"
SOURCE_FILE="$CONFIG_FILE"
if [[ -f "$SESSION_FILE" ]]; then
  SOURCE_FILE="$SESSION_FILE"
fi
if [[ "${RUNTIME_SESSION_REQUIRED:-}" == "1" && ! -f "$SESSION_FILE" ]]; then
  diep "runtime session not found: $SESSION_FILE. Run 'make init' first."
fi

BACKEND="${RUNTIME_BACKEND:-$(cfg_get "$SOURCE_FILE" "runtime.backend" "native")}"
GENESIS_MODE="${GENESIS_MODE:-$(cfg_get "$SOURCE_FILE" "network.genesis_mode" "posa")}"

CHAIN_ROOT_CFG="$(cfg_get "$SOURCE_FILE" "paths.chain_root" "../chain")"
CHAIN_ROOT="$(to_abs_path "$CHAIN_ROOT_CFG")"
RETH_ROOT_CFG="$(cfg_get "$SOURCE_FILE" "paths.reth_root" "../rchain")"
RETH_ROOT="$(to_abs_path "$RETH_ROOT_CFG")"
RETH_BYTECODE_FILE_CFG="$(cfg_get "$SOURCE_FILE" "paths.reth_bytecode_file" "")"
if [[ -n "$RETH_BYTECODE_FILE_CFG" ]]; then
  RETH_BYTECODE_FILE="$(to_abs_path "$RETH_BYTECODE_FILE_CFG")"
else
  RETH_BYTECODE_FILE="$RETH_ROOT/crates/congress-core/src/bytecode.rs"
fi

CHAIN_CONTRACT_ROOT_CFG="$(cfg_get "$SOURCE_FILE" "paths.chain_contract_root" "../chain-contract")"
CHAIN_CONTRACT_ROOT="$(to_abs_path "$CHAIN_CONTRACT_ROOT_CFG")"

CHAIN_CONTRACT_OUT_CFG="$(cfg_get "$SOURCE_FILE" "paths.chain_contract_out" "")"
if [[ -n "$CHAIN_CONTRACT_OUT_CFG" ]]; then
  CHAIN_CONTRACT_OUT="$(to_abs_path "$CHAIN_CONTRACT_OUT_CFG")"
else
  CHAIN_CONTRACT_OUT="$CHAIN_CONTRACT_ROOT/out"
fi

BYTECODE_GO="$CHAIN_ROOT/consensus/congress/bytecode.go"
CONGRESS_GO="$CHAIN_ROOT/consensus/congress/congress.go"
CONGRESS_ABI_GO="$CHAIN_ROOT/consensus/congress/abi.go"
CHECK_SCRIPT="$SCRIPT_DIR/check_bytecode_consistency.js"

RUNTIME_IMPL_MODE="$(cfg_get "$SOURCE_FILE" "runtime.impl_mode" "single")"
DEFAULT_RUNTIME_IMPL="$(normalize_impl "$(cfg_get "$SOURCE_FILE" "runtime.impl" "geth")")"
NODE_COUNT="$(cfg_get "$SOURCE_FILE" "network.node_count" "4")"

if ! [[ "$NODE_COUNT" =~ ^[0-9]+$ ]] || (( NODE_COUNT < 1 || NODE_COUNT > 4 )); then
  diep "invalid network.node_count: $NODE_COUNT"
fi

need_geth=false
need_reth=false
for ((i=0; i<NODE_COUNT; i++)); do
  node_cfg="$(cfg_get "$SOURCE_FILE" "runtime_nodes.node${i}" "")"
  case "$RUNTIME_IMPL_MODE" in
    single)
      impl="$DEFAULT_RUNTIME_IMPL"
      ;;
    mixed)
      if [[ -n "$node_cfg" ]]; then
        impl="$(normalize_impl "$node_cfg")"
      else
        impl="$DEFAULT_RUNTIME_IMPL"
      fi
      ;;
    *)
      diep "runtime.impl_mode must be single|mixed, got: $RUNTIME_IMPL_MODE"
      ;;
  esac

  if [[ "$impl" == "geth" ]]; then
    need_geth=true
  else
    need_reth=true
  fi
done

[[ -f "$CHECK_SCRIPT" ]] || diep "missing checker script: $CHECK_SCRIPT"
if $need_geth; then
  [[ -d "$CHAIN_ROOT" ]] || diep "chain root not found: $CHAIN_ROOT"
fi
if $need_reth; then
  [[ -d "$RETH_ROOT" ]] || diep "reth root not found: $RETH_ROOT"
fi

bytecode_impl=""
if $need_geth && $need_reth; then
  bytecode_impl="mixed"
elif $need_geth; then
  bytecode_impl="geth"
elif $need_reth; then
  bytecode_impl="reth"
fi

if [[ "$GENESIS_MODE" == "posa" ]]; then
  [[ -d "$CHAIN_CONTRACT_OUT" ]] || diep "chain-contract out dir not found: $CHAIN_CONTRACT_OUT"
  command -v node >/dev/null 2>&1 || diep "node is required for bytecode consistency checks"

  check_args=(--out-dir "$CHAIN_CONTRACT_OUT" --impl "$bytecode_impl")
  if $need_geth; then
    [[ -f "$BYTECODE_GO" ]] || diep "missing consensus bytecode file: $BYTECODE_GO"
    check_args+=(--bytecode-go "$BYTECODE_GO")
  fi
  if $need_reth; then
    [[ -f "$RETH_BYTECODE_FILE" ]] || diep "missing reth bytecode file: $RETH_BYTECODE_FILE"
    check_args+=(--bytecode-rs "$RETH_BYTECODE_FILE")
  fi

  logp "checking bytecode consistency (impl=$bytecode_impl)..."
  node "$CHECK_SCRIPT" "${check_args[@]}"
else
  logp "skip bytecode consistency check for genesis_mode=$GENESIS_MODE"
fi

if [[ "$BACKEND" == "native" ]]; then
  GETH_BIN_CFG="$(cfg_get "$SOURCE_FILE" "binaries.geth_native" "$(cfg_get "$SOURCE_FILE" "native.geth_binary" "")")"
  RETH_BIN_CFG="$(cfg_get "$SOURCE_FILE" "binaries.reth_native" "$(cfg_get "$SOURCE_FILE" "native.reth_binary" "")")"

  GETH_BIN=""
  RETH_BIN=""
  if $need_geth; then
    if ! GETH_BIN="$(resolve_binary "$(to_abs_path "$GETH_BIN_CFG")" "$CHAIN_ROOT/build/bin/geth")"; then
      diep "native geth binary not found. tried: $(to_abs_path "$GETH_BIN_CFG") $CHAIN_ROOT/build/bin/geth"
    fi
  fi
  if $need_reth; then
    if ! RETH_BIN="$(resolve_binary "$(to_abs_path "$RETH_BIN_CFG")" "$RETH_ROOT/target/release/congress-node" "$RETH_ROOT/target/debug/congress-node")"; then
      diep "native reth binary not found. tried: $(to_abs_path "$RETH_BIN_CFG") $RETH_ROOT/target/release/congress-node $RETH_ROOT/target/debug/congress-node"
    fi
  fi

  if [[ "$GENESIS_MODE" == "posa" ]]; then
    newest_artifact_ref=""
    newest_artifact_ts=0
    for contract in Validators Proposal Punish Staking; do
      artifact="$CHAIN_CONTRACT_OUT/${contract}.sol/${contract}.json"
      [[ -f "$artifact" ]] || diep "missing artifact: $artifact"
      artifact_ts="$(file_mtime "$artifact")"
      if (( artifact_ts > newest_artifact_ts )); then
        newest_artifact_ts="$artifact_ts"
        newest_artifact_ref="$artifact"
      fi
    done

    if $need_geth; then
      newest_ref="$BYTECODE_GO"
      newest_ts="$(file_mtime "$BYTECODE_GO")"
      if (( newest_artifact_ts > newest_ts )); then
        newest_ts="$newest_artifact_ts"
        newest_ref="$newest_artifact_ref"
      fi
      geth_ts="$(file_mtime "$GETH_BIN")"
      if (( geth_ts < newest_ts )); then
        diep "native geth binary is older than $newest_ref. rebuild geth binary."
      fi
    fi

    if $need_reth; then
      newest_ref="$RETH_BYTECODE_FILE"
      newest_ts="$(file_mtime "$RETH_BYTECODE_FILE")"
      if (( newest_artifact_ts > newest_ts )); then
        newest_ts="$newest_artifact_ts"
        newest_ref="$newest_artifact_ref"
      fi
      reth_ts="$(file_mtime "$RETH_BIN")"
      if (( reth_ts < newest_ts )); then
        diep "native reth binary is older than $newest_ref. rebuild reth binary."
      fi
    fi
  fi

  if $need_geth; then
    newest_geth_source_ref=""
    newest_geth_source_ts=0
    for geth_source in \
      "$CONGRESS_GO" \
      "$CONGRESS_ABI_GO" \
      "$BYTECODE_GO"; do
      [[ -f "$geth_source" ]] || diep "missing geth consensus source file: $geth_source"
      source_ts="$(file_mtime "$geth_source")"
      if (( source_ts > newest_geth_source_ts )); then
        newest_geth_source_ts="$source_ts"
        newest_geth_source_ref="$geth_source"
      fi
    done

    geth_ts="$(file_mtime "$GETH_BIN")"
    if (( geth_ts < newest_geth_source_ts )); then
      diep "native geth binary is older than $newest_geth_source_ref. rebuild geth binary."
    fi
  fi
fi

if [[ "$BACKEND" == "docker" ]] && $need_reth; then
  diep "docker backend currently supports geth runtime only; set runtime.impl=geth or switch runtime.backend=native for reth"
fi

logp "environment consistency OK."
