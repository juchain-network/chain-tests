#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RUN_ID="$(date +%Y%m%d_%H%M%S)"
LOG_DIR="${LOG_DIR:-$ROOT_DIR/reports/manual_all_non_perf_$RUN_ID}"
mkdir -p "$LOG_DIR"

FAILURES=()

run_step() {
  local name="$1"
  shift

  local log_file="$LOG_DIR/${name}.log"
  echo
  echo "==> $name"
  echo "cmd: $*"
  echo "log: $log_file"

  if "$@" 2>&1 | tee "$log_file"; then
    echo "<== $name: PASS"
  else
    local status=$?
    echo "<== $name: FAIL (exit=$status)"
    FAILURES+=("$name:$status:$log_file")
  fi
}

# `make test` expects a ready network, so bootstrap it explicitly first.
run_step reset make reset
run_step test make test

run_step test_group_all make test-group GROUP=all
run_step test_smoke_all make test-smoke TOPOLOGY=all MATRIX=1
run_step test_fork_all make test-fork TOPOLOGY=all
run_step test_scenario_all make test-scenario SCENARIO=all
run_step test_regression_core make test-regression SCOPE=core
run_step test_regression_full make test-regression SCOPE=full
run_step test_coverage_max make test-coverage-max

echo
echo "logs: $LOG_DIR"

if ((${#FAILURES[@]} > 0)); then
  echo "failed steps:"
  for item in "${FAILURES[@]}"; do
    IFS=":" read -r name status log_file <<<"$item"
    echo "  - $name (exit=$status) log=$log_file"
  done
  exit 1
fi

echo "all non-perf test commands passed"
