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

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
BACKEND="${RUNTIME_BACKEND:-$(cfg_get "$CONFIG_FILE" "runtime.backend" "native")}"

CHAIN_ROOT_CFG="$(cfg_get "$CONFIG_FILE" "paths.chain_root" "../chain")"
CHAIN_ROOT="$(to_abs_path "$CHAIN_ROOT_CFG")"

CHAIN_CONTRACT_ROOT_CFG="$(cfg_get "$CONFIG_FILE" "paths.chain_contract_root" "../chain-contract")"
CHAIN_CONTRACT_ROOT="$(to_abs_path "$CHAIN_CONTRACT_ROOT_CFG")"

CHAIN_CONTRACT_OUT_CFG="$(cfg_get "$CONFIG_FILE" "paths.chain_contract_out" "")"
if [[ -n "$CHAIN_CONTRACT_OUT_CFG" ]]; then
  CHAIN_CONTRACT_OUT="$(to_abs_path "$CHAIN_CONTRACT_OUT_CFG")"
else
  CHAIN_CONTRACT_OUT="$CHAIN_CONTRACT_ROOT/out"
fi

BYTECODE_GO="$CHAIN_ROOT/consensus/congress/bytecode.go"
CHECK_SCRIPT="$SCRIPT_DIR/check_bytecode_consistency.js"

[[ -f "$CHECK_SCRIPT" ]] || diep "missing checker script: $CHECK_SCRIPT"
[[ -d "$CHAIN_ROOT" ]] || diep "chain root not found: $CHAIN_ROOT"
[[ -f "$BYTECODE_GO" ]] || diep "missing consensus bytecode file: $BYTECODE_GO"
[[ -d "$CHAIN_CONTRACT_OUT" ]] || diep "chain-contract out dir not found: $CHAIN_CONTRACT_OUT"

command -v node >/dev/null 2>&1 || diep "node is required for bytecode consistency checks"

logp "checking bytecode consistency..."
node "$CHECK_SCRIPT" --out-dir "$CHAIN_CONTRACT_OUT" --bytecode-go "$BYTECODE_GO"

if [[ "$BACKEND" == "native" ]]; then
  GETH_BIN_CFG="$(cfg_get "$CONFIG_FILE" "native.geth_binary" "")"
  CANDIDATES=()
  if [[ -n "$GETH_BIN_CFG" ]]; then
    CANDIDATES+=("$(to_abs_path "$GETH_BIN_CFG")")
  fi
  CANDIDATES+=("$CHAIN_ROOT/build/bin/geth")

  GETH_BIN=""
  for candidate in "${CANDIDATES[@]}"; do
    if [[ -x "$candidate" ]]; then
      GETH_BIN="$candidate"
      break
    fi
  done
  if [[ -z "$GETH_BIN" ]]; then
    diep "native.geth_binary not found. Tried: ${CANDIDATES[*]}"
  fi

  newest_ref="$BYTECODE_GO"
  newest_ts="$(file_mtime "$BYTECODE_GO")"
  for contract in Validators Proposal Punish Staking; do
    artifact="$CHAIN_CONTRACT_OUT/${contract}.sol/${contract}.json"
    [[ -f "$artifact" ]] || diep "missing artifact: $artifact"
    artifact_ts="$(file_mtime "$artifact")"
    if (( artifact_ts > newest_ts )); then
      newest_ts="$artifact_ts"
      newest_ref="$artifact"
    fi
  done

  geth_ts="$(file_mtime "$GETH_BIN")"
  if (( geth_ts < newest_ts )); then
    diep "native geth binary is older than $newest_ref. Rebuild with: make -C $CHAIN_ROOT geth"
  fi
fi

logp "environment consistency OK."
