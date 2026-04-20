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
PERF_MODE="${PERF_MODE:-tiers}"
TIERS_RAW="${PERF_TPS_TIERS:-$(cfg_get "$CONFIG_FILE" "perf.tps_tiers" "10,30,60")}"
TIERS="$(echo "$TIERS_RAW" | tr -d '[] ' | tr ';' ',' )"
DURATION="${PERF_TIER_DURATION:-90s}"
SAMPLE_INTERVAL_RAW="${PERF_SAMPLE_INTERVAL:-$(cfg_get "$CONFIG_FILE" "perf.sample_interval_seconds" "2")}" 
SAMPLE_INTERVAL="$(normalize_duration_like "$SAMPLE_INTERVAL_RAW")"
DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
TEST_CONFIG="${PERF_TEST_CONFIG:-$DATA_DIR/test_config.yaml}"
OUT_DIR="${PERF_REPORT_DIR:-$ROOT_DIR/reports/perf_$(date +%Y%m%d_%H%M%S)}"
PERF_SCOPE="${PERF_SCOPE:-single}"
PERF_TOPOLOGY="${PERF_TOPOLOGY:-single}"
PERF_INIT_MODE="${PERF_INIT_MODE:-posa}"
PERF_AUTO_STOP="${PERF_AUTO_STOP:-1}"
PERF_SENDER_ACCOUNTS="${PERF_SENDER_ACCOUNTS:-0}"
PERF_MAX_BASE_TPS="${PERF_MAX_BASE_TPS:-1000}"
PERF_MAX_STEP="${PERF_MAX_STEP:-100}"
PERF_MAX_STEP_DURATION="${PERF_MAX_STEP_DURATION:-90s}"
PERF_MAX_TARGET_TPS="${PERF_MAX_TARGET_TPS:-5000}"

MIN_SUCCESS_RATE="$(cfg_get "$CONFIG_FILE" "perf.thresholds.success_rate_min" "0.99")"
MAX_STALL="$(cfg_get "$CONFIG_FILE" "perf.thresholds.stall_window_seconds_max" "15")"
MAX_LAG="$(cfg_get "$CONFIG_FILE" "perf.thresholds.max_height_lag" "8")"
MAX_P95="$(cfg_get "$CONFIG_FILE" "perf.thresholds.rpc_latency_p95_ms_max" "500")"
MULTI_WARMUP_TIMEOUT="${PERF_MULTI_WARMUP_TIMEOUT:-60s}"
MULTI_WARMUP_STABLE_SAMPLES="${PERF_MULTI_WARMUP_STABLE_SAMPLES:-3}"

mkdir -p "$OUT_DIR"

if [[ "$PERF_MODE" == "max" ]]; then
  DURATION="$PERF_MAX_STEP_DURATION"
  echo "Running max TPS exploration: base=$PERF_MAX_BASE_TPS step=$PERF_MAX_STEP target=$PERF_MAX_TARGET_TPS duration=$DURATION topology=$PERF_TOPOLOGY scope=$PERF_SCOPE"
else
  echo "Running TPS tiers: tiers=$TIERS duration=$DURATION sample_interval=$SAMPLE_INTERVAL topology=$PERF_TOPOLOGY scope=$PERF_SCOPE"
fi
echo "Output dir: $OUT_DIR"

perf_cleanup() {
  local rc=$?
  if [[ "$PERF_AUTO_STOP" == "1" || "$PERF_AUTO_STOP" == "true" || "$PERF_AUTO_STOP" == "yes" ]]; then
    if runtime_session_exists "${RUNTIME_SESSION_FILE:-}"; then
      echo "Perf cleanup: stopping runtime..."
      TEST_ENV_CONFIG="$CONFIG_FILE" "$ROOT_DIR/scripts/network/dispatch.sh" down "$CONFIG_FILE" >/dev/null 2>&1 || true
    fi
  fi
  exit "$rc"
}

trap perf_cleanup EXIT INT TERM

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
  if [[ "$PERF_TOPOLOGY" == "single" ]]; then
    echo "Perf setup: topology=single uses native_single background process; pm2 status will stay empty"
  else
    echo "Perf setup: topology=multi uses pm2-managed native runtime"
  fi
  TEST_ENV_CONFIG="$CONFIG_FILE" make --no-print-directory clean >/dev/null
  TEST_ENV_CONFIG="$CONFIG_FILE" TOPOLOGY="$PERF_TOPOLOGY" INIT_MODE="$PERF_INIT_MODE" EPOCH="${EPOCH:-}" make --no-print-directory init >/dev/null
  TEST_ENV_CONFIG="$CONFIG_FILE" make --no-print-directory run >/dev/null
  if ! rpc_ready "$rpc_url"; then
    echo "Perf setup: RPC not ready after run at $rpc_url" >&2
    return 1
  fi
  TEST_ENV_CONFIG="$CONFIG_FILE" make --no-print-directory ready >/dev/null
  if [[ "$PERF_TOPOLOGY" == "single" && -f "$DATA_DIR/native-single/node.pid" ]]; then
    echo "Perf setup: native_single pid=$(cat "$DATA_DIR/native-single/node.pid") log=$DATA_DIR/native-single/node.log"
  fi
}

prepare_perf_runtime

cd "$ROOT_DIR"
go run ./scripts/perf \
  -mode "$PERF_MODE" \
  -scope "$PERF_SCOPE" \
  -topology "$PERF_TOPOLOGY" \
  -config "$TEST_CONFIG" \
  -tiers "$TIERS" \
  -duration "$DURATION" \
  -sample-interval "$SAMPLE_INTERVAL" \
  -data-dir "$DATA_DIR" \
  -out-dir "$OUT_DIR" \
  -sender-accounts "$PERF_SENDER_ACCOUNTS" \
  -max-base-tps "$PERF_MAX_BASE_TPS" \
  -max-step "$PERF_MAX_STEP" \
  -max-target-tps "$PERF_MAX_TARGET_TPS" \
  -min-success-rate "$MIN_SUCCESS_RATE" \
  -max-stall-seconds "$MAX_STALL" \
  -max-height-lag "$MAX_LAG" \
  -max-p95-latency-ms "$MAX_P95" \
  -multi-warmup-timeout "$MULTI_WARMUP_TIMEOUT" \
  -multi-warmup-stable-samples "$MULTI_WARMUP_STABLE_SAMPLES"
