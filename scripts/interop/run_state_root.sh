#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG_PATH="${CONFIG_PATH:-$ROOT_DIR/data/test_config.yaml}"
TIMEOUT="${INTEROP_TIMEOUT:-15m}"

cd "$ROOT_DIR"
go test ./tests/interop -v -run '^TestI_StateRootCheckpoint$' -count=1 -timeout "$TIMEOUT" -config "$CONFIG_PATH"
