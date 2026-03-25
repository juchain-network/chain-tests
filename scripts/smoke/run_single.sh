#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$ROOT_DIR/scripts/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
CFG_RUNTIME_IMPL_MODE="$(cfg_get "$CONFIG_FILE" "runtime.impl_mode" "single")"
CFG_RUNTIME_IMPL="$(cfg_get "$CONFIG_FILE" "runtime.impl" "geth")"
CFG_NODE0_IMPL="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node0" "")"
CFG_AUTH_MODE="$(cfg_get "$CONFIG_FILE" "validator_auth.mode" "auto")"
CFG_GENESIS_MODE="$(cfg_get "$CONFIG_FILE" "network.genesis_mode" "")"
CFG_OBSERVE_SECONDS="$(cfg_get "$CONFIG_FILE" "tests.smoke.observe_seconds" "300")"

CFG_SINGLE_IMPL="$CFG_RUNTIME_IMPL"
if [[ "$CFG_RUNTIME_IMPL_MODE" == "mixed" && -n "$CFG_NODE0_IMPL" ]]; then
  CFG_SINGLE_IMPL="$CFG_NODE0_IMPL"
fi

SMOKE_IMPL="${SMOKE_SINGLE_IMPL:-$CFG_SINGLE_IMPL}"
SMOKE_AUTH_MODE="${SMOKE_SINGLE_AUTH_MODE:-$CFG_AUTH_MODE}"
if [[ -n "${SMOKE_SINGLE_GENESIS_MODE:-}" ]]; then
  SMOKE_GENESIS_MODE="$SMOKE_SINGLE_GENESIS_MODE"
elif [[ -n "$CFG_GENESIS_MODE" ]]; then
  SMOKE_GENESIS_MODE="$CFG_GENESIS_MODE"
elif [[ "$SMOKE_IMPL" == "reth" ]]; then
  SMOKE_GENESIS_MODE="posa"
else
  SMOKE_GENESIS_MODE="poa"
fi
SMOKE_SINGLE_FORK_TARGET="${SMOKE_SINGLE_FORK_TARGET:-${FORK_TARGET:-}}"
SMOKE_OBSERVE_SECONDS="${SMOKE_SINGLE_OBSERVE_SECONDS:-$CFG_OBSERVE_SECONDS}"
SMOKE_TIMEOUT="${SMOKE_SINGLE_TEST_TIMEOUT:-12m}"
SMOKE_GOCACHE="${GOCACHE:-/tmp/gocache}"

case "$SMOKE_IMPL" in
  geth|reth) ;;
  *) die "SMOKE_SINGLE_IMPL must be geth|reth, got: $SMOKE_IMPL" ;;
esac

case "$SMOKE_AUTH_MODE" in
  auto|private_key|keystore) ;;
  *) die "SMOKE_SINGLE_AUTH_MODE must be auto|private_key|keystore, got: $SMOKE_AUTH_MODE" ;;
esac

if [[ "$SMOKE_GENESIS_MODE" == "smoke" && -z "$SMOKE_SINGLE_FORK_TARGET" ]]; then
  die "SMOKE_SINGLE_FORK_TARGET is required when SMOKE_SINGLE_GENESIS_MODE=smoke"
fi

TMP_CFG="$(mktemp "${TMPDIR:-/tmp}/chain-tests-smoke-single.XXXXXX")"
cp "$CONFIG_FILE" "$TMP_CFG"
trap 'TEST_ENV_CONFIG="$TMP_CFG" "$ROOT_DIR/scripts/network/native_single.sh" down "$TMP_CFG" >/dev/null 2>&1 || true; TEST_ENV_CONFIG="$TMP_CFG" make -C "$ROOT_DIR" clean >/dev/null 2>&1 || true; rm -f "$TMP_CFG" "${TMP_CFG}.next"' EXIT

awk -v impl="$SMOKE_IMPL" -v auth_mode="$SMOKE_AUTH_MODE" '
  /^runtime:/ { in_runtime=1; in_validator_auth=0; print; next }
  /^validator_auth:/ { in_runtime=0; in_validator_auth=1; print; next }
  /^[^[:space:]]/ && $0 !~ /^runtime:/ && $0 !~ /^validator_auth:/ { in_runtime=0; in_validator_auth=0 }
  in_runtime && /^  backend:/ { print "  backend: native"; next }
  in_runtime && /^  impl_mode:/ { print "  impl_mode: single"; next }
  in_runtime && /^  impl:/ { print "  impl: " impl; next }
  in_validator_auth && /^  mode:/ { print "  mode: " auth_mode; next }
  { print }
' "$TMP_CFG" > "${TMP_CFG}.next"
mv "${TMP_CFG}.next" "$TMP_CFG"

echo "[smoke-single] config=$TMP_CFG impl=$SMOKE_IMPL auth_mode=$SMOKE_AUTH_MODE genesis_mode=$SMOKE_GENESIS_MODE fork_target=${SMOKE_SINGLE_FORK_TARGET:-<none>}"

TEST_ENV_CONFIG="$TMP_CFG" make -C "$ROOT_DIR" clean
TEST_ENV_CONFIG="$TMP_CFG" GENESIS_MODE="$SMOKE_GENESIS_MODE" FORK_TARGET="$SMOKE_SINGLE_FORK_TARGET" TEST_NETWORK_NODE_COUNT=1 TEST_NETWORK_VALIDATOR_COUNT=1 make -C "$ROOT_DIR" init
TEST_ENV_CONFIG="$TMP_CFG" "$ROOT_DIR/scripts/network/native_single.sh" up "$TMP_CFG"
TEST_ENV_CONFIG="$TMP_CFG" "$ROOT_DIR/scripts/network/native_single.sh" ready "$TMP_CFG"

CFG_ABS="$ROOT_DIR/data/test_config.yaml"
(
  cd "$ROOT_DIR"
  GOCACHE="$SMOKE_GOCACHE" \
  SMOKE_SINGLE_OBSERVE_SECONDS="$SMOKE_OBSERVE_SECONDS" \
  go test ./tests/smoke -v -run '^TestS_SmokeSingleNodeLiveness$' -count=1 -timeout "$SMOKE_TIMEOUT" -config "$CFG_ABS"
)

echo "[smoke-single] 🟢 PASS impl=$SMOKE_IMPL genesis_mode=$SMOKE_GENESIS_MODE"
