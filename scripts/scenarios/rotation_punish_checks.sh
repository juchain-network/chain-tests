#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/../network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"
SCENARIO_EPOCH="${SCENARIO_EPOCH:-240}"

cleanup() {
  if [[ -f "$SESSION_FILE" ]]; then
    bash "$ROOT_DIR/scripts/network/native.sh" down "$SESSION_FILE" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "[scenario/rotation-punish] prepare multi-validator separated-signer network (epoch=$SCENARIO_EPOCH)"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=multi \
BOOTSTRAP_SIGNER_MODE=separate \
TEST_NETWORK_EPOCH="$SCENARIO_EPOCH" \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null

bash "$ROOT_DIR/scripts/network/native.sh" init "$SESSION_FILE"
bash "$ROOT_DIR/scripts/network/native.sh" up "$SESSION_FILE"
bash "$ROOT_DIR/scripts/network/native.sh" ready "$SESSION_FILE"

(
  cd "$ROOT_DIR"
  go test ./tests/punish -run TestZ_SignerRotationMissingNewSignerTriggersPunishAndJail -count=1 -timeout 20m
)

bash "$ROOT_DIR/scripts/network/native.sh" down "$SESSION_FILE"

echo "[scenario/rotation-punish] 🟢 PASS"
