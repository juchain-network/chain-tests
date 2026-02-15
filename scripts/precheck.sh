#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STAMP_FILE="$ROOT_DIR/.chain-tests-precheck.stamp"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/chain-tests-precheck.XXXXXX")"
GOCACHE_DIR="${GOCACHE:-/tmp/go-build}"

trap 'rm -rf "$TMP_DIR"' EXIT

if command -v shasum >/dev/null 2>&1; then
  HASH_FILE_CMD=(shasum)
elif command -v sha256sum >/dev/null 2>&1; then
  HASH_FILE_CMD=(sha256sum)
else
  echo "[precheck] ERROR: shasum/sha256sum not found" >&2
  exit 1
fi

hash_path() {
  "${HASH_FILE_CMD[@]}" "$1" | awk '{print $1}'
}

calc_fingerprint() {
  local list_file="$TMP_DIR/sources.txt"
  local fp_file="$TMP_DIR/fingerprint.txt"
  : > "$list_file"
  : > "$fp_file"

  {
    [[ -f "$ROOT_DIR/go.mod" ]] && echo "$ROOT_DIR/go.mod"
    [[ -f "$ROOT_DIR/go.sum" ]] && echo "$ROOT_DIR/go.sum"
    [[ -f "$ROOT_DIR/ci.go" ]] && echo "$ROOT_DIR/ci.go"
    [[ -d "$ROOT_DIR/internal" ]] && find "$ROOT_DIR/internal" -type f -name '*.go'
    [[ -d "$ROOT_DIR/contracts" ]] && find "$ROOT_DIR/contracts" -type f -name '*.go'
    [[ -d "$ROOT_DIR/tests" ]] && find "$ROOT_DIR/tests" -type f -name '*.go'
  } | sort > "$list_file"

  local src
  while IFS= read -r src; do
    [[ -f "$src" ]] || continue
    printf '%s %s\n' "$(hash_path "$src")" "$src" >> "$fp_file"
  done < "$list_file"

  hash_path "$fp_file"
}

fingerprint="$(calc_fingerprint)"
if [[ -f "$STAMP_FILE" ]] && [[ "$(cat "$STAMP_FILE")" == "$fingerprint" ]]; then
  echo "[precheck] source fingerprint unchanged, skip compile checks"
  exit 0
fi

echo "[precheck] compiling test packages..."
while IFS= read -r pkg; do
  [[ -n "$pkg" ]] || continue
  out_file="$TMP_DIR/$(echo "$pkg" | tr '/.' '__').test"
  (cd "$ROOT_DIR" && GOCACHE="$GOCACHE_DIR" go test -c -o "$out_file" "$pkg")
done < <(cd "$ROOT_DIR" && go list ./tests/...)

echo "[precheck] compiling ci tool and core packages..."
(cd "$ROOT_DIR" && GOCACHE="$GOCACHE_DIR" go build -o "$TMP_DIR/ci-tool" ./ci.go)
(cd "$ROOT_DIR" && GOCACHE="$GOCACHE_DIR" go build ./internal/...)
(cd "$ROOT_DIR" && GOCACHE="$GOCACHE_DIR" go build ./contracts/...)

echo "$fingerprint" > "$STAMP_FILE"
echo "[precheck] OK"
