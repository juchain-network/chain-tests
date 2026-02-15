#!/bin/bash

# Default to localhost:18545 if not set (Sync Node)
RPC_URL="${1:-http://localhost:18545}"
RETRIES="${RETRIES:-300}"
INCREMENTS_REQUIRED="${INCREMENTS_REQUIRED:-3}"
MIN_BLOCK="${MIN_BLOCK:-}"
INCREMENTS=0
PREV_BLOCK=""

echo "⏳ Waiting for node at $RPC_URL to be ready..."

while [ $RETRIES -gt 0 ]; do
    RESP=$(curl -s -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        "$RPC_URL" || true)
    HEX=$(echo "$RESP" | sed -n 's/.*"result":"\(0x[0-9a-fA-F]*\)".*/\1/p')
    if [ -n "$HEX" ]; then
        DEC=${HEX#0x}
        if [ -n "$DEC" ]; then
            CUR=$((16#$DEC))
            if [ "$CUR" != "$PREV_BLOCK" ]; then
                echo -n "[$CUR]"
            fi
            if [ -n "$MIN_BLOCK" ] && [ "$CUR" -ge "$MIN_BLOCK" ]; then
                echo ""
                echo "✅ Node is up (block >= $MIN_BLOCK)"
                exit 0
            fi
            if [ -n "$PREV_BLOCK" ] && [ "$CUR" -gt "$PREV_BLOCK" ]; then
                INCREMENTS=$((INCREMENTS+1))
            fi
            PREV_BLOCK=$CUR
            if [ "$INCREMENTS_REQUIRED" -le 0 ]; then
                echo ""
                echo "✅ Node is up (RPC responding)"
                exit 0
            fi
            if [ "$INCREMENTS" -ge "$INCREMENTS_REQUIRED" ]; then
                echo ""
                echo "✅ Node is up and producing blocks!"
                exit 0
            fi
        fi
    fi
    sleep 1
    RETRIES=$((RETRIES-1))
    echo -n "."
done

echo ""
echo "❌ Timeout waiting for node to produce blocks"
exit 1
