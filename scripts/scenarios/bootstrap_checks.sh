#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/../network/lib.sh"
# shellcheck source=scripts/scenarios/lib.sh
source "$SCRIPT_DIR/lib.sh"
ensure_go_build_env

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"

cleanup() {
  if [[ -f "$SESSION_FILE" ]]; then
    scenario_network down >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

validate_mode() {
  local expected_mode="$1"
  local expected_same="$2"
  local expected_single="$3"
  python3 - "$ROOT_DIR/data/test_config.yaml" "$ROOT_DIR/data/genesis.json" "$expected_mode" "$expected_same" "$expected_single" <<'PY'
import json
import sys

import yaml

cfg_path, genesis_path, expected_mode, expected_same, expected_single = sys.argv[1:]
want_same = expected_same == "true"
want_single = expected_single == "true"

expected_funder = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
expected_validators = [
    "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
    "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc",
    "0x90f79bf6eb2c4f870365e785982e1f101e93b906",
]
expected_separate_signers = [
    "0x15d34aaf54267db7d7c367839aaf71a00a2c6a65",
    "0x9965507d1a55bcc2695c58ba16fb37d819b0a4dc",
    "0x976ea74026e726554db657fa54763abd0c3a0aa9",
]

with open(cfg_path, "r", encoding="utf-8") as fh:
    cfg = yaml.safe_load(fh) or {}
with open(genesis_path, "r", encoding="utf-8") as fh:
    genesis = json.load(fh)

funder = ((cfg.get("funder") or {}).get("address") or "").lower()
if funder != expected_funder:
    raise SystemExit(f"unexpected funder address: {funder} != {expected_funder}")

validators = cfg.get("validators") or []
if want_single and len(validators) != 1:
    raise SystemExit(f"expected single-topology validator count=1, got {len(validators)}")
if not validators:
    raise SystemExit("generated validators list is empty")
if len(validators) > len(expected_validators):
    raise SystemExit(f"unexpected validator count for fixed bootstrap mapping: {len(validators)}")

for idx, item in enumerate(validators):
    validator = (item.get("address") or "").lower()
    signer = (item.get("signer_address") or "").lower()
    if not validator or not signer:
        raise SystemExit("validator or signer address missing in test_config.yaml")
    expected_validator = expected_validators[idx]
    if validator != expected_validator:
        raise SystemExit(f"validator[{idx}] mismatch: {validator} != {expected_validator}")
    if want_same and validator != signer:
        raise SystemExit(f"expected same-address mode but got validator={validator} signer={signer}")
    if not want_same and validator == signer:
        raise SystemExit(f"expected separate signer mode but got same address {validator}")
    if want_same and signer != expected_validator:
        raise SystemExit(f"same-address signer[{idx}] mismatch: {signer} != {expected_validator}")
    if not want_same:
        expected_signer = expected_separate_signers[idx]
        if signer != expected_signer:
            raise SystemExit(f"signer[{idx}] mismatch: {signer} != {expected_signer}")

congress = (genesis.get("config") or {}).get("congress") or {}
initial_validators = congress.get("initialValidators") or []
initial_signers = congress.get("initialSigners") or []
if len(initial_validators) != len(validators):
    raise SystemExit(f"initialValidators length mismatch: {len(initial_validators)} != {len(validators)}")
if len(initial_signers) != len(validators):
    raise SystemExit(f"initialSigners length mismatch: {len(initial_signers)} != {len(validators)}")

extra = genesis.get("extraData") or ""
if not isinstance(extra, str) or not extra.startswith("0x"):
    raise SystemExit("genesis.extraData is missing or invalid")
hexdata = extra[2:]
signer_hex = hexdata[64:-130]
extra_signers = [f"0x{signer_hex[i:i+40]}".lower() for i in range(0, len(signer_hex), 40) if signer_hex[i:i+40]]
expected_signers = sorted(addr.lower() for addr in initial_signers)
if sorted(extra_signers) != expected_signers:
    raise SystemExit(f"extraData signer set mismatch: {extra_signers} != {expected_signers}")

session_mode = (((cfg.get("runtime") or {}).get("backend")) or "").strip()
if not session_mode:
    raise SystemExit("runtime backend missing in test_config.yaml")

print(f"validated bootstrap mode={expected_mode} validators={len(validators)}")
PY
}

echo "[scenario/bootstrap] generate same-address single-topology config"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=single \
BOOTSTRAP_SIGNER_MODE=same_address \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null
validate_mode "same_address" "true" "true"

echo "[scenario/bootstrap] generate separate-signer single-topology config"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=single \
BOOTSTRAP_SIGNER_MODE=separate \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null
validate_mode "separate" "false" "true"

echo "[scenario/bootstrap] generate separate-signer multi-topology config"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=multi \
BOOTSTRAP_SIGNER_MODE=separate \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null
validate_mode "separate" "false" "false"

echo "[scenario/bootstrap] validate native main path delegates single topology"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=single \
BOOTSTRAP_SIGNER_MODE=separate \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null
scenario_network init
scenario_network up
scenario_network ready

RPC_URL="$(cfg_get "$SESSION_FILE" "native.external_rpc" "http://localhost:18545")"
RESP="$(curl -s --max-time 5 -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' "$RPC_URL" || true)"
if [[ "$RESP" != *'"result":"0x'* ]]; then
  die "bootstrap scenario failed to query block number from $RPC_URL"
fi

echo "[scenario/bootstrap] 🟢 PASS"
