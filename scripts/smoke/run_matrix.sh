#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$PROJECT_ROOT/scripts/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
TOPOLOGY="${1:-${SMOKE_TOPOLOGY:-}}"
if [[ -z "$TOPOLOGY" ]]; then
  cfg_nodes="$(cfg_get "$CONFIG_FILE" "network.node_count" "4")"
  cfg_validators="$(cfg_get "$CONFIG_FILE" "network.validator_count" "3")"
  if [[ "$cfg_nodes" == "1" && "$cfg_validators" == "1" ]]; then
    TOPOLOGY="single"
  else
    TOPOLOGY="multi"
  fi
fi
case "$TOPOLOGY" in
  single|multi) ;;
  *)
    echo "usage: scripts/smoke/run_matrix.sh <single|multi>" >&2
    exit 1
    ;;
esac

SMOKE_CASES="${SMOKE_CASES:-poa,poa_shanghai,poa_shanghai_cancun,poa_shanghai_cancun_fixheader,poa_shanghai_cancun_fixheader_posa,poa_shanghai_cancun_fixheader_posa_prague,poa_shanghai_cancun_fixheader_posa_prague_osaka}"
SMOKE_SINGLE_OBSERVE_SECONDS="${SMOKE_SINGLE_OBSERVE_SECONDS:-$(cfg_get "$CONFIG_FILE" "tests.smoke.observe_seconds" "300")}"
SMOKE_REPORT_DIR="${SMOKE_REPORT_DIR:-$PROJECT_ROOT/reports/smoke_matrix_${TOPOLOGY}_$(date +%Y%m%d_%H%M%S)}"
COLLECTOR_SCRIPT="$PROJECT_ROOT/scripts/smoke/collect_matrix_report.sh"

mkdir -p "$SMOKE_REPORT_DIR"
RESULTS_TSV="$SMOKE_REPORT_DIR/matrix_results.tsv"
printf 'topology\tcase\tmode\ttarget\tstatus\trc\tlog\treport\tsummary\tmanifest\trepro\n' > "$RESULTS_TSV"

sanitize_case() {
  printf '%s' "$1" | tr '[:space:]' '_' | tr -c 'a-zA-Z0-9._:-' '_' | tr ':' '_'
}

status_display() {
  case "${1:-}" in
    PASS) printf '🟢 PASS' ;;
    FAIL) printf '🔴 FAIL' ;;
    SKIP) printf '🟡 SKIP' ;;
    *) printf '%s' "${1:-}" ;;
  esac
}

case_to_mode_target() {
  local label="$1"
  case "$label" in
    poa)
      printf 'poa\t\n'
      ;;
    poa_shanghai|poa_shanghai_cancun|poa_shanghai_cancun_fixheader|poa_shanghai_cancun_fixheader_posa|poa_shanghai_cancun_fixheader_posa_prague|poa_shanghai_cancun_fixheader_posa_prague_osaka)
      printf 'smoke\t%s\n' "$label"
      ;;
    *)
      return 1
      ;;
  esac
}

find_ci_artifact() {
  local base_dir="$1"
  local file_name="$2"
  local latest
  latest="$(ls -1dt "$base_dir"/ci_* 2>/dev/null | head -n 1 || true)"
  if [[ -z "$latest" ]]; then
    echo ""
    return 0
  fi
  if [[ -f "$latest/$file_name" ]]; then
    echo "$latest/$file_name"
    return 0
  fi
  echo ""
}

emit_case_artifacts() {
  local case_dir="$1"
  local topology="$2"
  local label="$3"
  local mode="$4"
  local target="$5"
  local status="$6"
  local rc="$7"
  local repro="$8"
  local case_log="$9"
  local report_path="$case_dir/report.md"
  local summary_path="$case_dir/summary.json"
  local manifest_path="$case_dir/manifest.json"
  local generated_at
  generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  local status_display_text
  status_display_text="$(status_display "$status")"

  cat > "$report_path" <<REPORT
# Smoke Matrix Case Report

- Generated: $generated_at
- Topology: $topology
- Case: $label
- Genesis Mode: $mode
- Target: ${target:-<none>}
- Status: $status_display_text
- RC: $rc
- Log: $case_log
- Repro: \`$repro\`
REPORT

  cat > "$summary_path" <<SUMMARY
{
  "generated_at": "$generated_at",
  "mode": "smoke-case",
  "status": "$status",
  "total_step_count": 1,
  "failed_step_count": $([[ "$status" == "FAIL" ]] && echo 1 || echo 0),
  "total_pass_tests": $([[ "$status" == "PASS" ]] && echo 1 || echo 0),
  "total_fail_tests": $([[ "$status" == "FAIL" ]] && echo 1 || echo 0),
  "total_skip_tests": $([[ "$status" == "SKIP" ]] && echo 1 || echo 0),
  "report_path": "$report_path",
  "steps": [
    {
      "name": "smoke_single_$label",
      "status": "$status",
      "duration": 0,
      "pass_count": $([[ "$status" == "PASS" ]] && echo 1 || echo 0),
      "fail_count": $([[ "$status" == "FAIL" ]] && echo 1 || echo 0),
      "skip_count": $([[ "$status" == "SKIP" ]] && echo 1 || echo 0),
      "log_path": "$case_log"
    }
  ]
}
SUMMARY

  cat > "$manifest_path" <<MANIFEST
{
  "generated_at": "$generated_at",
  "mode": "smoke-case",
  "runtime_backend": "native",
  "runtime_impl": "",
  "runtime_impl_mode": "single",
  "git_commit": "",
  "geth_version": "",
  "reth_version": "",
  "genesis_hash": "",
  "fork_schedule": {
    "mode": "$mode",
    "target": "$target"
  },
  "case_list": ["$label"],
  "repro_commands": ["$repro"],
  "report_path": "$report_path",
  "summary_path": "$summary_path",
  "step_status": {
    "smoke_single_$label": "$status"
  },
  "step_logs": {
    "smoke_single_$label": "$case_log"
  },
  "step_durations": {
    "smoke_single_$label": 0
  }
}
MANIFEST

  printf '%s\t%s\t%s\n' "$report_path" "$summary_path" "$manifest_path"
}

resolve_artifact_paths() {
  local artifact_triplet="$1"
  report_path="$(printf '%s\n' "$artifact_triplet" | awk -F '\t' 'NR==1 {print $1}')"
  summary_path="$(printf '%s\n' "$artifact_triplet" | awk -F '\t' 'NR==1 {print $2}')"
  manifest_path="$(printf '%s\n' "$artifact_triplet" | awk -F '\t' 'NR==1 {print $3}')"
}

run_case() {
  local mode="$1"
  local target="$2"
  local label="$3"
  local case_slug
  local case_dir
  local case_log
  local status="PASS"
  local rc=0
  local report_path=""
  local summary_path=""
  local manifest_path=""

  case_slug="$(sanitize_case "$label")"
  case_dir="$SMOKE_REPORT_DIR/${TOPOLOGY}_${case_slug}"
  case_log="$case_dir/run.log"
  mkdir -p "$case_dir"

  local repro="SMOKE_CASES=$label TOPOLOGY=$TOPOLOGY MATRIX=1 make test-smoke"
  local support_report
  support_report="$(runtime_case_support_report "$CONFIG_FILE" "$TOPOLOGY" "$mode" "$target")"

  if [[ "$(printf '%s' "$support_report" | python3 -c 'import json,sys; print("1" if json.load(sys.stdin)["supported"] else "0")')" != "1" ]]; then
    local support_summary
    support_summary="$(printf '%s' "$support_report" | python3 -c 'import json,sys; d=json.load(sys.stdin); nodes=", ".join("{}:{}@{}->{}".format(n["node"], n["impl"], n["version"], n["max_fork"]) for n in d["nodes"]); print("required={} network_max={} nodes=[{}]".format(d["required_fork"], d["network_max_fork"], nodes))')"
    printf '=== [smoke/%s] case=%s mode=%s target=%s ===\n' "$TOPOLOGY" "$label" "$mode" "${target:-<none>}" > "$case_log"
    printf 'Config: %s\n' "$CONFIG_FILE" >> "$case_log"
    printf 'ReportDir: %s\n' "$case_dir" >> "$case_log"
    printf 'Case skipped: runtime capability mismatch (%s)\n' "$support_summary" >> "$case_log"
    status="SKIP"
    resolve_artifact_paths "$(emit_case_artifacts "$case_dir" "$TOPOLOGY" "$label" "$mode" "$target" "$status" "$rc" "$repro" "$case_log")"
    cat "$case_log"
    echo "$(status_display "$status") [smoke/$TOPOLOGY] case=$label mode=$mode target=${target:-<none>} rc=$rc"
    printf '%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n' \
      "$TOPOLOGY" "$label" "$mode" "$target" "$status" "$rc" "$case_log" "$report_path" "$summary_path" "$manifest_path" "$repro" >> "$RESULTS_TSV"
    return 0
  fi

  set +e
  {
    echo "=== [smoke/$TOPOLOGY] case=$label mode=$mode target=${target:-<none>} ==="
    echo "Config: $CONFIG_FILE"
    echo "ReportDir: $case_dir"

    if [[ "$TOPOLOGY" == "single" ]]; then
      env \
        TEST_ENV_CONFIG="$CONFIG_FILE" \
        SMOKE_SINGLE_GENESIS_MODE="$mode" \
        SMOKE_SINGLE_FORK_TARGET="$target" \
        SMOKE_SINGLE_OBSERVE_SECONDS="$SMOKE_SINGLE_OBSERVE_SECONDS" \
        make -C "$PROJECT_ROOT" test-smoke TOPOLOGY=single MATRIX=0
      rc=$?
    else
      env \
        TEST_ENV_CONFIG="$CONFIG_FILE" \
        GENESIS_MODE="$mode" \
        FORK_TARGET="$target" \
        REPORT_DIR="$case_dir/reports" \
        make -C "$PROJECT_ROOT" test-smoke TOPOLOGY=multi MATRIX=0
      rc=$?
    fi

    echo "Case result rc=$rc"
  } > "$case_log" 2>&1
  set -e

  cat "$case_log"

  if [[ $rc -ne 0 ]]; then
    status="FAIL"
  fi

  echo "$(status_display "$status") [smoke/$TOPOLOGY] case=$label mode=$mode target=${target:-<none>} rc=$rc"

  if [[ "$TOPOLOGY" == "multi" ]]; then
    report_path="$(find_ci_artifact "$case_dir/reports" report.md)"
    summary_path="$(find_ci_artifact "$case_dir/reports" summary.json)"
    manifest_path="$(find_ci_artifact "$case_dir/reports" manifest.json)"
  else
    resolve_artifact_paths "$(emit_case_artifacts "$case_dir" "$TOPOLOGY" "$label" "$mode" "$target" "$status" "$rc" "$repro" "$case_log")"
  fi

  if [[ -z "$report_path" || -z "$summary_path" || -z "$manifest_path" ]]; then
    resolve_artifact_paths "$(emit_case_artifacts "$case_dir" "$TOPOLOGY" "$label" "$mode" "$target" "$status" "$rc" "$repro" "$case_log")"
  fi

  printf '%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n' \
    "$TOPOLOGY" "$label" "$mode" "$target" "$status" "$rc" "$case_log" "$report_path" "$summary_path" "$manifest_path" "$repro" >> "$RESULTS_TSV"

  return "$rc"
}

overall_rc=0
OLD_IFS="$IFS"
IFS=','
for raw_case in $SMOKE_CASES; do
  case_item="$(echo "$raw_case" | tr -d '[:space:]')"
  [[ -n "$case_item" ]] || continue

  mode_target="$(case_to_mode_target "$case_item" || true)"
  if [[ -z "$mode_target" ]]; then
    echo "invalid smoke case '$case_item'" >&2
    overall_rc=1
    continue
  fi

  mode="${mode_target%%$'\t'*}"
  target="${mode_target#*$'\t'}"

  if ! run_case "$mode" "$target" "$case_item"; then
    overall_rc=1
  fi
done
IFS="$OLD_IFS"

if [[ -x "$COLLECTOR_SCRIPT" ]]; then
  "$COLLECTOR_SCRIPT" "$RESULTS_TSV" "$SMOKE_REPORT_DIR"
fi

echo "Smoke matrix report: $SMOKE_REPORT_DIR"
exit "$overall_rc"
