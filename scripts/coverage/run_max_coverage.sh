#!/usr/bin/env bash
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

CONFIG_FILE="${TEST_ENV_CONFIG:-$ROOT_DIR/config/test_env.yaml}"
if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "[coverage] ERROR: test env config not found: $CONFIG_FILE" >&2
  exit 2
fi

RUN_ID="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="${COVERAGE_OUT_DIR:-$ROOT_DIR/reports/max_coverage_${RUN_ID}}"
STEP_TIMEOUT="${COVERAGE_STEP_TIMEOUT:-2h}"
HARD_RESET="${COVERAGE_HARD_RESET:-1}"
EXIT_POLICY="${COVERAGE_EXIT_POLICY:-always_zero}"
INCLUDE_PERF_TIERS="${COVERAGE_INCLUDE_PERF_TIERS:-0}"

mkdir -p "$OUT_DIR/logs"
STEPS_FILE="$OUT_DIR/steps.tsv"

printf 'step\tcategory\tstatus\trc\tstarted_at\tfinished_at\tduration_sec\tlog_file\treport_dir\tcommand\n' > "$STEPS_FILE"

TOTAL=0
PASS=0
FAIL=0
TIMEOUT=0
INFRA_ERR=0
INTERRUPTED=0
CURRENT_STEP=""
CURRENT_STEP_LOG=""
CURRENT_CHILD_PID=""
CURRENT_CHILD_PGID=""

bool_true() {
  case "$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

status_display() {
  case "${1:-}" in
    PASS) printf '🟢 PASS' ;;
    FAIL) printf '🔴 FAIL' ;;
    TIMEOUT) printf '🟠 TIMEOUT' ;;
    SKIP) printf '🟡 SKIP' ;;
    *) printf '%s' "${1:-UNKNOWN}" ;;
  esac
}

cleanup_step_process() {
  local root_pid="$CURRENT_CHILD_PID"
  [[ -n "$root_pid" ]] || return 0

  local -a descendants
  local parent child
  descendants=()

  # Collect descendant pids iteratively (children, grandchildren, ...)
  local -a queue
  queue=("$root_pid")
  while [[ "${#queue[@]}" -gt 0 ]]; do
    parent="${queue[0]}"
    queue=("${queue[@]:1}")
    while read -r child; do
      [[ -n "$child" ]] || continue
      descendants+=("$child")
      queue+=("$child")
    done < <(ps -eo pid=,ppid= | awk -v p="$parent" '$2==p {print $1}')
  done

  # TERM descendants first, then root.
  for child in "${descendants[@]}"; do
    kill -TERM "$child" >/dev/null 2>&1 || true
  done
  kill -TERM "$root_pid" >/dev/null 2>&1 || true

  sleep 1

  # Escalate to KILL if still alive.
  for child in "${descendants[@]}"; do
    kill -0 "$child" >/dev/null 2>&1 && kill -KILL "$child" >/dev/null 2>&1 || true
  done
  kill -0 "$root_pid" >/dev/null 2>&1 && kill -KILL "$root_pid" >/dev/null 2>&1 || true

  CURRENT_CHILD_PID=""
  CURRENT_CHILD_PGID=""
}

cleanup_on_exit() {
  # Defensive cleanup when exiting unexpectedly.
  if [[ -n "$CURRENT_CHILD_PID" ]]; then
    cleanup_step_process
  fi
}

safe_make_stop() {
  TEST_ENV_CONFIG="$CONFIG_FILE" make -C "$ROOT_DIR" stop >/dev/null 2>&1 || true
}

safe_make_clean() {
  TEST_ENV_CONFIG="$CONFIG_FILE" make -C "$ROOT_DIR" clean >/dev/null 2>&1 || true
}

pre_step_reset() {
  if bool_true "$HARD_RESET"; then
    safe_make_clean
  else
    safe_make_stop
  fi
}

post_step_reset() {
  safe_make_stop
}

finalize_partial_report() {
  if ! python3 "$ROOT_DIR/scripts/report/aggregate_max_coverage.py" \
    --steps "$STEPS_FILE" \
    --output-dir "$OUT_DIR"; then
    INFRA_ERR=1
  fi
}

handle_interrupt() {
  local signal="${1:-INT}"
  if [[ "$INTERRUPTED" -eq 1 ]]; then
    return
  fi
  INTERRUPTED=1
  echo ""
  echo "[coverage] ⚠️  received $signal, stopping now..."
  if [[ -n "$CURRENT_STEP" ]]; then
    echo "[coverage] current step: $CURRENT_STEP"
    [[ -n "$CURRENT_STEP_LOG" ]] && echo "[coverage] current step log: $CURRENT_STEP_LOG"
  fi

  cleanup_step_process
  post_step_reset
  finalize_partial_report

  echo "[coverage] interrupted. partial report: $OUT_DIR/index.md"
  exit 130
}

trap 'handle_interrupt INT' INT
trap 'handle_interrupt TERM' TERM
trap 'cleanup_on_exit' EXIT

command_display() {
  local out=""
  local item
  for item in "$@"; do
    out+=$(printf '%q ' "$item")
  done
  echo "${out%% }"
}

run_with_optional_timeout() {
  local timeout_value="$1"
  shift
  if command -v timeout >/dev/null 2>&1; then
    timeout --signal=TERM --kill-after=30s "$timeout_value" "$@"
  else
    "$@"
  fi
}

append_step_record() {
  local step="$1"
  local category="$2"
  local status="$3"
  local rc="$4"
  local started="$5"
  local finished="$6"
  local duration="$7"
  local log_file="$8"
  local report_dir="$9"
  shift 9
  local cmd="$*"

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$step" "$category" "$status" "$rc" "$started" "$finished" "$duration" "$log_file" "$report_dir" "$cmd" >> "$STEPS_FILE"
}

run_step() {
  local step="$1"
  local category="$2"
  local report_dir="$3"
  shift 3
  local -a cmd=("$@")

  TOTAL=$((TOTAL + 1))

  local step_log="$OUT_DIR/logs/${step}.log"
  mkdir -p "$report_dir"

  local cmd_text
  cmd_text="$(command_display "${cmd[@]}")"

  pre_step_reset

  local started_at started_epoch finished_at finished_epoch duration rc status
  started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  started_epoch="$(date +%s)"

  echo "==> [$step] $cmd_text"
  echo "    log: $step_log"
  # Force stdin to /dev/null to avoid child processes being SIGTTIN-stopped
  # when they try to read from terminal in non-foreground process groups.
  CURRENT_STEP="$step"
  CURRENT_STEP_LOG="$step_log"
  CURRENT_CHILD_PID=""
  CURRENT_CHILD_PGID=""

  (
    run_with_optional_timeout "$STEP_TIMEOUT" "${cmd[@]}" < /dev/null >"$step_log" 2>&1
  ) &
  CURRENT_CHILD_PID="$!"
  CURRENT_CHILD_PGID="$(ps -o pgid= "$CURRENT_CHILD_PID" 2>/dev/null | tr -d '[:space:]' || true)"
  wait "$CURRENT_CHILD_PID"
  rc=$?
  CURRENT_CHILD_PID=""
  CURRENT_CHILD_PGID=""
  CURRENT_STEP=""
  CURRENT_STEP_LOG=""

  finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  finished_epoch="$(date +%s)"
  duration=$((finished_epoch - started_epoch))

  status="PASS"
  if [[ "$rc" -eq 0 ]]; then
    PASS=$((PASS + 1))
  elif [[ "$rc" -eq 124 || "$rc" -eq 137 ]]; then
    status="TIMEOUT"
    TIMEOUT=$((TIMEOUT + 1))
    FAIL=$((FAIL + 1))
  else
    status="FAIL"
    FAIL=$((FAIL + 1))
  fi

  echo "<== [$step] $(status_display "$status") (${duration}s) log=$step_log"
  append_step_record "$step" "$category" "$status" "$rc" "$started_at" "$finished_at" "$duration" "$step_log" "$report_dir" "$cmd_text"

  post_step_reset
}

echo "[coverage] run id: $RUN_ID"
echo "[coverage] config: $CONFIG_FILE"
echo "[coverage] out dir: $OUT_DIR"
echo "[coverage] step timeout: $STEP_TIMEOUT"
echo "[coverage] hard reset: $HARD_RESET"
echo "[coverage] include perf tiers: $INCLUDE_PERF_TIERS"
echo "[coverage] exit policy: $EXIT_POLICY"

# Smoke matrix (single + multi)
run_step "smoke_single_matrix" "smoke" "$OUT_DIR/smoke/single" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" SMOKE_REPORT_DIR="$OUT_DIR/smoke/single" \
  make -C "$ROOT_DIR" test-smoke TOPOLOGY=single MATRIX=1

run_step "smoke_multi_matrix" "smoke" "$OUT_DIR/smoke/multi" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" SMOKE_REPORT_DIR="$OUT_DIR/smoke/multi" \
  make -C "$ROOT_DIR" test-smoke TOPOLOGY=multi MATRIX=1

# Business groups (run independently; never break global flow)
for group in config governance staking delegation punish rewards epoch; do
  run_step "group_${group}" "group" "$OUT_DIR/groups/$group" \
    env TEST_ENV_CONFIG="$CONFIG_FILE" REPORT_DIR="$OUT_DIR/groups/$group" \
    make -C "$ROOT_DIR" test-group GROUP="$group"
done

# Fork matrix
run_step "fork_single" "fork" "$OUT_DIR/fork/single" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" FORK_REPORT_DIR="$OUT_DIR/fork/single" \
  make -C "$ROOT_DIR" test-fork TOPOLOGY=single

run_step "fork_multi" "fork" "$OUT_DIR/fork/multi" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" FORK_REPORT_DIR="$OUT_DIR/fork/multi" \
  make -C "$ROOT_DIR" test-fork TOPOLOGY=multi

# Scenarios
run_step "scenario_posa" "scenario" "$OUT_DIR/scenarios/posa" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" REPORT_DIR="$OUT_DIR/scenarios/posa" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=posa

run_step "scenario_interop_sync" "scenario" "$OUT_DIR/scenarios/interop_sync" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" REPORT_DIR="$OUT_DIR/scenarios/interop_sync" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=interop CHECK=sync

run_step "scenario_interop_state_root" "scenario" "$OUT_DIR/scenarios/interop_state_root" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" REPORT_DIR="$OUT_DIR/scenarios/interop_state_root" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=interop CHECK=state-root

run_step "scenario_bootstrap" "scenario" "$OUT_DIR/scenarios/bootstrap" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=bootstrap

run_step "scenario_upgrade" "scenario" "$OUT_DIR/scenarios/upgrade" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=upgrade

run_step "scenario_checkpoint" "scenario" "$OUT_DIR/scenarios/checkpoint" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=checkpoint

run_step "scenario_negative" "scenario" "$OUT_DIR/scenarios/negative" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=negative

run_step "scenario_rotation_punish" "scenario" "$OUT_DIR/scenarios/rotation_punish" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=rotation-punish

run_step "scenario_rotation_live" "scenario" "$OUT_DIR/scenarios/rotation_live" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=rotation-live

run_step "scenario_add_validator_live" "scenario" "$OUT_DIR/scenarios/add_validator_live" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=add-validator-live

run_step "scenario_add_validator_punish" "scenario" "$OUT_DIR/scenarios/add_validator_punish" \
  env TEST_ENV_CONFIG="$CONFIG_FILE" \
  make -C "$ROOT_DIR" test-scenario SCENARIO=add-validator-punish

if bool_true "$INCLUDE_PERF_TIERS"; then
  run_step "perf_tiers" "perf" "$OUT_DIR/perf/tiers" \
    env TEST_ENV_CONFIG="$CONFIG_FILE" \
    make -C "$ROOT_DIR" test-perf MODE=tiers
fi

# Final defensive stop
post_step_reset

if ! python3 "$ROOT_DIR/scripts/report/aggregate_max_coverage.py" \
  --steps "$STEPS_FILE" \
  --output-dir "$OUT_DIR"; then
  echo "[coverage] ERROR: failed to aggregate coverage report" >&2
  INFRA_ERR=1
fi

echo "[coverage] summary: total=$TOTAL pass=$PASS fail=$FAIL timeout=$TIMEOUT infra_err=$INFRA_ERR"
echo "[coverage] report: $OUT_DIR/index.md"
echo "[coverage] raw steps: $STEPS_FILE"

case "$EXIT_POLICY" in
  strict)
    if [[ "$FAIL" -gt 0 || "$INFRA_ERR" -gt 0 ]]; then
      exit 1
    fi
    exit 0
    ;;
  infra_only)
    if [[ "$INFRA_ERR" -gt 0 ]]; then
      exit 1
    fi
    exit 0
    ;;
  always_zero|*)
    exit 0
    ;;
esac
