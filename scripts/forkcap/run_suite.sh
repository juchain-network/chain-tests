#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_CONFIG="${TEST_ENV_CONFIG:-config/test_env.yaml}"
FORK="${FORK:-all}"
CASE_PATTERN="${CASE:-TestK_Forkcap.*}"
REPORT_ROOT="${REPORT_DIR:-reports/forkcap_$(date +%Y%m%d_%H%M%S)}"
CI_TOOL_CMD=(go run ./ci.go)
TEMP_CONFIGS=()

cleanup() {
  make --no-print-directory stop >/dev/null 2>&1 || true
  for path in "${TEMP_CONFIGS[@]:-}"; do
    rm -f "$path"
  done
}
trap cleanup EXIT

phase_config() {
  local fork="$1"
  local phase="$2"
  case "$fork:$phase" in
    shanghai:pre) echo "poa|" ;;
    shanghai:post) echo "smoke|poa_shanghai" ;;
    cancun:pre) echo "smoke|poa_shanghai" ;;
    cancun:post) echo "smoke|poa_shanghai_cancun" ;;
    fixheader:pre) echo "smoke|poa_shanghai_cancun" ;;
    fixheader:post) echo "smoke|poa_shanghai_cancun_fixheader" ;;
    posa:pre) echo "smoke|poa_shanghai_cancun_fixheader" ;;
    posa:post) echo "smoke|poa_shanghai_cancun_fixheader_posa" ;;
    prague:pre) echo "smoke|poa_shanghai_cancun_fixheader_posa" ;;
    prague:post) echo "smoke|poa_shanghai_cancun_fixheader_posa_prague" ;;
    osaka:pre) echo "smoke|poa_shanghai_cancun_fixheader_posa_prague" ;;
    osaka:post) echo "smoke|poa_shanghai_cancun_fixheader_posa_prague_osaka" ;;
    bpo1:pre) echo "smoke|poa_shanghai_cancun_fixheader_posa_prague_osaka" ;;
    bpo1:post) echo "smoke|poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1" ;;
    bpo2:pre) echo "smoke|poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1" ;;
    bpo2:post) echo "smoke|poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2" ;;
    *) return 1 ;;
  esac
}

make_geth_config() {
  local src="$1"
  local tmp_cfg
  tmp_cfg="$(mktemp "${TMPDIR:-/tmp}/chain-tests-forkcap-geth.XXXXXX")"
  awk '
    /^runtime:/ { in_runtime=1; print; next }
    /^[^[:space:]]/ && $0 !~ /^runtime:/ { in_runtime=0 }
    in_runtime && /^  backend:/ { print "  backend: native"; next }
    in_runtime && /^  impl_mode:/ { print "  impl_mode: single"; next }
    in_runtime && /^  impl:/ { print "  impl: geth"; next }
    { print }
  ' "$src" > "$tmp_cfg"
  TEMP_CONFIGS+=("$tmp_cfg")
  echo "$tmp_cfg"
}

run_phase() {
  local fork="$1"
  local phase="$2"
  local cfg
  cfg="$(phase_config "$fork" "$phase")"
  local init_mode="${cfg%%|*}"
  local init_target="${cfg#*|}"
  local phase_report_root="$REPORT_ROOT/$fork/$phase"
  local phase_config_file
  phase_config_file="$(make_geth_config "$BASE_CONFIG")"

  echo "⚙️  forkcap setup fork=$fork phase=$phase mode=$init_mode target=${init_target:-<none>} impl=geth"
  TEST_ENV_CONFIG="$phase_config_file" make --no-print-directory clean
  if [[ -n "$init_target" ]]; then
    TEST_ENV_CONFIG="$phase_config_file" make --no-print-directory init TOPOLOGY=single INIT_MODE="$init_mode" INIT_TARGET="$init_target"
  else
    TEST_ENV_CONFIG="$phase_config_file" make --no-print-directory init TOPOLOGY=single INIT_MODE="$init_mode"
  fi
  TEST_ENV_CONFIG="$phase_config_file" make --no-print-directory run

  echo "🧪 forkcap tests fork=$fork phase=$phase pattern=$CASE_PATTERN"
  mkdir -p "$phase_report_root"
  FORKCAP_FORK="$fork" \
  FORKCAP_PHASE="$phase" \
  REPORT_DIR="$phase_report_root" \
  TEST_ENV_CONFIG="$phase_config_file" \
  "${CI_TOOL_CMD[@]}" -mode tests -pkgs ./tests/forkcaps -run "$CASE_PATTERN" -skip-setup -report-dir "$phase_report_root"

  TEST_ENV_CONFIG="$phase_config_file" make --no-print-directory stop
}

run_fork() {
  local fork="$1"
  run_phase "$fork" pre
  run_phase "$fork" post
}

case "$FORK" in
  shanghai|cancun|fixheader|posa|prague|osaka|bpo1|bpo2)
    run_fork "$FORK"
    ;;
  all)
    for fork in shanghai cancun fixheader posa prague osaka bpo1 bpo2; do
      run_fork "$fork"
    done
    ;;
  *)
    echo "FORK must be shanghai|cancun|fixheader|posa|prague|osaka|bpo1|bpo2|all" >&2
    exit 1
    ;;
esac

echo "📦 forkcap reports under $REPORT_ROOT"
