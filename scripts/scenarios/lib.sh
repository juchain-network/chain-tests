#!/usr/bin/env bash

sanitize_scenario_name() {
  local value="${1:-}"
  echo "$value" | tr '[:space:]' '_' | tr -c 'a-zA-Z0-9._:-' '_' | tr ':' '_'
}

scenario_report_root() {
  local name="$1"
  local slug
  slug="$(sanitize_scenario_name "$name")"

  if [[ -n "${SCENARIO_REPORT_DIR:-}" ]]; then
    to_abs_path "$SCENARIO_REPORT_DIR"
    return 0
  fi
  if [[ -n "${REPORT_DIR:-}" ]]; then
    to_abs_path "$REPORT_DIR"
    return 0
  fi
  echo "$ROOT_DIR/reports/scenario_${slug}_$(date +%Y%m%d_%H%M%S)"
}

scenario_init() {
  SCENARIO_NAME="${1:?scenario name required}"
  SCENARIO_REPORT_ROOT="${SCENARIO_REPORT_ROOT:-$(scenario_report_root "$SCENARIO_NAME")}"
  mkdir -p "$SCENARIO_REPORT_ROOT"
  scenario_select_case "${2:-main}"
}

scenario_select_case() {
  local label="${1:-main}"
  local slug
  slug="$(sanitize_scenario_name "$label")"
  SCENARIO_CASE_LABEL="$label"
  SCENARIO_CASE_DIR="$SCENARIO_REPORT_ROOT/$slug"
  mkdir -p "$SCENARIO_CASE_DIR"
  export SCENARIO_STAGE_FILE="$SCENARIO_CASE_DIR/stage.txt"
  scenario_mark_stage "${SCENARIO_STAGE:-startup/bootstrap}"
}

scenario_mark_stage() {
  SCENARIO_STAGE="${1:-unknown}"
  if [[ -n "${SCENARIO_STAGE_FILE:-}" ]]; then
    printf '%s\n' "$SCENARIO_STAGE" > "$SCENARIO_STAGE_FILE"
  fi
}

scenario_current_stage() {
  if [[ -n "${SCENARIO_STAGE_FILE:-}" && -f "${SCENARIO_STAGE_FILE:-}" ]]; then
    head -n 1 "$SCENARIO_STAGE_FILE"
    return 0
  fi
  printf '%s\n' "${SCENARIO_STAGE:-unknown}"
}

scenario_log() {
  printf '[scenario/%s] %s\n' "${SCENARIO_NAME:-unknown}" "$*"
}

archive_log_tree() {
  local src_dir="$1"
  local dst_dir="$2"
  [[ -d "$src_dir" ]] || return 0

  local file rel parent
  while IFS= read -r file; do
    rel="${file#$src_dir/}"
    parent="$(dirname "$dst_dir/$rel")"
    mkdir -p "$parent"
    cp "$file" "$dst_dir/$rel"
  done < <(find "$src_dir" -type f -name '*.log' | sort)
}

archive_scenario_artifacts() {
  local status="${1:-FAIL}"
  local case_dir="${SCENARIO_CASE_DIR:-$SCENARIO_REPORT_ROOT}"
  local current_stage
  current_stage="$(scenario_current_stage)"
  SCENARIO_STAGE="$current_stage"
  mkdir -p "$case_dir"

  local files=(
    "$ROOT_DIR/data/runtime_session.yaml"
    "$ROOT_DIR/data/runtime_session.json"
    "$ROOT_DIR/data/test_config.yaml"
    "$ROOT_DIR/data/runtime_nodes.yaml"
    "$ROOT_DIR/data/genesis.json"
  )

  local file
  for file in "${files[@]}"; do
    if [[ -f "$file" ]]; then
      cp "$file" "$case_dir/$(basename "$file")"
    fi
  done

  archive_log_tree "$ROOT_DIR/data/native-logs" "$case_dir/native-logs"
  archive_log_tree "$ROOT_DIR/data/native-single" "$case_dir/native-single"

  cat > "$case_dir/scenario_meta.txt" <<EOF
scenario=${SCENARIO_NAME:-unknown}
case=${SCENARIO_CASE_LABEL:-main}
status=$status
stage=$current_stage
timestamp=$(date +%Y-%m-%dT%H:%M:%S%z)
EOF
}

scenario_cleanup() {
  local rc="${1:-0}"
  set +e

  if (( rc != 0 )); then
    archive_scenario_artifacts "FAIL"
    scenario_log "archived failure artifacts stage=$(scenario_current_stage) dir=${SCENARIO_CASE_DIR:-$SCENARIO_REPORT_ROOT}"
  fi

  if [[ -n "${SESSION_FILE:-}" && -f "${SESSION_FILE:-}" ]]; then
    bash "$ROOT_DIR/scripts/network/native.sh" down "$SESSION_FILE" >/dev/null 2>&1 || true
  fi

  return "$rc"
}

scenario_rpc_urls() {
  local config_file="${1:?config file required}"
  python3 - "$config_file" <<'PY'
import sys
import yaml

cfg_path = sys.argv[1]
with open(cfg_path, "r", encoding="utf-8") as fh:
    cfg = yaml.safe_load(fh) or {}

urls = []

def add(url):
    url = (url or "").strip()
    if url and url not in urls:
        urls.append(url)

for url in cfg.get("validator_rpcs") or []:
    add(url)
add(cfg.get("sync_rpc"))
for item in cfg.get("node_rpcs") or []:
    add((item or {}).get("url"))
for url in cfg.get("rpcs") or []:
    add(url)

for url in urls:
    print(url)
PY
}

rpc_hex_result() {
  local rpc_url="$1"
  local method="$2"
  local response
  response="$(curl -s --max-time 3 \
    -H 'Content-Type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":[]}" \
    "$rpc_url" || true)"

  if [[ "$response" =~ \"result\":\"(0x[0-9a-fA-F]+)\" ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

wait_for_scenario_rpc_stability() {
  local config_file="${1:?config file required}"
  local timeout="${2:-30}"
  local required_rounds="${3:-3}"
  local deadline=$((SECONDS + timeout))
  local stable_rounds=0
  local last_summary=""

  mapfile -t rpc_urls < <(scenario_rpc_urls "$config_file")
  [[ ${#rpc_urls[@]} -gt 0 ]] || die "bootstrap rpc never stabilized: no rpc urls resolved from $config_file"

  scenario_log "waiting for RPC stabilization rounds=${required_rounds} timeout=${timeout}s"
  while (( SECONDS <= deadline )); do
    local all_ok=1
    local summary_parts=()
    local rpc_url chain_id block_number

    for rpc_url in "${rpc_urls[@]}"; do
      chain_id="$(rpc_hex_result "$rpc_url" "eth_chainId" || true)"
      block_number="$(rpc_hex_result "$rpc_url" "eth_blockNumber" || true)"
      if [[ -z "$chain_id" || -z "$block_number" ]]; then
        all_ok=0
        summary_parts+=("$rpc_url:unready")
      else
        summary_parts+=("$rpc_url:chainId=$chain_id:block=$block_number")
      fi
    done

    last_summary="$(IFS=' '; echo "${summary_parts[*]}")"
    if (( all_ok == 1 )); then
      stable_rounds=$((stable_rounds + 1))
      if (( stable_rounds >= required_rounds )); then
        scenario_log "RPC stabilization complete ${last_summary}"
        return 0
      fi
    else
      stable_rounds=0
    fi
    sleep 1
  done

  die "bootstrap rpc never stabilized after ${timeout}s: ${last_summary}"
}
