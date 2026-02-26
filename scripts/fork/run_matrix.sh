#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$PROJECT_ROOT/scripts/network/lib.sh"

TOPOLOGY="${1:-multi}"
case "$TOPOLOGY" in
  single|multi) ;;
  *)
    echo "usage: scripts/fork/run_matrix.sh <single|multi>" >&2
    exit 1
    ;;
esac

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
FORK_CASES="${FORK_CASES:-poa,upgrade:shanghaiTime,upgrade:cancunTime,upgrade:posaTime,upgrade:fixHeaderTime,posa}"
FORK_DELAY_SECONDS="${FORK_DELAY_SECONDS:-120}"
FORK_UPGRADE_STARTUP_BUFFER_SINGLE="${FORK_UPGRADE_STARTUP_BUFFER_SINGLE:-5}"
FORK_UPGRADE_STARTUP_BUFFER_MULTI="${FORK_UPGRADE_STARTUP_BUFFER_MULTI:-30}"
FORK_TEST_TIMEOUT="${FORK_TEST_TIMEOUT:-20m}"
TEST_CONFIG_FILE="$PROJECT_ROOT/data/test_config.yaml"

run_case() {
  local mode="$1"
  local target="$2"
  local label="$3"
  local case_delay="$FORK_DELAY_SECONDS"
  local rc=0

  if [[ "$mode" == "upgrade" ]]; then
    local startup_buffer="$FORK_UPGRADE_STARTUP_BUFFER_MULTI"
    if [[ "$TOPOLOGY" == "single" ]]; then
      startup_buffer="$FORK_UPGRADE_STARTUP_BUFFER_SINGLE"
    fi
    case_delay=$((FORK_DELAY_SECONDS + startup_buffer))
  fi

  local init_env=(
    "TEST_ENV_CONFIG=$CONFIG_FILE"
    "GENESIS_MODE=$mode"
    "FORK_TARGET=$target"
    "FORK_DELAY_SECONDS=$case_delay"
  )

  if [[ "$TOPOLOGY" == "single" ]]; then
    init_env+=(
      "TEST_NETWORK_VALIDATOR_COUNT=1"
      "TEST_NETWORK_NODE_COUNT=1"
    )
  fi

  echo "=== [fork/$TOPOLOGY] case=$label mode=$mode target=${target:-<none>} ==="

  env "${init_env[@]}" make -C "$PROJECT_ROOT" clean
  env "${init_env[@]}" make -C "$PROJECT_ROOT" init

  if [[ "$TOPOLOGY" == "single" ]]; then
    if env "${init_env[@]}" "$PROJECT_ROOT/scripts/network/native_single.sh" up "$CONFIG_FILE"; then
      :
    else
      rc=$?
    fi
    if [[ $rc -eq 0 ]]; then
      if env "${init_env[@]}" "$PROJECT_ROOT/scripts/network/native_single.sh" ready "$CONFIG_FILE"; then
        :
      else
        rc=$?
      fi
    fi
  else
    if env "${init_env[@]}" make -C "$PROJECT_ROOT" run; then
      :
    else
      rc=$?
    fi
    if [[ $rc -eq 0 ]]; then
      if env "${init_env[@]}" make -C "$PROJECT_ROOT" ready; then
        :
      else
        rc=$?
      fi
    fi
  fi

  if [[ $rc -eq 0 ]]; then
    if (cd "$PROJECT_ROOT" && env "${init_env[@]}" go test ./tests/fork -v -run "^TestF_ForkLiveness$" -count=1 -parallel=1 -p 1 -timeout "$FORK_TEST_TIMEOUT" -config "$TEST_CONFIG_FILE"); then
      :
    else
      rc=$?
    fi
  fi

  if [[ "$TOPOLOGY" == "single" ]]; then
    env "${init_env[@]}" "$PROJECT_ROOT/scripts/network/native_single.sh" down "$CONFIG_FILE" || true
  else
    env "${init_env[@]}" make -C "$PROJECT_ROOT" stop || true
  fi
  env "${init_env[@]}" make -C "$PROJECT_ROOT" clean || true

  return "$rc"
}

IFS=',' read -r -a CASE_ARRAY <<< "$FORK_CASES"
for raw_case in "${CASE_ARRAY[@]}"; do
  case_item="$(echo "$raw_case" | tr -d '[:space:]')"
  [[ -n "$case_item" ]] || continue

  mode="$case_item"
  target=""
  if [[ "$case_item" == upgrade:* ]]; then
    mode="upgrade"
    target="${case_item#upgrade:}"
  fi
  if [[ "$mode" == "upgrade" && -z "$target" ]]; then
    echo "invalid fork case '$case_item': upgrade requires target" >&2
    exit 1
  fi
  if ! run_case "$mode" "$target" "$case_item"; then
    exit 1
  fi
done
