#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

PATTERN="${CHAIN_HEALTH_ERROR_PATTERN:-invalid block|header validation error|invalid header|consensus failure|panic:}"
IGNORE_PATTERN="${CHAIN_HEALTH_IGNORE_PATTERN:-}"
MAX_LINES="${CHAIN_HEALTH_MAX_LINES:-20}"

collect_files() {
  if [[ "$#" -gt 0 ]]; then
    printf '%s\n' "$@"
    return
  fi

  local defaults=(
    "$ROOT_DIR/data/native-logs"/*.log
    "$ROOT_DIR/data/native-single"/*.log
  )

  local f
  for f in "${defaults[@]}"; do
    [[ -f "$f" ]] || continue
    printf '%s\n' "$f"
  done
}

mapfile -t files < <(collect_files "$@")
if [[ "${#files[@]}" -eq 0 ]]; then
  echo "[chain-health] no local log files found, skip"
  exit 0
fi

status=0
for file in "${files[@]}"; do
  [[ -f "$file" ]] || continue
  if ! grep -Eiq "$PATTERN" "$file"; then
    continue
  fi

  if [[ -n "$IGNORE_PATTERN" ]] && grep -Eiq "$PATTERN" "$file" && grep -Eiq "$IGNORE_PATTERN" "$file"; then
    continue
  fi

  echo "[chain-health] error pattern matched in $file"
  grep -Ein "$PATTERN" "$file" | head -n "$MAX_LINES" || true
  status=1
done

if [[ "$status" -ne 0 ]]; then
  echo "[chain-health] detected unhealthy log patterns"
  exit 1
fi

echo "[chain-health] no unhealthy log patterns found"
