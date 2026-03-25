#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$PROJECT_ROOT/scripts/network/lib.sh"

TOPOLOGY="${1:-multi}"
case "$TOPOLOGY" in
  single|multi) ;;
  *)
    echo "usage: scripts/fork/run_matrix.sh <single|multi>" >&2
    exit 1
    ;;
esac

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
FORK_CASES="${FORK_CASES:-poa,upgrade:shanghaiTime,upgrade:cancunTime,upgrade:posaTime,upgrade:fixHeaderTime,upgrade:allStaggered,upgrade:allSame,posa}"
FORK_DELAY_SECONDS="${FORK_DELAY_SECONDS:-$(cfg_get "$CONFIG_FILE" "network.fork_delay_seconds" "120")}"
FORK_UPGRADE_STARTUP_BUFFER_SINGLE="${FORK_UPGRADE_STARTUP_BUFFER_SINGLE:-5}"
FORK_UPGRADE_STARTUP_BUFFER_MULTI="${FORK_UPGRADE_STARTUP_BUFFER_MULTI:-30}"
FORK_TEST_TIMEOUT="${FORK_TEST_TIMEOUT:-20m}"
FORK_REPORT_DIR="${FORK_REPORT_DIR:-$PROJECT_ROOT/reports/fork_$(date +%Y%m%d_%H%M%S)}"
TEST_CONFIG_FILE="$PROJECT_ROOT/data/test_config.yaml"
COLLECTOR_SCRIPT="$PROJECT_ROOT/scripts/fork/collect_matrix_report.sh"
HEALTH_SCRIPT="$PROJECT_ROOT/scripts/report/assert_chain_health.sh"

mkdir -p "$FORK_REPORT_DIR"
RESULTS_TSV="$FORK_REPORT_DIR/matrix_results.tsv"
printf 'topology\tcase\tmode\ttarget\tstatus\trc\tlog\trepro\n' > "$RESULTS_TSV"

sanitize_case() {
  echo "$1" | tr '[:space:]' '_' | tr -c 'a-zA-Z0-9._:-' '_' | tr ':' '_'
}

status_display() {
  case "${1:-}" in
    PASS) printf '🟢 PASS' ;;
    FAIL) printf '🔴 FAIL' ;;
    SKIP) printf '🟡 SKIP' ;;
    *) printf '%s' "${1:-}" ;;
  esac
}

run_case() {
  local mode="$1"
  local target="$2"
  local label="$3"
  local case_delay="$FORK_DELAY_SECONDS"
  local rc=0

  if [[ "$mode" == "upgrade" ]]; then
    local startup_buffer="$FORK_UPGRADE_STARTUP_BUFFER_MULTI"
    if [[ "$TOPOLOGY" == "single" ]]; then
      startup_buffer="$FORK_UPGRADE_STARTUP_BUFFER_SINGLE"
    fi
    case_delay=$((FORK_DELAY_SECONDS + startup_buffer))
  fi

  local init_env=(
    "TEST_ENV_CONFIG=$CONFIG_FILE"
    "GENESIS_MODE=$mode"
    "FORK_TARGET=$target"
    "FORK_DELAY_SECONDS=$case_delay"
  )

  if [[ "$TOPOLOGY" == "single" ]]; then
    init_env+=(
      "TEST_NETWORK_VALIDATOR_COUNT=1"
      "TEST_NETWORK_NODE_COUNT=1"
    )
  fi

  local case_slug
  case_slug="$(sanitize_case "$label")"
  local case_dir="$FORK_REPORT_DIR/${TOPOLOGY}_${case_slug}"
  local case_log="$case_dir/run.log"
  local repro="FORK_CASES=$label FORK_DELAY_SECONDS=$FORK_DELAY_SECONDS FORK_TEST_TIMEOUT=$FORK_TEST_TIMEOUT TOPOLOGY=$TOPOLOGY make test-fork"

  mkdir -p "$case_dir"

  set +e
  {
    echo "=== [fork/$TOPOLOGY] case=$label mode=$mode target=${target:-<none>} ==="
    echo "Config: $CONFIG_FILE"
    echo "ReportDir: $case_dir"

    env "${init_env[@]}" make -C "$PROJECT_ROOT" clean
    rc=$?
    if [[ $rc -eq 0 ]]; then
      env "${init_env[@]}" make -C "$PROJECT_ROOT" init
      rc=$?
    fi

    if [[ $rc -eq 0 ]]; then
      if [[ "$TOPOLOGY" == "single" ]]; then
        env "${init_env[@]}" "$PROJECT_ROOT/scripts/network/native_single.sh" up "$CONFIG_FILE"
        rc=$?
        if [[ $rc -eq 0 ]]; then
          env "${init_env[@]}" "$PROJECT_ROOT/scripts/network/native_single.sh" ready "$CONFIG_FILE"
          rc=$?
        fi
      else
        env "${init_env[@]}" make -C "$PROJECT_ROOT" run
        rc=$?
        if [[ $rc -eq 0 ]]; then
          env "${init_env[@]}" make -C "$PROJECT_ROOT" ready
          rc=$?
        fi
      fi
    fi

    if [[ $rc -eq 0 ]]; then
      (cd "$PROJECT_ROOT" && env "${init_env[@]}" go test ./tests/fork -v -run "^TestF_ForkLiveness$" -count=1 -parallel=1 -p 1 -timeout "$FORK_TEST_TIMEOUT" -config "$TEST_CONFIG_FILE")
      rc=$?
    fi

    if [[ $rc -eq 0 ]] && [[ -x "$HEALTH_SCRIPT" ]]; then
      env "${init_env[@]}" "$HEALTH_SCRIPT" || rc=$?
    fi

    if [[ "$TOPOLOGY" == "single" ]]; then
      env "${init_env[@]}" "$PROJECT_ROOT/scripts/network/native_single.sh" down "$CONFIG_FILE" || true
    else
      env "${init_env[@]}" make -C "$PROJECT_ROOT" stop || true
    fi
    env "${init_env[@]}" make -C "$PROJECT_ROOT" clean || true

    echo "Case result rc=$rc"
  } > "$case_log" 2>&1
  set -e

  cat "$case_log"

  local status="PASS"
  if [[ $rc -ne 0 ]]; then
    status="FAIL"
  fi
  echo "$(status_display "$status") [fork/$TOPOLOGY] case=$label mode=$mode target=${target:-<none>} rc=$rc"
  printf '%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n' "$TOPOLOGY" "$label" "$mode" "${target:-}" "$status" "$rc" "$case_log" "$repro" >> "$RESULTS_TSV"
  return "$rc"
}

overall_rc=0
IFS=',' read -r -a CASE_ARRAY <<< "$FORK_CASES"
for raw_case in "${CASE_ARRAY[@]}"; do
  case_item="$(echo "$raw_case" | tr -d '[:space:]')"
  [[ -n "$case_item" ]] || continue

  mode="$case_item"
  target=""
  if [[ "$case_item" == upgrade:* ]]; then
    mode="upgrade"
    target="${case_item#upgrade:}"
  fi
  if [[ "$mode" == "upgrade" && -z "$target" ]]; then
    echo "invalid fork case '$case_item': upgrade requires target" >&2
    overall_rc=1
    continue
  fi

  if ! run_case "$mode" "$target" "$case_item"; then
    overall_rc=1
  fi
done

if [[ -x "$COLLECTOR_SCRIPT" ]]; then
  "$COLLECTOR_SCRIPT" "$RESULTS_TSV" "$FORK_REPORT_DIR"
fi

echo "Fork matrix report: $FORK_REPORT_DIR"
exit "$overall_rc"
