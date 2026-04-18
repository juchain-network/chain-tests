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

runtime_case_support_report() {
  local config_file="$1"
  local topology="${2:-multi}"
  local mode="${3:-}"
  local target="${4:-}"

  python3 - "$config_file" "$topology" "$mode" "$target" "$ROOT_DIR" <<'PY'
import json
import os
import re
import subprocess
import sys

try:
    import yaml
except Exception as exc:
    raise SystemExit(f"yaml support is required for runtime capability resolution: {exc}")

config_file, topology, mode, target, root_dir = sys.argv[1:]
topology = (topology or "multi").strip().lower()
mode = (mode or "").strip()
target = (target or "").strip()

with open(config_file, "r", encoding="utf-8") as fh:
    data = yaml.safe_load(fh) or {}

runtime = data.get("runtime") or {}
network = data.get("network") or {}
paths = data.get("paths") or {}
binaries = data.get("binaries") or {}
native = data.get("native") or {}
runtime_nodes = data.get("runtime_nodes") or {}
runtime_capability = data.get("runtime_capability") or {}

fork_order = runtime_capability.get("fork_order") or [
    "poa",
    "shanghai",
    "cancun",
    "fixHeader",
    "posa",
    "prague",
    "osaka",
    "bpo1",
    "bpo2",
]
fork_rank = {name: idx for idx, name in enumerate(fork_order)}

default_version_matrix = {
    "geth": {
        "1.13.x": "posa",
        "1.16.x": "osaka",
    },
    "reth": {
        "1.1.x": "osaka",
        "1.11.x": "osaka",
    },
}

version_matrix = {}
for impl, mapping in default_version_matrix.items():
    version_matrix[impl] = dict(mapping)

cfg_matrix = runtime_capability.get("version_matrix") or {}
if isinstance(cfg_matrix, dict):
    for impl, mapping in cfg_matrix.items():
        if isinstance(mapping, dict):
            merged = dict(version_matrix.get(impl, {}))
            merged.update(mapping)
            version_matrix[impl] = merged

default_impl = str(runtime.get("impl") or "geth").strip().lower()
impl_mode = str(runtime.get("impl_mode") or "single").strip().lower()
node_count = 1 if topology == "single" else int(network.get("node_count") or 4)

def to_abs(path_value: str) -> str:
    if not path_value:
        return ""
    if os.path.isabs(path_value):
        return path_value
    return os.path.join(root_dir, path_value[2:] if path_value.startswith("./") else path_value)

def node_cfg(idx):
    if isinstance(runtime_nodes, list):
        if idx < len(runtime_nodes) and isinstance(runtime_nodes[idx], dict):
            return runtime_nodes[idx]
        return {}
    if isinstance(runtime_nodes, dict):
        node = runtime_nodes.get(f"node{idx}") or {}
        return node if isinstance(node, dict) else {}
    return {}

def resolve_impl(idx):
    if impl_mode == "mixed":
        impl = str(node_cfg(idx).get("impl") or default_impl).strip().lower()
    else:
        impl = default_impl
    if impl not in {"geth", "reth"}:
        raise SystemExit(f"unsupported runtime implementation for node{idx}: {impl}")
    return impl

def resolve_binary(idx, impl):
    node = node_cfg(idx)
    node_binary = to_abs(str(node.get("binary") or "").strip())
    if node_binary:
        return node_binary

    if impl == "geth":
        configured = to_abs(str(binaries.get("geth_native") or native.get("geth_binary") or "").strip())
        if configured:
            return configured
        chain_root = to_abs(str(paths.get("chain_root") or "../chain").strip())
        return os.path.join(chain_root, "build", "bin", "geth")

    configured = to_abs(str(binaries.get("reth_native") or native.get("reth_binary") or "").strip())
    if configured:
        return configured
    reth_root = to_abs(str(paths.get("reth_root") or "../rchain").strip())
    return os.path.join(reth_root, "target", "release", "congress-node")

version_cache = {}

def detect_version(binary, impl):
    cache_key = (binary, impl)
    if cache_key in version_cache:
        return version_cache[cache_key]
    if not os.path.exists(binary):
        raise SystemExit(f"runtime binary not found for {impl}: {binary}")

    commands = [[binary, "version"]] if impl == "geth" else [[binary, "--version"], [binary, "version"]]
    output = ""
    last_err = None
    for cmd in commands:
        try:
            proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
            output = (proc.stdout or "") + "\n" + (proc.stderr or "")
            if output.strip():
                break
        except Exception as exc:
            last_err = exc
    if not output.strip():
        raise SystemExit(f"failed to read version from {binary}: {last_err}")

    match = re.search(r'(\d+)\.(\d+)\.(\d+)', output)
    if not match:
        raise SystemExit(f"failed to parse semver from {binary} output: {output.strip()}")
    version = ".".join(match.groups())
    version_cache[cache_key] = version
    return version

def pattern_matches(version, pattern):
    pattern = str(pattern or "").strip().lower()
    if pattern in {"", "*", "default"}:
        return True, 0

    vparts = version.split(".")
    pparts = pattern.split(".")
    exact = 0
    for idx, part in enumerate(pparts):
        if part in {"x", "*"}:
            return True, exact
        if idx >= len(vparts) or vparts[idx] != part:
            return False, -1
        exact += 1
    if len(pparts) > len(vparts):
        return False, -1
    return True, exact

def resolve_max_fork(impl, version):
    mapping = version_matrix.get(impl) or {}
    best = None
    best_score = -1
    for pattern, max_fork in mapping.items():
        matched, score = pattern_matches(version, pattern)
        if matched and score > best_score:
            best = max_fork
            best_score = score
    if not best:
        raise SystemExit(f"no runtime capability mapping for {impl} version {version}")
    if best not in fork_rank:
        raise SystemExit(f"unknown max_fork {best} for {impl} version {version}")
    return best

required_fork_map = {
    ("poa", ""): "poa",
    ("posa", ""): "posa",
    ("smoke", "poa"): "poa",
    ("smoke", "poa_shanghai"): "shanghai",
    ("smoke", "poa_shanghai_cancun"): "cancun",
    ("smoke", "poa_shanghai_cancun_fixheader"): "fixHeader",
    ("smoke", "poa_shanghai_cancun_fixheader_posa"): "posa",
    ("smoke", "poa_shanghai_cancun_fixheader_posa_prague"): "prague",
    ("smoke", "poa_shanghai_cancun_fixheader_posa_prague_osaka"): "osaka",
    ("smoke", "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1"): "bpo1",
    ("smoke", "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2"): "bpo2",
    ("upgrade", "shanghaiTime"): "shanghai",
    ("upgrade", "cancunTime"): "cancun",
    ("upgrade", "fixHeaderTime"): "fixHeader",
    ("upgrade", "posaTime"): "posa",
    ("upgrade", "pragueTime"): "prague",
    ("upgrade", "osakaTime"): "osaka",
    ("upgrade", "bpo1Time"): "bpo1",
    ("upgrade", "bpo2Time"): "bpo2",
    ("upgrade", "allSame"): "bpo2",
    ("upgrade", "allStaggered"): "bpo2",
}

required_fork = required_fork_map.get((mode, target))
if required_fork is None:
    raise SystemExit(f"unsupported runtime capability query for mode={mode} target={target}")

nodes = []
network_max = None
for idx in range(node_count):
    impl = resolve_impl(idx)
    binary = resolve_binary(idx, impl)
    version = detect_version(binary, impl)
    max_fork = resolve_max_fork(impl, version)
    nodes.append({
        "node": f"node{idx}",
        "impl": impl,
        "binary": binary,
        "version": version,
        "max_fork": max_fork,
    })
    if network_max is None or fork_rank[max_fork] < fork_rank[network_max]:
        network_max = max_fork

print(json.dumps({
    "topology": topology,
    "mode": mode,
    "target": target,
    "required_fork": required_fork,
    "network_max_fork": network_max,
    "supported": fork_rank[network_max] >= fork_rank[required_fork],
    "nodes": nodes,
}))
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
