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
SCENARIO_EPOCH="${SCENARIO_EPOCH:-30}"
SCENARIO_BOOTSTRAP_TIMEOUT="${SCENARIO_BOOTSTRAP_TIMEOUT:-30}"
SCENARIO_BOOTSTRAP_STABLE_ROUNDS="${SCENARIO_BOOTSTRAP_STABLE_ROUNDS:-3}"
LIVENESS_REPRO_MAX_ATTEMPTS="${LIVENESS_REPRO_MAX_ATTEMPTS:-10}"
LIVENESS_REPRO_WINDOW_TIMEOUT="${LIVENESS_REPRO_WINDOW_TIMEOUT:-90s}"
LIVENESS_REPRO_FAULT_MODE="${LIVENESS_REPRO_FAULT_MODE:-off}"
LIVENESS_REPRO_CONFIRMATION_PROBES="${LIVENESS_REPRO_CONFIRMATION_PROBES:-3}"
LIVENESS_REPRO_CONFIRMATION_POLL="${LIVENESS_REPRO_CONFIRMATION_POLL:-120ms}"
LIVENESS_REPRO_POST_STOP_PROBES="${LIVENESS_REPRO_POST_STOP_PROBES:-5}"
LIVENESS_REPRO_POST_STOP_POLL="${LIVENESS_REPRO_POST_STOP_POLL:-120ms}"
LIVENESS_REPRO_MIN_SPLIT_ROUNDS="${LIVENESS_REPRO_MIN_SPLIT_ROUNDS:-3}"

scenario_init "liveness-repro"

cleanup() {
  local rc=$?
  scenario_cleanup "$rc"
}
trap cleanup EXIT

scenario_select_case "stop_in_turn_signer_competing_window"
scenario_mark_stage "startup/bootstrap"

echo "[scenario/liveness-repro] prepare multi-validator separated-signer network"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY="multi" \
BOOTSTRAP_SIGNER_MODE=separate \
TEST_NETWORK_EPOCH="$SCENARIO_EPOCH" \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null

scenario_network init
scenario_network up
scenario_network ready
wait_for_scenario_rpc_stability "$ROOT_DIR/data/test_config.yaml" "$SCENARIO_BOOTSTRAP_TIMEOUT" "$SCENARIO_BOOTSTRAP_STABLE_ROUNDS"

scenario_mark_stage "liveness-repro"
go_test_log="${SCENARIO_CASE_DIR}/go_test.log"
(
  cd "$ROOT_DIR"
  echo "[scenario/liveness-repro] attempts=$LIVENESS_REPRO_MAX_ATTEMPTS window_timeout=$LIVENESS_REPRO_WINDOW_TIMEOUT fault_mode=$LIVENESS_REPRO_FAULT_MODE confirmation_probes=$LIVENESS_REPRO_CONFIRMATION_PROBES confirmation_poll=$LIVENESS_REPRO_CONFIRMATION_POLL post_stop_probes=$LIVENESS_REPRO_POST_STOP_PROBES post_stop_poll=$LIVENESS_REPRO_POST_STOP_POLL min_split_rounds=$LIVENESS_REPRO_MIN_SPLIT_ROUNDS" | tee "$go_test_log"
  LIVENESS_REPRO_MAX_ATTEMPTS="$LIVENESS_REPRO_MAX_ATTEMPTS" \
  LIVENESS_REPRO_WINDOW_TIMEOUT="$LIVENESS_REPRO_WINDOW_TIMEOUT" \
  LIVENESS_REPRO_FAULT_MODE="$LIVENESS_REPRO_FAULT_MODE" \
  LIVENESS_REPRO_CONFIRMATION_PROBES="$LIVENESS_REPRO_CONFIRMATION_PROBES" \
  LIVENESS_REPRO_CONFIRMATION_POLL="$LIVENESS_REPRO_CONFIRMATION_POLL" \
  LIVENESS_REPRO_POST_STOP_PROBES="$LIVENESS_REPRO_POST_STOP_PROBES" \
  LIVENESS_REPRO_POST_STOP_POLL="$LIVENESS_REPRO_POST_STOP_POLL" \
  LIVENESS_REPRO_MIN_SPLIT_ROUNDS="$LIVENESS_REPRO_MIN_SPLIT_ROUNDS" \
  go test ./tests/punish -run '^TestZ_Liveness_StopInTurnSigner_DuringCompetingWindow$' -count=1 2>&1 | tee -a "$go_test_log"
)

scenario_mark_stage "completed"
archive_scenario_artifacts "PASS"
scenario_network down

scenario_select_case "main"
scenario_mark_stage "completed"
archive_scenario_artifacts "PASS"

echo "[scenario/liveness-repro] 🟢 PASS"
