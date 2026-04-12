#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/../network/lib.sh"
# shellcheck source=scripts/scenarios/lib.sh
source "$SCRIPT_DIR/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"

gen_addr() {
  local seed="$1"
  (
    cd "$ROOT_DIR"
    go run ./cmd/genkeys "$seed"
  ) | awk -F',' '{print $1}' | tr -d '[:space:]'
}

hardhat_addr() {
  local index="$1"
  (
    cd "$ROOT_DIR"
    go run ./cmd/genhardhat "$index"
  ) | awk -F',' '{print $1}' | tr -d '[:space:]'
}

cleanup() {
  if [[ -f "$SESSION_FILE" ]]; then
    scenario_network down >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

expect_gen_fail() {
  local name="$1"
  shift
  echo "[scenario/negative] expect generation failure :: $name"
  if "$@"; then
    die "$name unexpectedly succeeded"
  fi
}

block_number_hex() {
  local rpc_url="$1"
  local response
  response="$(curl -s --max-time 3 \
    -H 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
    "$rpc_url" || true)"
  python3 - "$response" <<'PY'
import json
import sys

try:
    payload = json.loads(sys.argv[1] or "{}")
except Exception:
    print("")
    raise SystemExit(0)
print(payload.get("result", ""))
PY
}

partial_validator="$(gen_addr "partial-override-validator")"
partial_signer="$(gen_addr "signer-0")"

expect_gen_fail "partial override validators-only" env \
  TEST_ENV_CONFIG="$CONFIG_FILE" \
  GENESIS_MODE=upgrade \
  FORK_TARGET=posaTime \
  TOPOLOGY=single \
  UPGRADE_OVERRIDE_POSA_VALIDATORS="$partial_validator" \
  bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null 2>&1

expect_gen_fail "partial override signers-only" env \
  TEST_ENV_CONFIG="$CONFIG_FILE" \
  GENESIS_MODE=upgrade \
  FORK_TARGET=posaTime \
  TOPOLOGY=single \
  UPGRADE_OVERRIDE_POSA_SIGNERS="$partial_signer" \
  bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null 2>&1

echo "[scenario/negative] fresh PoSA underfunded validator should not pass block 1"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=single \
BOOTSTRAP_SIGNER_MODE=separate \
BOOTSTRAP_VALIDATOR_BALANCE_WEI=0 \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null
scenario_network init

RPC_URL="$(cfg_get "$SESSION_FILE" "native.external_rpc" "http://localhost:18545")"
if scenario_network up; then
  if scenario_network ready; then
    stagnant=true
    for _ in $(seq 1 12); do
      block_hex="$(block_number_hex "$RPC_URL")"
      if [[ -n "$block_hex" && "$block_hex" != "0x0" ]]; then
        stagnant=false
        break
      fi
      sleep 1
    done
    if [[ "$stagnant" != "true" ]]; then
      die "underfunded fresh PoSA unexpectedly progressed beyond genesis"
    fi
  fi
fi
scenario_network down

echo "[scenario/negative] underfunded upgrade should defer migration and keep chain live"
underfunded_validator="$(gen_addr "underfunded-upgrade-validator")"
# PoA -> PoSA migration signers must cover the live POA signer set.
# In the default single-node separated bootstrap layout, that signer is Hardhat index 4.
runtime_signer="$(hardhat_addr 4)"
underfunded_time="$(( $(date +%s) + 45 ))"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=single \
BOOTSTRAP_SIGNER_MODE=separate \
BOOTSTRAP_VALIDATOR_BALANCE_WEI=0 \
GENESIS_MODE=upgrade \
FORK_TARGET=posaTime \
FORK_DELAY_SECONDS=30 \
UPGRADE_OVERRIDE_POSA_TIME="$underfunded_time" \
UPGRADE_OVERRIDE_POSA_VALIDATORS="$underfunded_validator" \
UPGRADE_OVERRIDE_POSA_SIGNERS="$runtime_signer" \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null
scenario_network init
scenario_network up
scenario_network ready
(
  cd "$ROOT_DIR"
  EXPECT_UPGRADE_DEFER=1 go test ./tests/epoch -run TestZ_UnderfundedUpgradeDefersMigration -count=1
)
scenario_network down

echo "[scenario/negative] override drift on restart should be rejected"
stable_validator="$(gen_addr "stable-upgrade-validator")"
stable_time="$(( $(date +%s) + 45 ))"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=single \
BOOTSTRAP_SIGNER_MODE=separate \
GENESIS_MODE=upgrade \
FORK_TARGET=posaTime \
FORK_DELAY_SECONDS=30 \
UPGRADE_OVERRIDE_POSA_TIME="$stable_time" \
UPGRADE_OVERRIDE_POSA_VALIDATORS="$stable_validator" \
UPGRADE_OVERRIDE_POSA_SIGNERS="$runtime_signer" \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null
scenario_network init
scenario_network up
scenario_network ready
(
  cd "$ROOT_DIR"
  go test ./tests/epoch -run TestZ_UpgradeOverrideBootstrapMapping -count=1
)
scenario_network down

drift_validator="$(gen_addr "drift-upgrade-validator")"
if env \
  UPGRADE_OVERRIDE_POSA_VALIDATORS="$drift_validator" \
  UPGRADE_OVERRIDE_POSA_SIGNERS="$runtime_signer" \
  bash "$ROOT_DIR/scripts/network/native_single.sh" up "$SESSION_FILE"; then
  if env \
    UPGRADE_OVERRIDE_POSA_VALIDATORS="$drift_validator" \
    UPGRADE_OVERRIDE_POSA_SIGNERS="$runtime_signer" \
    WAIT_TIMEOUT=10 \
    bash "$ROOT_DIR/scripts/network/native_single.sh" ready "$SESSION_FILE"; then
    (
      cd "$ROOT_DIR"
      EXPECT_OVERRIDE_DRIFT_REJECT=1 \
      DRIFT_OVERRIDE_VALIDATOR="$drift_validator" \
      DRIFT_OVERRIDE_SIGNER="$runtime_signer" \
      go test ./tests/epoch -run 'TestZ_UpgradeOverrideBootstrapMapping|TestZ_OverrideDriftRestartKeepsStoredMapping' -count=1
    )
  fi
fi
bash "$ROOT_DIR/scripts/network/native_single.sh" down "$SESSION_FILE" >/dev/null 2>&1 || true

echo "[scenario/negative] 🟢 PASS"
