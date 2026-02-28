#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$ROOT_DIR/scripts/network/lib.sh"

PROFILE="${1:-}"
[[ -n "$PROFILE" ]] || { echo "usage: scripts/ci/run_profile.sh <pr|nightly|weekly_soak>" >&2; exit 1; }

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
  local run_smoke run_blacklist
  run_smoke="$(cfg_get "$CONFIG_FILE" "ci.pr_gate.run_smoke" "true")"
  run_blacklist="$(cfg_get "$CONFIG_FILE" "ci.pr_gate.run_blacklist" "true")"

  if is_true "$run_smoke"; then
    make -C "$ROOT_DIR" test-smoke
  fi
  GOCACHE="${GOCACHE:-}" make -C "$ROOT_DIR" ci-groups GROUPS="$groups"
  if is_true "$run_blacklist"; then
    make -C "$ROOT_DIR" test-blacklist
  fi
}

run_nightly() {
  local groups
  groups="$(cfg_get "$CONFIG_FILE" "ci.nightly.groups" "${CI_NIGHTLY_GROUPS:-config,governance,staking,delegation,punish,rewards,epoch}")"
  local run_smoke run_fork run_posa run_blacklist
  run_smoke="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_smoke" "true")"
  run_fork="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_fork_all" "true")"
  run_posa="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_posa" "true")"
  run_blacklist="$(cfg_get "$CONFIG_FILE" "ci.nightly.run_blacklist" "true")"

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
  if is_true "$run_blacklist"; then
    make -C "$ROOT_DIR" test-blacklist
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

case "$PROFILE" in
  pr) run_pr ;;
  nightly) run_nightly ;;
  weekly_soak) run_weekly_soak ;;
  *)
    echo "unsupported profile: $PROFILE" >&2
    exit 1
    ;;
esac
