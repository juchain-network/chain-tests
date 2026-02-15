#!/bin/bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
TEMPLATE_GENESIS="$(to_abs_path "./templates/genesis.tpl.json")"
PRIMARY_RPC="$(cfg_get "$CONFIG_FILE" "network.external_rpc" "http://localhost:18545")"

V1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_http" "18545")"
V2_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_http" "18547")"
V3_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_http" "18549")"

CFG_CONTRACT_ROOT="$(cfg_get "$CONFIG_FILE" "paths.chain_contract_root" "../chain-contract")"
CFG_CONTRACT_OUT="$(cfg_get "$CONFIG_FILE" "paths.chain_contract_out" "")"

CHAIN_CONTRACT_ROOT="$(to_abs_path "${CHAIN_CONTRACT_ROOT:-$CFG_CONTRACT_ROOT}")"
if [ -n "${CHAIN_CONTRACT_OUT:-}" ]; then
    CONTRACT_OUT_DIR="$(to_abs_path "$CHAIN_CONTRACT_OUT")"
elif [ -n "$CFG_CONTRACT_OUT" ]; then
    CONTRACT_OUT_DIR="$(to_abs_path "$CFG_CONTRACT_OUT")"
else
    CONTRACT_OUT_DIR="$CHAIN_CONTRACT_ROOT/out"
fi

if [ ! -d "$CONTRACT_OUT_DIR" ]; then
    echo "❌ Compiled contract output not found: $CONTRACT_OUT_DIR"
    echo "Provide CHAIN_CONTRACT_OUT env or configure paths.chain_contract_out / paths.chain_contract_root."
    echo "Only compiled artifacts are required (no source build in this repo)."
    exit 1
fi

# Clean up data directory (handle root-owned files from Docker)
if [ -d "$DATA_DIR" ]; then
    rm -rf "$DATA_DIR" 2>/dev/null || true
    if [ -d "$DATA_DIR" ]; then
        docker run --rm -v "$ROOT_DIR":/work alpine sh -c "rm -rf /work/data" || true
    fi
fi
mkdir -p "$DATA_DIR"

echo "=== Generating Network Configuration ==="

# 1. Generate Keys helper (using a temporary Go program)
cat > "$DATA_DIR/genkeys.go" <<EOF
package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	var key *ecdsa.PrivateKey
	if len(os.Args) > 1 && os.Args[1] != "" {
		seed := os.Args[1]
		sum := sha256.Sum256([]byte(seed))
		for {
			k, err := crypto.ToECDSA(sum[:])
			if err == nil {
				key = k
				break
			}
			sum = sha256.Sum256(sum[:])
		}
	} else {
		key, _ = crypto.GenerateKey()
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)
	priv := hex.EncodeToString(crypto.FromECDSA(key))
	pub := hex.EncodeToString(crypto.FromECDSAPub(&key.PublicKey)[1:]) // Remove 04 prefix
	fmt.Printf("%s,%s,%s\n", addr.Hex(), priv, pub)
}
EOF

# Initialize go module for genkeys if explicitly requested
if [ "${RUN_GO_MOD_TIDY:-}" = "1" ]; then
    pushd "$TEST_INT_DIR" > /dev/null
    go mod tidy
    popd > /dev/null
fi

generate_key() {
    local seed="$1"
    local output
    if ! output=$(cd "$ROOT_DIR" && go run "$DATA_DIR/genkeys.go" "$seed"); then
        echo "❌ genkeys failed for seed '${seed}'" >&2
        exit 1
    fi
    echo "$output"
}

# Generate Keys
echo "Generating keys..."
# Funder
IFS=',' read -r FUNDER_ADDR FUNDER_PRIV FUNDER_PUB <<< "$(generate_key "funder-0")"
# Trim any potential whitespace/newlines
FUNDER_ADDR=$(echo "$FUNDER_ADDR" | tr -d '[:space:]')
echo "Funder: $FUNDER_ADDR"

# Validators (3 nodes) + Sync Node (1 node) = 4 nodes total
NUM_VALIDATORS=3
NUM_NODES=4
NODE_IPS=("172.28.0.10" "172.28.0.11" "172.28.0.12" "172.28.0.13")
VAL_ADDRS=()
VAL_PRIVS=()
ENODES=()
NODE_PUBS=()

# We generate for 0..3 (4 nodes). 0-2 are validators, 3 is sync.
for i in $(seq 0 $((NUM_NODES-1))); do
    # Create node directory
    mkdir -p "$DATA_DIR/node$i/keystore"
    mkdir -p "$DATA_DIR/node$i/geth" 
    
    # Node P2P Key
    IFS=',' read -r NODE_ADDR NODE_PRIV NODE_PUB <<< "$(generate_key "node-p2p-$i")"
    echo "$NODE_PRIV" > "$DATA_DIR/node$i/nodekey"
    echo "$NODE_PUB" > "$DATA_DIR/node$i/nodepub"
    NODE_PUBS+=("$NODE_PUB")
    
    # Construct Enode URL (use static IPs to avoid DNS resolution issues)
    NODE_HOST="node$i"
    if [ ${#NODE_IPS[@]} -gt $i ] && [ -n "${NODE_IPS[$i]}" ]; then
        NODE_HOST="${NODE_IPS[$i]}"
    fi
    ENODES+=("enode://$NODE_PUB@${NODE_HOST}:30303")
    
    if [ $i -lt $NUM_VALIDATORS ]; then
        # Validator keys for 0-2
        IFS=',' read -r ADDR PRIV PUB <<< "$(generate_key "validator-$i")"
        ADDR=$(echo "$ADDR" | tr -d '[:space:]')
        VAL_ADDRS+=($ADDR)
        VAL_PRIVS+=($PRIV)
        echo "Validator $i: $ADDR"
        echo "$PRIV" > "$DATA_DIR/node$i/validator.key"
        echo "$ADDR" > "$DATA_DIR/node$i/validator.addr"
    else
        echo "Node $i: Sync Node (No validator key)"
    fi
done

# Generate config.toml for each node
echo "Generating config.toml for nodes..."

# Prepare StaticNodes string: ["enode://...", "enode://..."]
STATIC_NODES_TOML="["
for i in $(seq 0 $((NUM_NODES-1))); do
    if [ $i -ne 0 ]; then STATIC_NODES_TOML+=', ';
    fi
    STATIC_NODES_TOML+="\"${ENODES[$i]}\""
done
STATIC_NODES_TOML+="]"

for i in $(seq 0 $((NUM_NODES-1))); do
    cat > "$DATA_DIR/node$i/config.toml" <<EOF
[Node]
UserIdent = "juchain-node$i"
DataDir = "/data"
IPCPath = "geth.ipc"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
WSHost = "0.0.0.0"
WSPort = 8546
[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
StaticNodes = $STATIC_NODES_TOML
EOF
done

# 2. Build Genesis
echo "Building genesis.json..."

# Generate System Contracts Alloc using the helper script
echo "Generating system contracts alloc..."
CHAIN_CONTRACT_ROOT="$CHAIN_CONTRACT_ROOT" CHAIN_CONTRACT_OUT="$CONTRACT_OUT_DIR" node "$SCRIPT_DIR/build_alloc.js" > "$DATA_DIR/sys_contracts.json"
if [ $? -ne 0 ]; then
    echo "❌ Failed to generate system contracts alloc"
    exit 1
fi

# Add Funder and Validators to Alloc
# Prepare comma-separated validators list
VAL_ADDRS_CSV=$(IFS=,; echo "${VAL_ADDRS[*]}")

echo "Merging alloc with funder and validators..."
ALLOC_JSON=$(node "$SCRIPT_DIR/merge_alloc.js" "$DATA_DIR/sys_contracts.json" "$FUNDER_ADDR" "$VAL_ADDRS_CSV")
if [ $? -ne 0 ]; then
    echo "❌ Failed to merge alloc"
    exit 1
fi

VANITY="0000000000000000000000000000000000000000000000000000000000000000"
VALIDATORS_HEX=""
for addr in "${VAL_ADDRS[@]}"; do
    # Remove 0x prefix
    VALIDATORS_HEX+="${addr:2}"
done
SUFFIX="0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" # 65 bytes zeros

EXTRA_DATA="0x${VANITY}${VALIDATORS_HEX}${SUFFIX}"

# Replace placeholders
echo "$ALLOC_JSON" > "$DATA_DIR/alloc.json"

jq --slurpfile allocList "$DATA_DIR/alloc.json" --arg extra "$EXTRA_DATA" \
   '.alloc = $allocList[0] | .extraData = $extra' "$TEMPLATE_GENESIS" > "$DATA_DIR/genesis.json"

# 3. Generate test_config.yaml
echo "Generating test_config.yaml..."
cat > "$DATA_DIR/test_config.yaml" <<EOF
rpcs:
  - "$PRIMARY_RPC"

funder:
  address: "$FUNDER_ADDR"
  private_key: "$FUNDER_PRIV"

validators:
EOF

for i in $(seq 0 $((NUM_VALIDATORS-1))); do
cat >> "$DATA_DIR/test_config.yaml" <<EOF
  - address: "${VAL_ADDRS[$i]}"
    private_key: "${VAL_PRIVS[$i]}"
EOF
done

cat >> "$DATA_DIR/test_config.yaml" <<EOF

validator_rpcs:
EOF
echo "  - \"http://localhost:${V1_HTTP}\"" >> "$DATA_DIR/test_config.yaml"
echo "  - \"http://localhost:${V2_HTTP}\"" >> "$DATA_DIR/test_config.yaml"
echo "  - \"http://localhost:${V3_HTTP}\"" >> "$DATA_DIR/test_config.yaml"

cat >> "$DATA_DIR/test_config.yaml" <<EOF
test:
  funding_amount: "100000000000000000000" # 100 ETH
EOF

awk '
  /^  node0:$/ { in_node0=1; in_node3=0 }
  /^  node1:$/ || /^  node2:$/ { in_node0=0; in_node3=0 }
  /^  node3:$/ { in_node3=1; in_node0=0 }
  in_node3 && skip_ports {
    if ($0 ~ /^      -/) { next }
    skip_ports=0
  }
  in_node3 && $0 ~ /^    ports:/ { skip_ports=1; next }
  in_node3 && !inserted && $0 ~ /^    volumes:/ {
    print "    ports:"
    print "      - \"18545:8545\""
    print "      - \"18546:8546\" # WS"
    inserted=1
  }
  { print }
' "$ROOT_DIR/docker/docker-compose.yml" > "$DATA_DIR/docker-compose.runtime.yml"

# Ensure runtime compose mounts the base image assets from test-integration/docker.
if [ "$(uname -s)" = "Darwin" ]; then
  sed -i '' \
    -e 's|\./juchain|../docker/juchain|g' \
    -e 's|\./start.sh|../docker/start.sh|g' \
    "$DATA_DIR/docker-compose.runtime.yml"
else
  sed -i \
    -e 's|\./juchain|../docker/juchain|g' \
    -e 's|\./start.sh|../docker/start.sh|g' \
    "$DATA_DIR/docker-compose.runtime.yml"
fi

echo "✅ Configuration generated at $DATA_DIR"
