#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$ROOT_DIR/scripts/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
PM2_NAMESPACE="$(cfg_get "$CONFIG_FILE" "native.pm2_namespace" "ju-chain")"
pm2 restart "${PM2_NAMESPACE}-validator2" >/dev/null
echo "native restart: ${PM2_NAMESPACE}-validator2"
