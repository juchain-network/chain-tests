#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
CFG_CHAIN_ROOT="$(cfg_get "$CONFIG_FILE" "paths.chain_root" "../chain")"
CFG_CHAIN_DOCKER_BIN="$(cfg_get "$CONFIG_FILE" "binaries.geth_docker" "$(cfg_get "$CONFIG_FILE" "paths.chain_docker_binary" "")")"

CHAIN_ROOT="$(to_abs_path "${CHAIN_ROOT:-$CFG_CHAIN_ROOT}")"
CHAIN_DOCKER_BINARY="$(to_abs_path "${CHAIN_DOCKER_BINARY:-$CFG_CHAIN_DOCKER_BIN}")"
DOCKER_BIN="$SCRIPT_DIR/../docker/juchain"

TARGET_GOARCH="${TARGET_GOARCH:-}"
if [ -z "$TARGET_GOARCH" ]; then
    DOCKER_ARCH="$(docker info --format '{{.Architecture}}' 2>/dev/null || true)"
    case "$DOCKER_ARCH" in
        amd64|x86_64) TARGET_GOARCH="amd64" ;;
        arm64|aarch64) TARGET_GOARCH="arm64" ;;
        arm|armv7l) TARGET_GOARCH="arm" ;;
        *) TARGET_GOARCH="$(go env GOARCH)" ;;
    esac
fi

binary_matches_target() {
    local info="$1"
    local arch="$2"
    case "$arch" in
        amd64) echo "$info" | grep -Eiq 'x86-64|amd64' ;;
        arm64) echo "$info" | grep -Eiq 'aarch64|arm64' ;;
        arm) echo "$info" | grep -Eiq ' ARM ' ;;
        *) echo "$info" | grep -Eiq "$arch" ;;
    esac
}

is_compatible_elf() {
    local bin="$1"
    [ -x "$bin" ] || return 1
    command -v file >/dev/null 2>&1 || return 1
    local info
    info="$(file "$bin" 2>/dev/null || true)"
    echo "$info" | grep -q "ELF" || return 1
    binary_matches_target "$info" "$TARGET_GOARCH"
}

echo "=== Preparing docker runtime binary (target linux/${TARGET_GOARCH}) ==="

if is_compatible_elf "$DOCKER_BIN"; then
    echo "✅ Reusing existing docker binary: $DOCKER_BIN"
    exit 0
fi

CANDIDATES=()
if [ -n "$CHAIN_DOCKER_BINARY" ]; then
    CANDIDATES+=("$CHAIN_DOCKER_BINARY")
fi

if [ -d "$CHAIN_ROOT" ]; then
    CANDIDATES+=(
        "$CHAIN_ROOT/build/bin/geth-linux-${TARGET_GOARCH}"
        "$CHAIN_ROOT/build/bin/geth"
    )
fi

for bin in "${CANDIDATES[@]}"; do
    if is_compatible_elf "$bin"; then
        echo "✅ Using prebuilt docker binary: $bin"
        cp "$bin" "$DOCKER_BIN"
        chmod +x "$DOCKER_BIN"
        exit 0
    fi
done

echo "❌ No compatible linux/${TARGET_GOARCH} juchain binary found."
echo "Expected one of:"
for bin in "${CANDIDATES[@]}"; do
    echo "  - $bin"
done
echo ""
echo "Provide a prebuilt binary via:"
echo "  1) paths.chain_docker_binary in ${CONFIG_FILE}"
echo "  2) CHAIN_DOCKER_BINARY env"
echo ""
echo "Note: this repo does not compile chain source; it only consumes compiled binaries."
exit 1
