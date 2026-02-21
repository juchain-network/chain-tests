#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Reuse config parsing helpers (cfg_get/resolve_config_file).
# shellcheck source=/dev/null
source "$ROOT_DIR/scripts/network/lib.sh"

scope="${1:-groups}"   # groups | specials
name="${2:-}"
group_fallback="${3:-}"
scope="$(printf '%s' "$scope" | tr '[:upper:]' '[:lower:]')"

case "$scope" in
  groups|specials) ;;
  *)
    die "invalid scope '$scope' (expected groups|specials)"
    ;;
esac

if [[ -n "${EPOCH:-}" ]]; then
  echo "${EPOCH}"
  exit 0
fi

config_file="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
value=""

if [[ -n "$name" ]]; then
  value="$(cfg_get "$config_file" "tests.epoch_overrides.${scope}.${name}" "")"
fi

if [[ -z "$value" && "$scope" == "specials" && -n "$group_fallback" ]]; then
  value="$(cfg_get "$config_file" "tests.epoch_overrides.groups.${group_fallback}" "")"
fi

if [[ -z "$value" && ( "$scope" == "groups" || "$scope" == "specials" ) ]]; then
  value="$(cfg_get "$config_file" "tests.epoch_overrides.groups.default" "")"
fi

if [[ -z "$value" ]]; then
  profile="$(cfg_get "$config_file" "tests.profile" "fast")"
  value="$(cfg_get "$config_file" "tests.profiles.${profile}.epoch" "")"
fi

if [[ -z "$value" ]]; then
  value="$(cfg_get "$config_file" "network.epoch" "")"
fi

if [[ -z "$value" ]]; then
  exit 0
fi

if [[ ! "$value" =~ ^[0-9]+$ ]] || [[ "$value" -le 0 ]]; then
  die "invalid epoch value '$value' (scope=$scope key=$name)"
fi

echo "$value"
