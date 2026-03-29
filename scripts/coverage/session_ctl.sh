#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/coverage/lib.sh
source "$SCRIPT_DIR/lib.sh"

ACTION="${1:-}"
CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"

usage() {
  cat <<'EOF'
Usage:
  scripts/coverage/session_ctl.sh <start|merge|stop|status>
EOF
}

start_session() {
  export CHAIN_COVERAGE=1
  export CHAIN_COVERAGE_SESSION=1
  export CHAIN_COVERAGE_SCOPE="${CHAIN_COVERAGE_SCOPE:-congress}"
  export CHAIN_COVERAGE_KEEP_RAW="${CHAIN_COVERAGE_KEEP_RAW:-1}"

  if [[ -z "${CHAIN_COVERAGE_OUT_DIR:-}" ]]; then
    export CHAIN_COVERAGE_OUT_DIR="$ROOT_DIR/reports/coverage_session_$(date +%Y%m%d_%H%M%S)"
  else
    export CHAIN_COVERAGE_OUT_DIR="$(to_abs_path "$CHAIN_COVERAGE_OUT_DIR")"
  fi

  coverage_scope >/dev/null
  coverage_assert_supported_runtime "$CONFIG_FILE"
  coverage_prepare_dirs

  local out_dir raw_root session_file coverage_bin
  out_dir="$(coverage_out_dir)"
  raw_root="$(coverage_raw_root)"
  session_file="$(coverage_session_file)"

  rm -rf "$raw_root" "$out_dir/merged" "$out_dir/coverage.out" "$out_dir/func.txt" \
    "$out_dir/package_percent.txt" "$out_dir/summary.txt" "$out_dir/meta.json"
  mkdir -p "$raw_root" "$(dirname "$session_file")"

  coverage_bin="$("$SCRIPT_DIR/prepare_chain_coverage.sh" --config "$CONFIG_FILE" --print-binary)"

  cat >"$session_file" <<EOF_SESSION
SESSION_CHAIN_COVERAGE=1
SESSION_CHAIN_COVERAGE_SESSION=1
SESSION_CHAIN_COVERAGE_SCOPE=$CHAIN_COVERAGE_SCOPE
SESSION_CHAIN_COVERAGE_OUT_DIR=$out_dir
SESSION_CHAIN_COVERAGE_KEEP_RAW=$CHAIN_COVERAGE_KEEP_RAW
EOF_SESSION

  echo "[coverage] session started"
  echo "[coverage] out dir: $out_dir"
  echo "[coverage] geth binary: $coverage_bin"
  echo "[coverage] session file: $session_file"
}

merge_session() {
  coverage_apply_session_defaults
  coverage_session_active || die "coverage session not active"

  export CHAIN_COVERAGE=1
  export CHAIN_COVERAGE_SESSION=1

  "$SCRIPT_DIR/aggregate_chain_coverage.sh"
  rm -f "$(coverage_session_file)"
  echo "[coverage] session closed"
}

stop_session() {
  if coverage_session_active; then
    rm -f "$(coverage_session_file)"
    echo "[coverage] session stopped"
  else
    echo "[coverage] no active session"
  fi
}

status_session() {
  if ! coverage_session_active; then
    echo "[coverage] no active session"
    exit 0
  fi

  coverage_apply_session_defaults
  echo "[coverage] session: active"
  echo "[coverage] scope: ${CHAIN_COVERAGE_SCOPE:-congress}"
  echo "[coverage] out dir: $(coverage_out_dir)"
  echo "[coverage] raw root: $(coverage_raw_root)"
  echo "[coverage] session file: $(coverage_session_file)"
}

case "$ACTION" in
  start)
    start_session
    ;;
  merge)
    merge_session
    ;;
  stop)
    stop_session
    ;;
  status)
    status_session
    ;;
  ""|-h|--help)
    usage
    ;;
  *)
    die "unsupported coverage session action: $ACTION"
    ;;
esac
