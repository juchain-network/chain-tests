#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$ROOT_DIR/scripts/network/lib.sh"

normalize_duration_like() {
  local raw="$1"
  if [[ "$raw" =~ ^[0-9]+$ ]]; then
    printf '%ss' "$raw"
  else
    printf '%s' "$raw"
  fi
}

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
SOAK_DURATION="${PERF_SOAK_DURATION:-$(cfg_get "$CONFIG_FILE" "perf.soak.duration" "24h")}"
SOAK_TPS="${PERF_SOAK_TPS:-$(cfg_get "$CONFIG_FILE" "perf.soak.tps" "10")}"
SAMPLE_INTERVAL_RAW="${PERF_SAMPLE_INTERVAL:-$(cfg_get "$CONFIG_FILE" "perf.sample_interval_seconds" "2")}" 
SAMPLE_INTERVAL="$(normalize_duration_like "$SAMPLE_INTERVAL_RAW")"
RESTART_INTERVAL_RAW="${PERF_SOAK_RESTART_INTERVAL:-$(cfg_get "$CONFIG_FILE" "perf.soak.restart_interval" "1h")}"
DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
TEST_CONFIG="${PERF_TEST_CONFIG:-$DATA_DIR/test_config.yaml}"
OUT_DIR="${PERF_REPORT_DIR:-$ROOT_DIR/reports/perf_$(date +%Y%m%d_%H%M%S)_soak}"
PERF_SCOPE="${PERF_SCOPE:-single}"
PERF_TOPOLOGY="${PERF_TOPOLOGY:-single}"
PERF_INIT_MODE="${PERF_INIT_MODE:-posa}"
PERF_AUTO_STOP="${PERF_AUTO_STOP:-1}"
PERF_SENDER_ACCOUNTS="${PERF_SENDER_ACCOUNTS:-0}"

MIN_SUCCESS_RATE="$(cfg_get "$CONFIG_FILE" "perf.thresholds.success_rate_min" "0.99")"
MAX_STALL="$(cfg_get "$CONFIG_FILE" "perf.thresholds.stall_window_seconds_max" "15")"
MAX_LAG="$(cfg_get "$CONFIG_FILE" "perf.thresholds.max_height_lag" "8")"
MAX_P95="$(cfg_get "$CONFIG_FILE" "perf.thresholds.rpc_latency_p95_ms_max" "500")"
MULTI_WARMUP_TIMEOUT="${PERF_MULTI_WARMUP_TIMEOUT:-60s}"
MULTI_WARMUP_STABLE_SAMPLES="${PERF_MULTI_WARMUP_STABLE_SAMPLES:-3}"

parse_to_seconds() {
  python3 - "$1" <<'PY'
import re
import sys
raw = sys.argv[1].strip().lower()
m = re.fullmatch(r"(\d+)([smhd])", raw)
if not m:
    print(0)
    raise SystemExit(0)
val = int(m.group(1))
unit = m.group(2)
mult = {'s':1,'m':60,'h':3600,'d':86400}[unit]
print(val * mult)
PY
}

RESTART_SECONDS="$(parse_to_seconds "$RESTART_INTERVAL_RAW")"
mkdir -p "$OUT_DIR"

cd "$ROOT_DIR"
echo "Running soak: duration=$SOAK_DURATION tps=$SOAK_TPS sample_interval=$SAMPLE_INTERVAL restart_interval=$RESTART_INTERVAL_RAW topology=$PERF_TOPOLOGY scope=$PERF_SCOPE"
echo "Output dir: $OUT_DIR"

cleanup_perf_runtime() {
  local rc=$?
  if [[ -n "${RESTART_PID:-}" ]]; then
    kill "$RESTART_PID" >/dev/null 2>&1 || true
  fi
  if [[ "$PERF_AUTO_STOP" == "1" || "$PERF_AUTO_STOP" == "true" || "$PERF_AUTO_STOP" == "yes" ]]; then
    if runtime_session_exists "${RUNTIME_SESSION_FILE:-}"; then
      echo "Perf cleanup: stopping runtime..."
      TEST_ENV_CONFIG="$CONFIG_FILE" "$ROOT_DIR/scripts/network/dispatch.sh" down "$CONFIG_FILE" >/dev/null 2>&1 || true
    fi
  fi
  exit "$rc"
}

trap cleanup_perf_runtime EXIT INT TERM

rpc_ready() {
  local rpc_url="$1"
  local resp
  resp="$(curl -s --max-time 2 \
    -H 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
    "$rpc_url" || true)"
  [[ "$resp" == *'"result":"0x'* ]]
}

prepare_perf_runtime() {
  local rpc_url
  rpc_url="$(cfg_get "$CONFIG_FILE" "network.external_rpc" "http://localhost:18545")"

  echo "Perf setup: rebuilding runtime topology=$PERF_TOPOLOGY init_mode=$PERF_INIT_MODE"
  TEST_ENV_CONFIG="$CONFIG_FILE" make --no-print-directory clean >/dev/null
  TEST_ENV_CONFIG="$CONFIG_FILE" TOPOLOGY="$PERF_TOPOLOGY" INIT_MODE="$PERF_INIT_MODE" EPOCH="${EPOCH:-}" make --no-print-directory init >/dev/null
  TEST_ENV_CONFIG="$CONFIG_FILE" make --no-print-directory run >/dev/null
  if ! rpc_ready "$rpc_url"; then
    echo "Perf setup: RPC not ready after run at $rpc_url" >&2
    return 1
  fi
  TEST_ENV_CONFIG="$CONFIG_FILE" make --no-print-directory ready >/dev/null
}

prepare_perf_runtime

if [[ "$RESTART_SECONDS" -gt 0 ]]; then
  (
    while true; do
      sleep "$RESTART_SECONDS"
      if ! "$ROOT_DIR/scripts/perf/restart_node.sh" >> "$OUT_DIR/restarts.log" 2>&1; then
        echo "restart failed at $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$OUT_DIR/restarts.log"
      else
        echo "restart ok at $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$OUT_DIR/restarts.log"
      fi
    done
  ) &
  RESTART_PID=$!
fi

go run ./scripts/perf \
  -mode soak \
  -scope "$PERF_SCOPE" \
  -topology "$PERF_TOPOLOGY" \
  -config "$TEST_CONFIG" \
  -tiers "$SOAK_TPS" \
  -duration "$SOAK_DURATION" \
  -sample-interval "$SAMPLE_INTERVAL" \
  -data-dir "$DATA_DIR" \
  -out-dir "$OUT_DIR" \
  -sender-accounts "$PERF_SENDER_ACCOUNTS" \
  -min-success-rate "$MIN_SUCCESS_RATE" \
  -max-stall-seconds "$MAX_STALL" \
  -max-height-lag "$MAX_LAG" \
  -max-p95-latency-ms "$MAX_P95" \
  -multi-warmup-timeout "$MULTI_WARMUP_TIMEOUT" \
  -multi-warmup-stable-samples "$MULTI_WARMUP_STABLE_SAMPLES"
