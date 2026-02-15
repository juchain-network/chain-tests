#!/bin/bash
set -e

# Initialize genesis if not already done
if [ ! -d "/data/geth/chaindata" ]; then
    echo "Initializing genesis..."
    juchain --datadir /data init /genesis.json 
fi

SYNC_NODE="${SYNC_NODE:-false}"

if [ "$SYNC_NODE" = "true" ]; then
    echo "Starting SYNC NODE (Non-Validator)..."
    RPC_URL="http://127.0.0.1:8545"
    
    juchain \
        --config /data/config.toml \
        --networkid 666666 \
        --nodekey /data/nodekey \
        --gcmode "archive" \
        --cache 4096 \
        --http \
        --http.api "eth,net,web3,debug,admin,personal,miner,txpool" \
        --http.corsdomain "*" \
        --http.vhosts "*" \
        --http.addr "0.0.0.0" \
        --ws \
        --ws.api "eth,net,web3,debug,admin,personal,miner,txpool" \
        --ws.origins "*" \
        --ws.addr "0.0.0.0" \
        --nat extip:$(hostname -i) &
        
    NODE_PID=$!
    
    # Wait for RPC and Peers logic is shared below, but miner start is skipped
else
    # VALIDATOR NODE LOGIC

    # Import validator key if present
    echo "123456" > /tmp/password
    if [ -f "/data/validator.key" ] && [ ! "$(ls -A /data/keystore 2>/dev/null)" ]; then
        echo "Importing validator key..."
        echo "123456" > /tmp/password
        juchain account import --datadir /data --password /tmp/password /data/validator.key
    fi

    # Get address
    VAL_ADDR=${VAL_ADDR:-$(juchain account list --datadir /data | head -n 1 | cut -d '{' -f 2 | cut -d '}' -f 1)}

    echo "Starting VALIDATOR node: $VAL_ADDR"

    MIN_PEERS="${MIN_PEERS:-3}"
    MINER_START_DELAY="${MINER_START_DELAY:-0}"
    RPC_URL="http://127.0.0.1:8545"

    juchain \
        --config /data/config.toml \
        --networkid 666666 \
        --nodekey /data/nodekey \
        --cache 1024 \
        --mine \
        --http \
        --http.api "eth,net,web3,debug,admin,personal,miner,txpool" \
        --http.corsdomain "*" \
        --http.vhosts "*" \
        --http.addr "0.0.0.0" \
        --ws \
        --ws.api "eth,net,web3,debug,admin,personal,miner,txpool" \
        --ws.origins "*" \
        --ws.addr "0.0.0.0" \
        --miner.etherbase "$VAL_ADDR" \
        --miner.gasprice 0 \
        --unlock "$VAL_ADDR" \
        --password /tmp/password \
        --allow-insecure-unlock \
        --nat extip:$(hostname -i) &

    NODE_PID=$!
fi

# Shared Wait Logic

wait_for_rpc() {
    for i in $(seq 1 120); do
        RESP=$(juchain attach --datadir /data --exec "eth.blockNumber" 2>/dev/null || true)
        if [[ "$RESP" =~ ^0x[0-9a-fA-F]+$ ]]; then
            return 0
        fi
        if ! kill -0 "$NODE_PID" >/dev/null 2>&1; then
            return 1
        fi
        sleep 1
    done
    return 1
}

wait_for_peers() {
    MIN_PEERS="${MIN_PEERS:-3}"
    for i in $(seq 1 300); do
        RESP=$(juchain attach --datadir /data --exec "net.peerCount" 2>/dev/null || true)
        if [[ "$RESP" =~ ^0x[0-9a-fA-F]+$ ]]; then
            DEC=${RESP#0x}
            CUR=$((16#$DEC))
            if [ "$CUR" -ge "$MIN_PEERS" ]; then
                return 0
            fi
        fi
        if ! kill -0 "$NODE_PID" >/dev/null 2>&1; then
            return 1
        fi
        sleep 1
    done
    echo "⚠️ Peer count not reached ($MIN_PEERS), continuing anyway..."
    return 0
}

if ! wait_for_rpc; then
    echo "❌ RPC not ready, exiting"
    wait "$NODE_PID"
    exit 1
fi

if ! wait_for_peers; then
    echo "❌ Peers not ready (min=$MIN_PEERS), exiting"
    wait "$NODE_PID"
    exit 1
fi

if [ "$SYNC_NODE" != "true" ]; then
    if [ "$MINER_START_DELAY" -gt 0 ]; then
        sleep "$MINER_START_DELAY"
    fi
fi

echo "Node is ready."
wait "$NODE_PID"
