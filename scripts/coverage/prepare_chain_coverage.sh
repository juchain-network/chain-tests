#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/coverage/lib.sh
source "$SCRIPT_DIR/lib.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/coverage/prepare_chain_coverage.sh [--config <file>] [--print-binary|--print-out-dir|--print-raw-root|--print-node-dir <node>]
EOF
}

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
ACTION="prepare"
NODE_KEY=""

coverage_apply_session_defaults

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      shift
      [[ $# -gt 0 ]] || die "missing value for --config"
      CONFIG_FILE="$(resolve_config_file "$1")"
      ;;
    --print-binary)
      ACTION="print-binary"
      ;;
    --print-out-dir)
      ACTION="print-out-dir"
      ;;
    --print-raw-root)
      ACTION="print-raw-root"
      ;;
    --print-node-dir)
      shift
      [[ $# -gt 0 ]] || die "missing value for --print-node-dir"
      ACTION="print-node-dir"
      NODE_KEY="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unsupported argument: $1"
      ;;
  esac
  shift
done

coverage_scope >/dev/null
coverage_assert_supported_runtime "$CONFIG_FILE"
coverage_prepare_dirs

OUT_DIR="$(coverage_out_dir)"
RAW_ROOT="$(coverage_raw_root)"
BINARY_PATH="$(coverage_binary_path)"
META_PATH="$(coverage_build_meta_path)"
CHAIN_ROOT="$(coverage_chain_root "$CONFIG_FILE")"
EXPECTED_COVERPKG="./cmd/geth,./consensus/congress/..."

[[ -d "$CHAIN_ROOT" ]] || die "chain root not found: $CHAIN_ROOT"

build_binary_if_needed() {
  local latest_src_ts binary_ts current_coverpkg
  latest_src_ts="$(coverage_latest_source_ts "$CHAIN_ROOT")"
  binary_ts=0
  if [[ -f "$BINARY_PATH" ]]; then
    binary_ts="$(stat -c %Y "$BINARY_PATH" 2>/dev/null || echo 0)"
  fi
  current_coverpkg=""
  if [[ -f "$META_PATH" ]]; then
    current_coverpkg="$(awk -F= '$1=="coverpkg" {print $2; exit}' "$META_PATH" 2>/dev/null || true)"
  fi

  if [[ ! -x "$BINARY_PATH" || "$binary_ts" -lt "$latest_src_ts" || "$current_coverpkg" != "$EXPECTED_COVERPKG" ]]; then
    mkdir -p "$(dirname "$BINARY_PATH")"
    echo "[coverage] building congress coverage geth: $BINARY_PATH" >&2
    (
      cd "$CHAIN_ROOT"
      GOCACHE="${GOCACHE:-/tmp/go-build}" \
      go build -cover -coverpkg="$EXPECTED_COVERPKG" -o "$BINARY_PATH" ./cmd/geth
    )
    cat >"$META_PATH" <<EOF_META
scope=$(coverage_scope)
chain_root=$CHAIN_ROOT
binary=$BINARY_PATH
built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
source_ts=$latest_src_ts
coverpkg=$EXPECTED_COVERPKG
EOF_META
  fi
}

case "$ACTION" in
  print-out-dir)
    echo "$OUT_DIR"
    ;;
  print-raw-root)
    echo "$RAW_ROOT"
    ;;
  print-node-dir)
    echo "$(coverage_node_dir "$NODE_KEY")"
    ;;
  print-binary|prepare)
    build_binary_if_needed
    echo "$BINARY_PATH"
    ;;
  *)
    die "unsupported action: $ACTION"
    ;;
esac
