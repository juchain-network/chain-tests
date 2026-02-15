#!/bin/bash
set -e

# Wrapper script for Makefile-based workflow
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TEST_INT_DIR="$SCRIPT_DIR/.."

echo "⚠️  Note: This script is a wrapper around 'make'. Please consider using 'make' directly in 'test-integration/'."

pushd "$TEST_INT_DIR" > /dev/null

if [[ "$1" == "--build" ]]; then
    make ci
elif [[ "$1" == "--keep" ]]; then
    make clean image init run ready test
    echo "ℹ️  Environment kept running."
else
    make ci
fi

popd > /dev/null