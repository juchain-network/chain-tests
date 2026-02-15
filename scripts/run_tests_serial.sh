#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_PATH="${ROOT_DIR}/data/test_config.yaml"

export LC_ALL=C
export GOCACHE="${ROOT_DIR}/.gocache"
mkdir -p "${GOCACHE}"

cd "${ROOT_DIR}"

current_test=""

cleanup() {
  status=$?
  trap - EXIT
  set +e
  if [ "$status" -ne 0 ]; then
    if [ -n "${current_test}" ]; then
      echo ""
      echo "❌ Aborting on error (test: ${current_test}). Stopping network..."
    else
      echo ""
      echo "❌ Aborting on error. Stopping network..."
    fi
  fi
  if ! make stop; then
    echo "⚠️ Failed to stop network during cleanup"
    if [ "$status" -eq 0 ]; then
      status=1
    fi
  fi
  exit "$status"
}
trap cleanup EXIT

TESTS=$(grep -h "^func Test" "${ROOT_DIR}/tests"/*.go | sed -E 's/^func (Test[^(]+).*/\1/' | grep -v '^TestMain$' | sort -u)

if [ -z "${TESTS}" ]; then
  echo "❌ No tests found in ${ROOT_DIR}/tests"
  exit 1
fi

for T in ${TESTS}; do
  current_test="${T}"
  echo "=============================="
  echo "🧪 Running ${T}"
  echo "=============================="
  make stop
  sleep 5
  make clean
  sleep 5
  make init run ready
  go test ./tests/... -v -run "^${T}$" -count=1 -parallel=1 -p 1 -timeout 20m -config "${CONFIG_PATH}"
  make stop
  sleep 3
done
