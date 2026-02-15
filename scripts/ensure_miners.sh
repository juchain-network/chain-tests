#!/bin/bash
set -euo pipefail

NODES=("$@")
if [ "${#NODES[@]}" -eq 0 ]; then
  NODES=(node0 node1 node2)
fi

check_progress() {
  local node="$1"
  local container="juchain-${node}"
  echo "⛏️  Checking block progress on ${container}..."
  for i in $(seq 1 60); do
    if docker exec "${container}" juchain attach --datadir /data --exec "eth.blockNumber" >/dev/null 2>&1; then
      # Wait for block number to advance (node sees chain progress even if not sealing).
      local prev cur
      for j in $(seq 1 30); do
        prev=$(docker exec "${container}" juchain attach --datadir /data --exec "eth.blockNumber" 2>/dev/null || true)
        sleep 2
        cur=$(docker exec "${container}" juchain attach --datadir /data --exec "eth.blockNumber" 2>/dev/null || true)
        if [[ "$prev" =~ ^0x[0-9a-fA-F]+$ ]] && [[ "$cur" =~ ^0x[0-9a-fA-F]+$ ]] && [ "$cur" != "$prev" ]; then
          return 0
        fi
      done
      echo "⚠️  ${container} did not observe block progress yet"
      return 0
    fi
    sleep 1
  done
  return 0
}

pids=()
for node in "${NODES[@]}"; do
  check_progress "$node" &
  pids+=($!)
done

for pid in "${pids[@]}"; do
  wait "$pid" || true
done
