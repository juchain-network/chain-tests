#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/../network/lib.sh"

coverage_bool_true() {
  case "$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

coverage_enabled() {
  coverage_bool_true "${CHAIN_COVERAGE:-0}"
}

coverage_scope() {
  local scope="${CHAIN_COVERAGE_SCOPE:-congress}"
  case "$scope" in
    congress) echo "$scope" ;;
    *)
      printf '[coverage] ERROR: unsupported CHAIN_COVERAGE_SCOPE=%s (expected: congress)\n' "$scope" >&2
      exit 2
      ;;
  esac
}

coverage_out_dir() {
  local configured="${CHAIN_COVERAGE_OUT_DIR:-}"
  if [[ -n "$configured" ]]; then
    to_abs_path "$configured"
    return 0
  fi
  echo "$ROOT_DIR/reports/coverage_manual"
}

coverage_raw_root() {
  echo "$(coverage_out_dir)/raw"
}

coverage_node_dir() {
  local node_key="${1:-}"
  [[ -n "$node_key" ]] || {
    printf '[coverage] ERROR: missing coverage node key\n' >&2
    exit 2
  }
  echo "$(coverage_raw_root)/${node_key}"
}

coverage_state_root() {
  echo "$ROOT_DIR/reports/.coverage_state"
}

coverage_binary_path() {
  if [[ -n "${CHAIN_COVERAGE_GETH_BINARY:-}" ]]; then
    to_abs_path "$CHAIN_COVERAGE_GETH_BINARY"
    return 0
  fi
  echo "$(coverage_state_root)/bin/geth-cover-$(coverage_scope)"
}

coverage_build_meta_path() {
  echo "$(coverage_state_root)/bin/geth-cover-$(coverage_scope).meta"
}

coverage_chain_root() {
  local config_file="$1"
  to_abs_path "$(cfg_get "$config_file" "paths.chain_root" "../chain")"
}

coverage_runtime_backend() {
  echo "native"
}

coverage_assert_supported_runtime() {
  local config_file="$1"
  local impl_mode default_impl node_count impl node_cfg
  impl_mode="$(cfg_get "$config_file" "runtime.impl_mode" "single")"
  default_impl="$(cfg_get "$config_file" "runtime.impl" "geth")"
  node_count="$(cfg_get "$config_file" "network.node_count" "4")"
  [[ "$node_count" =~ ^[0-9]+$ ]] || node_count=4

  for ((i=0; i<node_count; i++)); do
    node_cfg="$(cfg_get "$config_file" "runtime_nodes.node${i}.impl" "")"
    case "$impl_mode" in
      single)
        impl="$default_impl"
        ;;
      mixed)
        if [[ -n "$node_cfg" ]]; then
          impl="$node_cfg"
        else
          printf '[coverage] ERROR: runtime_nodes.node%d.impl is required when runtime.impl_mode=mixed\n' "$i" >&2
          exit 2
        fi
        ;;
      *)
        printf '[coverage] ERROR: unsupported runtime.impl_mode=%s\n' "$impl_mode" >&2
        exit 2
        ;;
    esac
    if [[ "$impl" != "geth" ]]; then
      printf '[coverage] ERROR: CHAIN_COVERAGE=1 only supports native geth; node%d impl=%s\n' "$i" "$impl" >&2
      exit 2
    fi
  done
}

coverage_latest_source_ts() {
  local chain_root="$1"
  python3 - "$chain_root" <<'PY'
import os
import sys

root = sys.argv[1]
paths = [
    os.path.join(root, "go.mod"),
    os.path.join(root, "go.sum"),
    os.path.join(root, "cmd", "geth"),
    os.path.join(root, "consensus", "congress"),
]
latest = 0
for path in paths:
    if not os.path.exists(path):
        continue
    if os.path.isfile(path):
        latest = max(latest, int(os.path.getmtime(path)))
        continue
    for base, _, files in os.walk(path):
        for name in files:
            if name.endswith(".go") or name in {"go.mod", "go.sum"}:
                full = os.path.join(base, name)
                latest = max(latest, int(os.path.getmtime(full)))
print(latest)
PY
}

coverage_prepare_dirs() {
  local out_dir raw_root state_root
  out_dir="$(coverage_out_dir)"
  raw_root="$(coverage_raw_root)"
  state_root="$(coverage_state_root)"
  mkdir -p "$out_dir" "$raw_root" "$state_root" "$(dirname "$(coverage_binary_path)")"
}

coverage_session_file() {
  echo "$(coverage_state_root)/session.env"
}

coverage_session_active() {
  [[ -f "$(coverage_session_file)" ]]
}

coverage_apply_session_defaults() {
  local session_file
  session_file="$(coverage_session_file)"
  [[ -f "$session_file" ]] || return 0

  # shellcheck disable=SC1090
  source "$session_file"

  if [[ -z "${CHAIN_COVERAGE:-}" && -n "${SESSION_CHAIN_COVERAGE:-}" ]]; then
    export CHAIN_COVERAGE="$SESSION_CHAIN_COVERAGE"
  fi
  if [[ -z "${CHAIN_COVERAGE_SESSION:-}" && -n "${SESSION_CHAIN_COVERAGE_SESSION:-}" ]]; then
    export CHAIN_COVERAGE_SESSION="$SESSION_CHAIN_COVERAGE_SESSION"
  fi
  if [[ -z "${CHAIN_COVERAGE_SCOPE:-}" && -n "${SESSION_CHAIN_COVERAGE_SCOPE:-}" ]]; then
    export CHAIN_COVERAGE_SCOPE="$SESSION_CHAIN_COVERAGE_SCOPE"
  fi
  if [[ -z "${CHAIN_COVERAGE_OUT_DIR:-}" && -n "${SESSION_CHAIN_COVERAGE_OUT_DIR:-}" ]]; then
    export CHAIN_COVERAGE_OUT_DIR="$SESSION_CHAIN_COVERAGE_OUT_DIR"
  fi
  if [[ -z "${CHAIN_COVERAGE_KEEP_RAW:-}" && -n "${SESSION_CHAIN_COVERAGE_KEEP_RAW:-}" ]]; then
    export CHAIN_COVERAGE_KEEP_RAW="$SESSION_CHAIN_COVERAGE_KEEP_RAW"
  fi
}
