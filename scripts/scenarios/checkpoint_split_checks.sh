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
SCENARIO_EPOCH="${SCENARIO_EPOCH:-10}"
SCENARIO_BOOTSTRAP_TIMEOUT="${SCENARIO_BOOTSTRAP_TIMEOUT:-30}"
SCENARIO_BOOTSTRAP_STABLE_ROUNDS="${SCENARIO_BOOTSTRAP_STABLE_ROUNDS:-3}"

scenario_init "checkpoint"

cleanup() {
  local rc=$?
  scenario_cleanup "$rc"
}
trap cleanup EXIT

run_case() {
  local pkg="$1"
  local pattern="$2"
  local topology="${3:-single}"
  local case_label
  local go_test_log
  case_label="$(basename "$pkg")_${pattern}_${topology}"

  scenario_select_case "$case_label"
  scenario_mark_stage "startup/bootstrap"

  echo "[scenario/checkpoint] prepare network for $pkg :: $pattern (topology=$topology)"
  TEST_ENV_CONFIG="$CONFIG_FILE" \
  TOPOLOGY="$topology" \
  BOOTSTRAP_SIGNER_MODE=separate \
  TEST_NETWORK_EPOCH="$SCENARIO_EPOCH" \
  bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null

  scenario_network init
  scenario_network up
  scenario_network ready
  wait_for_scenario_rpc_stability "$ROOT_DIR/data/test_config.yaml" "$SCENARIO_BOOTSTRAP_TIMEOUT" "$SCENARIO_BOOTSTRAP_STABLE_ROUNDS"

  scenario_mark_stage "checkpoint-wait"
  go_test_log="${SCENARIO_CASE_DIR}/go_test.log"
  (
    cd "$ROOT_DIR"
    go test "$pkg" -run "$pattern" -count=1 2>&1 | tee "$go_test_log"
  )

  scenario_mark_stage "completed"
  archive_scenario_artifacts "PASS"
  scenario_network down
}

run_case ./tests/rewards TestZ_CheckpointRuntimeRewardsStillUseOldSigner single
run_case ./tests/punish TestZ_CheckpointRuntimePunishStillUsesOldSigner multi
run_case ./tests/punish TestZ_Liveness_StopOneValidator_RemainingTwoStillSeal multi
run_case ./tests/punish TestZ_ExecutePendingAutoByConsensus multi
run_case ./tests/epoch TestZ_CheckpointTransitionSignerSplit single

scenario_select_case "main"
scenario_mark_stage "completed"
archive_scenario_artifacts "PASS"

echo "[scenario/checkpoint] 🟢 PASS"
