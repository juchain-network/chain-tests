#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$ROOT_DIR/scripts/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
TIERS_RAW="${PERF_TPS_TIERS:-$(cfg_get "$CONFIG_FILE" "perf.tps_tiers" "10,30,60")}"
TIERS="$(echo "$TIERS_RAW" | tr -d '[] ' | tr ';' ',' )"
DURATION="${PERF_TIER_DURATION:-90s}"
SAMPLE_INTERVAL="${PERF_SAMPLE_INTERVAL:-$(cfg_get "$CONFIG_FILE" "perf.sample_interval_seconds" "2")s}"
DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
TEST_CONFIG="${PERF_TEST_CONFIG:-$DATA_DIR/test_config.yaml}"
OUT_DIR="${PERF_REPORT_DIR:-$ROOT_DIR/reports/perf_$(date +%Y%m%d_%H%M%S)}"

MIN_SUCCESS_RATE="$(cfg_get "$CONFIG_FILE" "perf.thresholds.success_rate_min" "0.99")"
MAX_STALL="$(cfg_get "$CONFIG_FILE" "perf.thresholds.stall_window_seconds_max" "15")"
MAX_LAG="$(cfg_get "$CONFIG_FILE" "perf.thresholds.max_height_lag" "8")"
MAX_P95="$(cfg_get "$CONFIG_FILE" "perf.thresholds.rpc_latency_p95_ms_max" "500")"

mkdir -p "$OUT_DIR"

echo "Running TPS tiers: tiers=$TIERS duration=$DURATION sample_interval=$SAMPLE_INTERVAL"
echo "Output dir: $OUT_DIR"

rpc_ready() {
  local rpc_url="$1"
  local resp
  resp="$(curl -s --max-time 2 \
    -H 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
    "$rpc_url" || true)"
  [[ "$resp" == *'"result":"0x'* ]]
}

ensure_perf_runtime_ready() {
  local rpc_url
  rpc_url="$(cfg_get "$CONFIG_FILE" "network.external_rpc" "http://localhost:18545")"

  if [[ ! -f "$TEST_CONFIG" ]]; then
    echo "Perf precheck: generated test config not found at $TEST_CONFIG, generating network config..."
    TEST_ENV_CONFIG="$CONFIG_FILE" TEST_NETWORK_EPOCH="${EPOCH:-}" bash "$ROOT_DIR/scripts/gen_network_config.sh" "$CONFIG_FILE"
  fi

  if ! rpc_ready "$rpc_url"; then
    echo "Perf precheck: RPC not ready at $rpc_url, starting network..."
    TEST_ENV_CONFIG="$CONFIG_FILE" "$ROOT_DIR/scripts/network/dispatch.sh" up "$CONFIG_FILE"
  fi

  TEST_ENV_CONFIG="$CONFIG_FILE" "$ROOT_DIR/scripts/network/dispatch.sh" ready "$CONFIG_FILE"
}

ensure_perf_runtime_ready

cd "$ROOT_DIR"
go run ./scripts/perf/perf_runner.go \
  -mode tiers \
  -config "$TEST_CONFIG" \
  -tiers "$TIERS" \
  -duration "$DURATION" \
  -sample-interval "$SAMPLE_INTERVAL" \
  -data-dir "$DATA_DIR" \
  -out-dir "$OUT_DIR" \
  -min-success-rate "$MIN_SUCCESS_RATE" \
  -max-stall-seconds "$MAX_STALL" \
  -max-height-lag "$MAX_LAG" \
  -max-p95-latency-ms "$MAX_P95"
