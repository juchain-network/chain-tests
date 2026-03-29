#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/coverage/lib.sh
source "$SCRIPT_DIR/lib.sh"

if [[ "${1:-}" == "--" ]]; then
  shift
fi
[[ $# -gt 0 ]] || die "missing wrapped command"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
coverage_apply_session_defaults
export CHAIN_COVERAGE=1
export CHAIN_COVERAGE_SCOPE="${CHAIN_COVERAGE_SCOPE:-congress}"
export CHAIN_COVERAGE_ACTIVE=1
export CHAIN_COVERAGE_SESSION="${CHAIN_COVERAGE_SESSION:-0}"

if [[ -z "${CHAIN_COVERAGE_OUT_DIR:-}" ]]; then
  export CHAIN_COVERAGE_OUT_DIR="$ROOT_DIR/reports/coverage_$(date +%Y%m%d_%H%M%S)"
fi

OUT_DIR="$(CHAIN_COVERAGE_OUT_DIR="$CHAIN_COVERAGE_OUT_DIR" coverage_out_dir)"
export CHAIN_COVERAGE_OUT_DIR="$OUT_DIR"

coverage_prepare_dirs
if ! coverage_bool_true "$CHAIN_COVERAGE_SESSION"; then
  rm -rf "$(coverage_raw_root)"
fi
mkdir -p "$(coverage_raw_root)"

GETH_BINARY="$("$SCRIPT_DIR/prepare_chain_coverage.sh" --config "$CONFIG_FILE" --print-binary)"
export CHAIN_COVERAGE_GETH_BINARY="$GETH_BINARY"

CMD_TEXT="$(printf '%q ' "$@")"
CMD_TEXT="${CMD_TEXT%% }"
export CHAIN_COVERAGE_COMMAND="$CMD_TEXT"

echo "[coverage] scope: $(coverage_scope)"
echo "[coverage] out dir: $OUT_DIR"
echo "[coverage] geth binary: $GETH_BINARY"
echo "[coverage] command: $CMD_TEXT"

set +e
"$@"
cmd_rc=$?
set -e

TEST_ENV_CONFIG="${TEST_ENV_CONFIG:-$CONFIG_FILE}" make -C "$ROOT_DIR" --no-print-directory stop >/dev/null 2>&1 || true

if coverage_bool_true "$CHAIN_COVERAGE_SESSION"; then
  raw_files="$(find "$(coverage_raw_root)" -type f 2>/dev/null | wc -l | tr -d '[:space:]')"
  if [[ "$raw_files" =~ ^[0-9]+$ ]] && (( raw_files > 0 )); then
    echo "[coverage] session mode active; raw data retained under $(coverage_raw_root) (files=$raw_files)"
  else
    echo "[coverage] WARN: session mode active but no raw coverage files were produced under $(coverage_raw_root)" >&2
  fi
  exit "$cmd_rc"
fi

set +e
"$SCRIPT_DIR/aggregate_chain_coverage.sh"
agg_rc=$?
set -e

if [[ "$cmd_rc" -ne 0 ]]; then
  if [[ "$agg_rc" -ne 0 ]]; then
    echo "[coverage] WARN: aggregation also failed with rc=$agg_rc" >&2
  fi
  exit "$cmd_rc"
fi

exit "$agg_rc"
