#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/../network/lib.sh"
# shellcheck source=scripts/scenarios/lib.sh
source "$SCRIPT_DIR/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"
SCENARIO_EPOCH="${SCENARIO_EPOCH:-300}"
SCENARIO_BOOTSTRAP_TIMEOUT="${SCENARIO_BOOTSTRAP_TIMEOUT:-30}"
SCENARIO_BOOTSTRAP_STABLE_ROUNDS="${SCENARIO_BOOTSTRAP_STABLE_ROUNDS:-3}"

scenario_init "add-validator-punish"

cleanup() {
  local rc=$?
  scenario_cleanup "$rc"
}
trap cleanup EXIT

scenario_mark_stage "startup/bootstrap"
echo "[scenario/add-validator-punish] prepare multi-validator separated-signer network (epoch=$SCENARIO_EPOCH)"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=multi \
BOOTSTRAP_SIGNER_MODE=separate \
TEST_NETWORK_EPOCH="$SCENARIO_EPOCH" \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null

bash "$ROOT_DIR/scripts/network/native.sh" init "$SESSION_FILE"
bash "$ROOT_DIR/scripts/network/native.sh" up "$SESSION_FILE"
bash "$ROOT_DIR/scripts/network/native.sh" ready "$SESSION_FILE"
wait_for_scenario_rpc_stability "$ROOT_DIR/data/test_config.yaml" "$SCENARIO_BOOTSTRAP_TIMEOUT" "$SCENARIO_BOOTSTRAP_STABLE_ROUNDS"

scenario_mark_stage "in-test"
(
  cd "$ROOT_DIR"
  go test ./tests/punish -run TestZ_AddValidatorMissingSignerTriggersPunishAndJail -count=1 -timeout 25m
)

scenario_mark_stage "completed"
bash "$ROOT_DIR/scripts/network/native.sh" down "$SESSION_FILE"

echo "[scenario/add-validator-punish] 🟢 PASS"
