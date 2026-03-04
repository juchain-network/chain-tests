#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$ROOT_DIR/scripts/network/lib.sh"

PROFILE="${1:-}"
[[ -n "$PROFILE" ]] || { echo "usage: scripts/ci/run_profile.sh <pr|nightly|weekly_soak|release_gate>" >&2; exit 1; }

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"

is_true() {
  case "$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

run_pr() {
  local groups
  groups="$(cfg_get "$CONFIG_FILE" "ci.pr_gate.groups" "${CI_PR_GROUPS:-config,governance,staking,punish,epoch}")"
  local run_smoke
  run_smoke="$(cfg_get "$CONFIG_FILE" "ci.pr_gate.run_smoke" "true")"

  if is_true "$run_smoke"; then
    make -C "$ROOT_DIR" test-smoke
  fi
  GOCACHE="${GOCACHE:-}" make -C "$ROOT_DIR" ci-groups GROUPS="$groups"
}

run_nightly() {
  local groups
  groups="$(cfg_get "$CONFIG_FILE" "ci.nightly.groups" "${CI_NIGHTLY_GROUPS:-config,governance,staking,delegation,punish,rewards,epoch}")"
  local run_smoke run_fork run_posa run_reth_keystore
  run_smoke="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_smoke" "true")"
  run_fork="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_fork_all" "true")"
  run_posa="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_posa" "true")"
  run_reth_keystore="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_reth_keystore_smoke" "false")"

  if is_true "$run_smoke"; then
    make -C "$ROOT_DIR" test-smoke
  fi
  GOCACHE="${GOCACHE:-}" make -C "$ROOT_DIR" ci-groups GROUPS="$groups"
  if is_true "$run_fork"; then
    make -C "$ROOT_DIR" test-fork-all
  fi
  if is_true "$run_posa"; then
    make -C "$ROOT_DIR" test-posa-multi
  fi
  if is_true "$run_reth_keystore"; then
    run_reth_keystore_smoke
  fi
}

run_weekly_soak() {
  local enabled
  enabled="$(cfg_get "$CONFIG_FILE" "ci.weekly_soak.enabled" "true")"
  if ! is_true "$enabled"; then
    echo "weekly_soak profile is disabled in config"
    exit 0
  fi
  make -C "$ROOT_DIR" test-soak-24h
}

run_release_gate() {
  local run_smoke run_fork run_posa
  run_smoke="$(cfg_get "$CONFIG_FILE" "ci.release_gate.run_smoke" "true")"
  run_fork="$(cfg_get "$CONFIG_FILE" "ci.release_gate.run_fork_all" "true")"
  run_posa="$(cfg_get "$CONFIG_FILE" "ci.release_gate.run_posa" "true")"

  if is_true "$run_smoke"; then
    make -C "$ROOT_DIR" test-smoke
  fi
  if is_true "$run_fork"; then
    make -C "$ROOT_DIR" test-fork-all
  fi
  if is_true "$run_posa"; then
    make -C "$ROOT_DIR" test-posa-multi
  fi
}

run_reth_keystore_smoke() {
  local tmp_cfg
  tmp_cfg="$(mktemp "${TMPDIR:-/tmp}/chain-tests-reth-XXXX.yaml")"
  trap 'rm -f "$tmp_cfg" "${tmp_cfg}.next"' RETURN
  cp "$CONFIG_FILE" "$tmp_cfg"
  awk '
    /^runtime:/ { in_runtime=1; in_validator_auth=0; print; next }
    /^validator_auth:/ { in_runtime=0; in_validator_auth=1; print; next }
    /^[^[:space:]]/ && $0 !~ /^runtime:/ && $0 !~ /^validator_auth:/ { in_runtime=0; in_validator_auth=0 }
    in_runtime && /^  backend:/ { print "  backend: native"; next }
    in_runtime && /^  impl_mode:/ { print "  impl_mode: single"; next }
    in_runtime && /^  impl:/ { print "  impl: reth"; next }
    in_validator_auth && /^  mode:/ { print "  mode: keystore"; next }
    { print }
  ' "$tmp_cfg" > "${tmp_cfg}.next"
  mv "${tmp_cfg}.next" "$tmp_cfg"

  TEST_ENV_CONFIG="$CONFIG_FILE" RUNTIME_BACKEND=docker "$ROOT_DIR/scripts/network/dispatch.sh" down || true
  TEST_ENV_CONFIG="$tmp_cfg" make -C "$ROOT_DIR" reset
  TEST_ENV_CONFIG="$tmp_cfg" make -C "$ROOT_DIR" test-smoke
  TEST_ENV_CONFIG="$tmp_cfg" make -C "$ROOT_DIR" stop || true
  trap - RETURN
  rm -f "$tmp_cfg"
}

case "$PROFILE" in
  pr) run_pr ;;
  nightly) run_nightly ;;
  weekly_soak) run_weekly_soak ;;
  release_gate) run_release_gate ;;
  *)
    echo "unsupported profile: $PROFILE" >&2
    exit 1
    ;;
esac
