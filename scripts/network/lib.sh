#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

log() {
  printf '[network] %s\n' "$*"
}

die() {
  printf '[network] ERROR: %s\n' "$*" >&2
  exit 1
}

usage_common() {
  cat <<'EOF'
Usage:
  scripts/network/dispatch.sh <up|down|reset|ready|logs|status|init|resolve-backend>

Environment:
  TEST_ENV_CONFIG   Path to config YAML (default: config/test_env.yaml, then .example)
  RUNTIME_SESSION_FILE Path to runtime session snapshot (default: data/runtime_session.yaml)
  WAIT_TIMEOUT      Seconds for ready checks (default: 120)
  NODE              Service/process name for logs
EOF
}

to_abs_path() {
  local input="${1:-}"
  if [[ -z "$input" ]]; then
    echo ""
    return 0
  fi

  if [[ "$input" = /* ]]; then
    echo "$input"
    return 0
  fi

  echo "$ROOT_DIR/${input#./}"
}

resolve_config_file() {
  local requested="${1:-${TEST_ENV_CONFIG:-}}"
  local config_file=""

  if [[ -n "$requested" ]]; then
    config_file="$(to_abs_path "$requested")"
  elif [[ -f "$ROOT_DIR/config/test_env.yaml" ]]; then
    config_file="$ROOT_DIR/config/test_env.yaml"
  elif [[ -f "$ROOT_DIR/config/test_env.yaml.example" ]]; then
    config_file="$ROOT_DIR/config/test_env.yaml.example"
  else
    die "config file not found. Expected config/test_env.yaml or config/test_env.yaml.example"
  fi

  [[ -f "$config_file" ]] || die "config file not found: $config_file"
  echo "$config_file"
}

yaml_get_awk() {
  local config_file="$1"
  local key_path="$2"

  awk -v target="$key_path" '
    function ltrim(s) { sub(/^[ \t\r\n]+/, "", s); return s }
    function rtrim(s) { sub(/[ \t\r\n]+$/, "", s); return s }
    function trim(s)  { return rtrim(ltrim(s)) }
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*$/ { next }
    {
      line = $0
      gsub(/\r/, "", line)

      first = match(line, /[^ ]/)
      if (first == 0) next
      indent = first - 1
      body = substr(line, first)

      if (body ~ /^-/) next
      sep = index(body, ":")
      if (sep == 0) next

      key = trim(substr(body, 1, sep - 1))
      if (key !~ /^[-A-Za-z0-9_]+$/) next

      val = trim(substr(body, sep + 1))

      while (depth > 0 && indent <= indents[depth]) depth--
      depth++
      keys[depth] = key
      indents[depth] = indent

      path = keys[1]
      for (i = 2; i <= depth; i++) path = path "." keys[i]

      sub(/[[:space:]]+#.*/, "", val)
      val = trim(val)

      if ((substr(val,1,1) == "\"" && substr(val,length(val),1) == "\"") ||
          (substr(val,1,1) == "'"'"'" && substr(val,length(val),1) == "'\''")) {
        val = substr(val, 2, length(val)-2)
      }

      if (path == target && val != "") {
        print val
        exit
      }
    }
  ' "$config_file"
}

cfg_get() {
  local config_file="$1"
  local key_path="$2"
  local default_value="${3:-}"
  local value=""

  if command -v yq >/dev/null 2>&1; then
    value="$(yq -r ".${key_path} // \"\"" "$config_file" 2>/dev/null || true)"
    [[ "$value" == "null" ]] && value=""
  fi

  if [[ -z "$value" ]]; then
    value="$(yaml_get_awk "$config_file" "$key_path" || true)"
  fi

  if [[ -z "$value" ]]; then
    value="$default_value"
  fi

  echo "$value"
}

cfg_get_json() {
  local config_file="$1"
  local key_path="$2"
  local default_json="${3:-null}"

  python3 - "$config_file" "$key_path" "$default_json" <<'PY'
import json
import sys

try:
    import yaml
except Exception:
    print(sys.argv[3])
    raise SystemExit(0)

config_file, key_path, default_json = sys.argv[1], sys.argv[2], sys.argv[3]

try:
    with open(config_file, "r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh) or {}
except Exception:
    print(default_json)
    raise SystemExit(0)

value = data
for part in key_path.split("."):
    if isinstance(value, dict) and part in value:
        value = value[part]
    else:
        print(default_json)
        raise SystemExit(0)

print(json.dumps(value))
PY
}

runtime_impls_for_topology() {
  local config_file="$1"
  local topology="${2:-multi}"

  python3 - "$config_file" "$topology" <<'PY'
import sys

try:
    import yaml
except Exception:
    print("")
    raise SystemExit(0)

config_file, topology = sys.argv[1], (sys.argv[2] or "multi").strip().lower()

try:
    with open(config_file, "r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh) or {}
except Exception:
    print("")
    raise SystemExit(0)

runtime = data.get("runtime") or {}
network = data.get("network") or {}
runtime_nodes = data.get("runtime_nodes") or {}

default_impl = str(runtime.get("impl") or "geth").strip().lower()
impl_mode = str(runtime.get("impl_mode") or "single").strip().lower()
node_count = 1 if topology == "single" else int(network.get("node_count") or 4)

def node_cfg(idx):
    if isinstance(runtime_nodes, list):
        if idx < len(runtime_nodes) and isinstance(runtime_nodes[idx], dict):
            return runtime_nodes[idx]
        return {}
    if isinstance(runtime_nodes, dict):
        node = runtime_nodes.get(f"node{idx}") or {}
        return node if isinstance(node, dict) else {}
    return {}

impls = []
if impl_mode == "mixed":
    for idx in range(node_count):
        impl = str(node_cfg(idx).get("impl") or default_impl).strip().lower()
        if impl:
            impls.append(impl)
else:
    if default_impl:
        impls.append(default_impl)

print(",".join(sorted(set(impls))))
PY
}

wait_for_rpc_ready() {
  local rpc_url="$1"
  local timeout="${2:-120}"
  local i=0

  command -v curl >/dev/null 2>&1 || die "curl is required for ready checks"

  log "waiting for RPC: $rpc_url (timeout=${timeout}s)"
  while (( i < timeout )); do
    local resp
    resp="$(curl -s --max-time 2 \
      -H 'Content-Type: application/json' \
      --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
      "$rpc_url" || true)"

    if [[ "$resp" == *'"result":"0x'* ]]; then
      log "RPC ready: $rpc_url"
      return 0
    fi

    sleep 1
    i=$((i + 1))
  done

  die "RPC not ready within ${timeout}s: $rpc_url"
}

resolve_runtime_session_file() {
  local requested="${1:-${RUNTIME_SESSION_FILE:-}}"
  local session_file=""

  if [[ -n "$requested" ]]; then
    session_file="$(to_abs_path "$requested")"
  else
    session_file="$ROOT_DIR/data/runtime_session.yaml"
  fi

  echo "$session_file"
}

ensure_go_build_env() {
  export GOCACHE="${GOCACHE:-$ROOT_DIR/.gocache}"
  export GOMODCACHE="${GOMODCACHE:-$ROOT_DIR/.gomodcache}"
  mkdir -p "$GOCACHE" "$GOMODCACHE"
}

runtime_session_exists() {
  local session_file
  session_file="$(resolve_runtime_session_file "${1:-}")"
  [[ -f "$session_file" ]]
}

require_runtime_session() {
  local action="${1:-lifecycle}"
  local session_file
  session_file="$(resolve_runtime_session_file "${2:-}")"
  if [[ ! -f "$session_file" ]]; then
    die "runtime session not found for action '$action': $session_file. Run 'make init' first."
  fi
  echo "$session_file"
}

session_get() {
  local session_file="$1"
  local key_path="$2"
  local default_value="${3:-}"
  cfg_get "$session_file" "$key_path" "$default_value"
}
