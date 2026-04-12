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

scenario_network() {
  local action="${1:?scenario network action required}"
  local config_file="${2:-${CONFIG_FILE:-${TEST_ENV_CONFIG:-}}}"
  local session_file="${3:-${SESSION_FILE:-${RUNTIME_SESSION_FILE:-}}}"
  local effective_config="$config_file"

  [[ -n "$config_file" ]] || die "scenario network action '$action' requires CONFIG_FILE or TEST_ENV_CONFIG"

  if [[ "$action" == "init" && -n "$session_file" && -f "$session_file" ]]; then
    effective_config="$session_file"
  fi

  TEST_ENV_CONFIG="$effective_config" \
  RUNTIME_SESSION_FILE="$session_file" \
  bash "$ROOT_DIR/scripts/network/dispatch.sh" "$action"
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
    scenario_network down >/dev/null 2>&1 || true
  fi

  return "$rc"
}

scenario_rpc_urls() {
  local config_file="${1:?config file required}"
  python3 - "$config_file" <<'PY'
import sys

cfg_path = sys.argv[1]
entries = []
seen = set()
section = None
current_node = None
generic_rpcs = []
has_specific_rpc = False

def strip_quotes(value):
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value

def add(name, role, url):
    global has_specific_rpc
    url = strip_quotes(url or "")
    if not url or url in seen:
        return
    if role in ("validator", "sync"):
        has_specific_rpc = True
    entries.append(((name or url).strip(), (role or "unknown").strip(), url))
    seen.add(url)

def flush_node():
    global current_node
    if current_node and current_node.get("url"):
        add(current_node.get("name"), current_node.get("role"), current_node.get("url"))
    current_node = None

with open(cfg_path, "r", encoding="utf-8") as fh:
    for raw in fh:
        line = raw.rstrip("\n")
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        if not line.startswith(" "):
            flush_node()
            if stripped.startswith("validator_rpcs:"):
                section = "validator_rpcs"
            elif stripped.startswith("sync_rpc:"):
                section = "sync_rpc"
                _, value = stripped.split(":", 1)
                add("sync1", "sync", value)
                section = None
            elif stripped.startswith("node_rpcs:"):
                section = "node_rpcs"
            elif stripped.startswith("rpcs:"):
                section = "rpcs"
            else:
                section = None
            continue

        if section in {"validator_rpcs", "rpcs"}:
            if stripped.startswith("- "):
                url = stripped[2:].strip()
                if section == "validator_rpcs":
                    name = f"validator{sum(1 for _, r, _ in entries if r == 'validator') + 1}"
                    add(name, "validator", url)
                else:
                    generic_rpcs.append(url)
            continue

        if section == "node_rpcs":
            if stripped.startswith("- "):
                flush_node()
                current_node = {"name": "", "role": "unknown", "url": ""}
                remainder = stripped[2:].strip()
                if remainder and ":" in remainder:
                    key, value = remainder.split(":", 1)
                    current_node[key.strip()] = strip_quotes(value)
                continue

            if current_node and ":" in stripped:
                key, value = stripped.split(":", 1)
                current_node[key.strip()] = strip_quotes(value)

flush_node()

if not has_specific_rpc:
    for url in generic_rpcs:
        add(None, "rpc", url)

for _, _, url in entries:
    print(url)
PY
}

scenario_rpc_entries() {
  local config_file="${1:?config file required}"
  python3 - "$config_file" <<'PY'
import sys

cfg_path = sys.argv[1]
entries = []
seen = set()
section = None
current_node = None
validator_idx = 0
rpc_idx = 0
generic_rpcs = []
has_specific_rpc = False

def strip_quotes(value):
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value

def add(name, role, url):
    global validator_idx, rpc_idx, has_specific_rpc
    url = strip_quotes(url or "")
    role = (role or "unknown").strip()
    if not url or url in seen:
        return
    if role in ("validator", "sync"):
        has_specific_rpc = True
    if not name:
        if role == "validator":
            validator_idx += 1
            name = f"validator{validator_idx}"
        elif role == "rpc":
            rpc_idx += 1
            name = f"rpc{rpc_idx}"
        elif role == "sync":
            name = "sync1"
        else:
            name = url
    entries.append((name.strip(), role, url))
    seen.add(url)

def flush_node():
    global current_node
    if current_node and current_node.get("url"):
        add(current_node.get("name"), current_node.get("role"), current_node.get("url"))
    current_node = None

with open(cfg_path, "r", encoding="utf-8") as fh:
    for raw in fh:
        line = raw.rstrip("\n")
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        if not line.startswith(" "):
            flush_node()
            if stripped.startswith("validator_rpcs:"):
                section = "validator_rpcs"
            elif stripped.startswith("sync_rpc:"):
                section = "sync_rpc"
                _, value = stripped.split(":", 1)
                add("sync1", "sync", value)
                section = None
            elif stripped.startswith("node_rpcs:"):
                section = "node_rpcs"
            elif stripped.startswith("rpcs:"):
                section = "rpcs"
            else:
                section = None
            continue

        if section in {"validator_rpcs", "rpcs"}:
            if stripped.startswith("- "):
                url = stripped[2:].strip()
                if section == "validator_rpcs":
                    add(None, "validator", url)
                else:
                    generic_rpcs.append(url)
            continue

        if section == "node_rpcs":
            if stripped.startswith("- "):
                flush_node()
                current_node = {"name": "", "role": "unknown", "url": ""}
                remainder = stripped[2:].strip()
                if remainder and ":" in remainder:
                    key, value = remainder.split(":", 1)
                    current_node[key.strip()] = strip_quotes(value)
                continue

            if current_node and ":" in stripped:
                key, value = stripped.split(":", 1)
                current_node[key.strip()] = strip_quotes(value)

flush_node()

if not has_specific_rpc:
    for url in generic_rpcs:
        add(None, "rpc", url)

for name, role, url in entries:
    print(f"{name}\t{role}\t{url}")
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

hex_to_dec() {
  local raw="${1:-}"
  raw="${raw#0x}"
  if [[ -z "$raw" ]]; then
    echo 0
    return 0
  fi
  printf '%d\n' "$((16#$raw))"
}

wait_for_scenario_rpc_stability() {
  local config_file="${1:?config file required}"
  local timeout="${2:-30}"
  local required_rounds="${3:-3}"
  local min_validator_peers="${SCENARIO_BOOTSTRAP_MIN_VALIDATOR_PEERS:-1}"
  local max_height_delta="${SCENARIO_BOOTSTRAP_MAX_HEIGHT_DELTA:-2}"
  local deadline=$((SECONDS + timeout))
  local stable_rounds=0
  local last_summary=""
  local rpc_entries=()
  local validator_total=0

  mapfile -t rpc_entries < <(scenario_rpc_entries "$config_file")
  [[ ${#rpc_entries[@]} -gt 0 ]] || die "bootstrap rpc never stabilized: no rpc urls resolved from $config_file"
  local rpc_entry
  for rpc_entry in "${rpc_entries[@]}"; do
    if [[ "$rpc_entry" == *$'\tvalidator\t'* ]]; then
      validator_total=$((validator_total + 1))
    fi
  done

  scenario_log "waiting for RPC stabilization rounds=${required_rounds} timeout=${timeout}s"
  while (( SECONDS <= deadline )); do
    local all_ok=1
    local summary_parts=()
    local validator_min_height=-1
    local validator_max_height=-1
    local validator_count=0
    local entry name role rpc_url chain_id block_number peer_hex block_dec peer_dec

    for entry in "${rpc_entries[@]}"; do
      IFS=$'\t' read -r name role rpc_url <<< "$entry"
      chain_id="$(rpc_hex_result "$rpc_url" "eth_chainId" || true)"
      block_number="$(rpc_hex_result "$rpc_url" "eth_blockNumber" || true)"
      if [[ -z "$chain_id" || -z "$block_number" ]]; then
        all_ok=0
        summary_parts+=("${name}:unready")
      else
        block_dec="$(hex_to_dec "$block_number")"
        if [[ "$role" == "validator" ]]; then
          peer_hex="$(rpc_hex_result "$rpc_url" "net_peerCount" || true)"
          if [[ -z "$peer_hex" ]]; then
            all_ok=0
            summary_parts+=("${name}:block=${block_dec}:peers=unready")
            continue
          fi
          peer_dec="$(hex_to_dec "$peer_hex")"
          summary_parts+=("${name}:block=${block_dec}:peers=${peer_dec}")
          validator_count=$((validator_count + 1))
          if (( validator_total > 1 && peer_dec < min_validator_peers )); then
            all_ok=0
          fi
          if (( validator_min_height < 0 || block_dec < validator_min_height )); then
            validator_min_height=$block_dec
          fi
          if (( validator_max_height < 0 || block_dec > validator_max_height )); then
            validator_max_height=$block_dec
          fi
        else
          summary_parts+=("${name}:block=${block_dec}")
        fi
      fi
    done

    if (( validator_count > 1 )) && (( validator_min_height >= 0 && validator_max_height >= 0 )); then
      if (( validator_max_height - validator_min_height > max_height_delta )); then
        all_ok=0
        summary_parts+=("validator_height_delta=$((validator_max_height - validator_min_height))")
      fi
    fi

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
