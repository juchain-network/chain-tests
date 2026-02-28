#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$ROOT_DIR/scripts/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
BACKEND="${RUNTIME_BACKEND:-$(cfg_get "$CONFIG_FILE" "runtime.backend" "native")}"

if [[ "$BACKEND" == "native" ]]; then
  PM2_NAMESPACE="$(cfg_get "$CONFIG_FILE" "native.pm2_namespace" "ju-chain")"
  pm2 restart "${PM2_NAMESPACE}-validator2" >/dev/null
  echo "native restart: ${PM2_NAMESPACE}-validator2"
  exit 0
fi

if [[ "$BACKEND" == "docker" ]]; then
  COMPOSE_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "docker.runtime_compose_file" "./data/docker-compose.runtime.yml")")"
  if [[ ! -f "$COMPOSE_FILE" ]]; then
    COMPOSE_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "docker.compose_file" "./docker/docker-compose.yml")")"
  fi
  PROJECT_NAME="$(cfg_get "$CONFIG_FILE" "docker.project_name" "juchain-it")"
  docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" restart node1 >/dev/null
  echo "docker restart: node1"
  exit 0
fi

echo "unsupported backend: $BACKEND" >&2
exit 1
