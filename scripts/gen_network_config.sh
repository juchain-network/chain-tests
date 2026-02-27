#!/bin/bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
TEMPLATE_GENESIS="$(to_abs_path "./templates/genesis.tpl.json")"
PRIMARY_RPC="$(cfg_get "$CONFIG_FILE" "network.external_rpc" "http://localhost:18545")"

TEST_PROFILE="${TEST_PROFILE:-$(cfg_get "$CONFIG_FILE" "tests.profile" "fast")}"
profile_get() {
    local key="$1"
    local default_value="$2"
    cfg_get "$CONFIG_FILE" "tests.profiles.${TEST_PROFILE}.${key}" "$default_value"
}

PROFILE_EPOCH="$(profile_get "epoch" "")"
if [ -n "${TEST_NETWORK_EPOCH:-}" ]; then
    NETWORK_EPOCH="$TEST_NETWORK_EPOCH"
elif [ -n "$PROFILE_EPOCH" ]; then
    NETWORK_EPOCH="$PROFILE_EPOCH"
else
    NETWORK_EPOCH="$(cfg_get "$CONFIG_FILE" "network.epoch" "30")"
fi

PROFILE_PROPOSAL_COOLDOWN="$(profile_get "proposal_cooldown" "1")"
PROFILE_UNBONDING_PERIOD="$(profile_get "unbonding_period" "3")"
PROFILE_VALIDATOR_UNJAIL_PERIOD="$(profile_get "validator_unjail_period" "3")"
PROFILE_WITHDRAW_PROFIT_PERIOD="$(profile_get "withdraw_profit_period" "2")"
PROFILE_COMMISSION_UPDATE_COOLDOWN="$(profile_get "commission_update_cooldown" "1")"
PROFILE_PROPOSAL_LASTING_PERIOD="$(profile_get "proposal_lasting_period" "30")"
SMOKE_OBSERVE_SECONDS="$(cfg_get "$CONFIG_FILE" "tests.smoke.observe_seconds" "300")"
PREFUND_STAKING_INITIAL_STAKE="$(cfg_get "$CONFIG_FILE" "network.prefund_staking_initial_stake" "true")"
MIN_VALIDATOR_STAKE_WEI="$(cfg_get "$CONFIG_FILE" "network.min_validator_stake_wei" "1000000000000000000")"
GENESIS_MODE="${GENESIS_MODE:-$(cfg_get "$CONFIG_FILE" "network.genesis_mode" "posa")}"
FORK_TARGET="${FORK_TARGET:-$(cfg_get "$CONFIG_FILE" "network.fork_target" "")}"
FORK_DELAY_SECONDS="${FORK_DELAY_SECONDS:-$(cfg_get "$CONFIG_FILE" "network.fork_delay_seconds" "120")}"
FORK_SCHEDULER_SCRIPT="$SCRIPT_DIR/fork/set_fork_schedule.py"
POA_ALLOC_TEMPLATE="$(to_abs_path "./templates/alloc_poa_system_contracts.json")"

V1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_http" "18545")"
V1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_ws" "18546")"
V1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_p2p" "30301")"
V2_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_http" "18547")"
V2_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_ws" "18548")"
V2_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_p2p" "30303")"
V3_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_http" "18549")"
V3_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_ws" "18553")"
V3_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_p2p" "30305")"
S1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.sync_http" "18551")"
S1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.sync_ws" "18555")"
S1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.sync_p2p" "30307")"

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

if ! [[ "$NETWORK_EPOCH" =~ ^[0-9]+$ ]] || [ "$NETWORK_EPOCH" -le 0 ]; then
    die "network.epoch must be a positive integer, got: $NETWORK_EPOCH"
fi

for v in \
    "$PROFILE_PROPOSAL_COOLDOWN" \
    "$PROFILE_UNBONDING_PERIOD" \
    "$PROFILE_VALIDATOR_UNJAIL_PERIOD" \
    "$PROFILE_WITHDRAW_PROFIT_PERIOD" \
    "$PROFILE_COMMISSION_UPDATE_COOLDOWN" \
    "$PROFILE_PROPOSAL_LASTING_PERIOD" \
    "$SMOKE_OBSERVE_SECONDS"; do
    if ! [[ "$v" =~ ^[0-9]+$ ]] || [ "$v" -le 0 ]; then
        die "test timing/profile values must be positive integers, got: $v"
    fi
done

# Clean up data directory (handle root-owned files from Docker)
if [ -d "$DATA_DIR" ]; then
    rm -rf "$DATA_DIR" 2>/dev/null || true
    if [ -d "$DATA_DIR" ]; then
        docker run --rm -v "$ROOT_DIR":/work alpine sh -c "rm -rf /work/data" || true
    fi
fi
mkdir -p "$DATA_DIR"
mkdir -p "${GOCACHE:-$ROOT_DIR/.gocache}" "${GOMODCACHE:-$ROOT_DIR/.gomodcache}"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.gocache}"
export GOMODCACHE="${GOMODCACHE:-$ROOT_DIR/.gomodcache}"

echo "=== Generating Network Configuration ==="
echo "Using epoch: $NETWORK_EPOCH"
echo "Using test profile: $TEST_PROFILE"
echo "Using genesis mode: $GENESIS_MODE"
if [ "$GENESIS_MODE" = "upgrade" ]; then
    echo "Using upgrade fork target: $FORK_TARGET (delay=${FORK_DELAY_SECONDS}s)"
fi

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
[ -n "$FUNDER_ADDR" ] || die "failed to generate funder key"
echo "Funder: $FUNDER_ADDR"

# Validators + sync node
if [ -n "${TEST_NETWORK_VALIDATOR_COUNT:-}" ]; then
    NUM_VALIDATORS="$TEST_NETWORK_VALIDATOR_COUNT"
else
    NUM_VALIDATORS="$(cfg_get "$CONFIG_FILE" "network.validator_count" "3")"
fi
if [ -n "${TEST_NETWORK_NODE_COUNT:-}" ]; then
    NUM_NODES="$TEST_NETWORK_NODE_COUNT"
else
    NUM_NODES="$(cfg_get "$CONFIG_FILE" "network.node_count" "4")"
fi
NODE_IPS=("172.28.0.10" "172.28.0.11" "172.28.0.12" "172.28.0.13")
VAL_ADDRS=()
VAL_PRIVS=()
ENODES=()
NODE_PUBS=()

if ! [[ "$NUM_VALIDATORS" =~ ^[0-9]+$ ]] || [ "$NUM_VALIDATORS" -le 0 ]; then
    die "network.validator_count must be a positive integer, got: $NUM_VALIDATORS"
fi
if ! [[ "$NUM_NODES" =~ ^[0-9]+$ ]] || [ "$NUM_NODES" -lt "$NUM_VALIDATORS" ]; then
    die "network.node_count must be >= validator_count, got node_count=$NUM_NODES validator_count=$NUM_VALIDATORS"
fi
if [ "$NUM_NODES" -gt 4 ]; then
    die "network.node_count currently supports up to 4 nodes, got: $NUM_NODES"
fi
if ! [[ "$MIN_VALIDATOR_STAKE_WEI" =~ ^[0-9]+$ ]]; then
    die "network.min_validator_stake_wei must be an unsigned integer in wei, got: $MIN_VALIDATOR_STAKE_WEI"
fi
if ! [[ "$FORK_DELAY_SECONDS" =~ ^[0-9]+$ ]]; then
    die "network.fork_delay_seconds must be an unsigned integer in seconds, got: $FORK_DELAY_SECONDS"
fi
case "$GENESIS_MODE" in
    poa|posa|upgrade)
        ;;
    *)
        die "network.genesis_mode must be one of poa|posa|upgrade, got: $GENESIS_MODE"
        ;;
esac
if [ "$GENESIS_MODE" = "upgrade" ]; then
    case "$FORK_TARGET" in
        shanghaiTime|cancunTime|posaTime|fixHeaderTime|allStaggered|allSame)
            ;;
        *)
            die "FORK_TARGET must be one of shanghaiTime|cancunTime|posaTime|fixHeaderTime|allStaggered|allSame when GENESIS_MODE=upgrade, got: ${FORK_TARGET:-<empty>}"
            ;;
    esac
fi

# We generate for 0..3 (4 nodes). 0-2 are validators, 3 is sync.
for i in $(seq 0 $((NUM_NODES-1))); do
    # Create node directory
    mkdir -p "$DATA_DIR/node$i/keystore"
    mkdir -p "$DATA_DIR/node$i/geth" 
    
    # Node P2P Key
    IFS=',' read -r NODE_ADDR NODE_PRIV NODE_PUB <<< "$(generate_key "node-p2p-$i")"
    [ -n "$NODE_PRIV" ] || die "failed to generate p2p key for node$i"
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
        [ -n "$ADDR" ] || die "failed to generate validator key for node$i"
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
if [ "$GENESIS_MODE" = "poa" ] || [ "$GENESIS_MODE" = "upgrade" ]; then
    if [ ! -f "$POA_ALLOC_TEMPLATE" ]; then
        die "missing PoA alloc template: $POA_ALLOC_TEMPLATE"
    fi
    cp "$POA_ALLOC_TEMPLATE" "$DATA_DIR/sys_contracts.json"
else
    CHAIN_CONTRACT_ROOT="$CHAIN_CONTRACT_ROOT" \
    CHAIN_CONTRACT_OUT="$CONTRACT_OUT_DIR" \
    PREFUND_STAKING="$PREFUND_STAKING_INITIAL_STAKE" \
    MIN_VALIDATOR_STAKE_WEI="$MIN_VALIDATOR_STAKE_WEI" \
    VALIDATOR_COUNT="$NUM_VALIDATORS" \
    node "$SCRIPT_DIR/build_alloc.js" > "$DATA_DIR/sys_contracts.json"
    if [ $? -ne 0 ]; then
        echo "❌ Failed to generate system contracts alloc"
        exit 1
    fi
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

jq --slurpfile allocList "$DATA_DIR/alloc.json" --arg extra "$EXTRA_DATA" --arg epoch "$NETWORK_EPOCH" \
   '.alloc = $allocList[0] | .extraData = $extra | .config.congress.epoch = ($epoch | tonumber)' "$TEMPLATE_GENESIS" > "$DATA_DIR/genesis.json"

# Apply mode-specific fork schedule after template merge.
if [ ! -f "$FORK_SCHEDULER_SCRIPT" ]; then
    die "missing fork scheduler script: $FORK_SCHEDULER_SCRIPT"
fi
command -v python3 >/dev/null 2>&1 || die "python3 is required for fork schedule generation"
FORK_META_JSON="$(python3 "$FORK_SCHEDULER_SCRIPT" "$DATA_DIR/genesis.json" "$GENESIS_MODE" "$FORK_TARGET" "$FORK_DELAY_SECONDS")"
if [ -z "$FORK_META_JSON" ]; then
    die "fork scheduler returned empty metadata"
fi
FORK_EFFECTIVE_TARGET="$(printf '%s' "$FORK_META_JSON" | jq -r '.target // ""')"
FORK_SCHEDULED_TIME="$(printf '%s' "$FORK_META_JSON" | jq -r '.scheduled_time // 0')"
FORK_EFFECTIVE_DELAY_SECONDS="$(printf '%s' "$FORK_META_JSON" | jq -r '.effective_delay_seconds // '"$FORK_DELAY_SECONDS"'')"
FORK_SHANGHAI_TIME="$(printf '%s' "$FORK_META_JSON" | jq -r '.schedule.shanghaiTime // 0')"
FORK_CANCUN_TIME="$(printf '%s' "$FORK_META_JSON" | jq -r '.schedule.cancunTime // 0')"
FORK_FIX_HEADER_TIME="$(printf '%s' "$FORK_META_JSON" | jq -r '.schedule.fixHeaderTime // 0')"
FORK_POSA_TIME="$(printf '%s' "$FORK_META_JSON" | jq -r '.schedule.posaTime // 0')"

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
VALIDATOR_HTTP_PORTS=("$V1_HTTP" "$V2_HTTP" "$V3_HTTP")
for i in $(seq 0 $((NUM_VALIDATORS-1))); do
    if [ "$i" -lt "${#VALIDATOR_HTTP_PORTS[@]}" ]; then
        echo "  - \"http://localhost:${VALIDATOR_HTTP_PORTS[$i]}\"" >> "$DATA_DIR/test_config.yaml"
    fi
done

cat >> "$DATA_DIR/test_config.yaml" <<EOF

sync_rpc: ""

node_rpcs:
EOF

NODE_HTTP_PORTS=("$V1_HTTP" "$V2_HTTP" "$V3_HTTP" "$S1_HTTP")
for i in $(seq 0 $((NUM_NODES-1))); do
    role="validator"
    name="validator$((i+1))"
    if [ "$i" -ge "$NUM_VALIDATORS" ]; then
        role="sync"
        name="sync$((i-NUM_VALIDATORS+1))"
    fi
    if [ "$i" -lt "${#NODE_HTTP_PORTS[@]}" ]; then
        rpc_port="${NODE_HTTP_PORTS[$i]}"
    else
        continue
    fi
    cat >> "$DATA_DIR/test_config.yaml" <<EOF
  - name: "$name"
    role: "$role"
    url: "http://localhost:${rpc_port}"
EOF
done

if [ "$NUM_NODES" -gt "$NUM_VALIDATORS" ]; then
    if [ "$NUM_VALIDATORS" -lt "${#NODE_HTTP_PORTS[@]}" ]; then
        SYNC_PORT="${NODE_HTTP_PORTS[$NUM_VALIDATORS]}"
        if [ -n "$SYNC_PORT" ]; then
            if [ "$(uname -s)" = "Darwin" ]; then
                sed -i '' "s#^sync_rpc: \"\"#sync_rpc: \"http://localhost:${SYNC_PORT}\"#" "$DATA_DIR/test_config.yaml"
            else
                sed -i "s#^sync_rpc: \"\"#sync_rpc: \"http://localhost:${SYNC_PORT}\"#" "$DATA_DIR/test_config.yaml"
            fi
        fi
    fi
fi

cat >> "$DATA_DIR/test_config.yaml" <<EOF

network:
  epoch: $NETWORK_EPOCH

test:
  profile: "$TEST_PROFILE"
  funding_amount: "100000000000000000000" # 100 ETH
  smoke:
    observe_seconds: $SMOKE_OBSERVE_SECONDS
  params:
    proposal_cooldown: $PROFILE_PROPOSAL_COOLDOWN
    unbonding_period: $PROFILE_UNBONDING_PERIOD
    validator_unjail_period: $PROFILE_VALIDATOR_UNJAIL_PERIOD
    withdraw_profit_period: $PROFILE_WITHDRAW_PROFIT_PERIOD
    commission_update_cooldown: $PROFILE_COMMISSION_UPDATE_COOLDOWN
    proposal_lasting_period: $PROFILE_PROPOSAL_LASTING_PERIOD

fork:
  mode: "$GENESIS_MODE"
  target: "$FORK_EFFECTIVE_TARGET"
  scheduled_time: $FORK_SCHEDULED_TIME
  delay_seconds: $FORK_EFFECTIVE_DELAY_SECONDS
  schedule:
    shanghai_time: $FORK_SHANGHAI_TIME
    cancun_time: $FORK_CANCUN_TIME
    fix_header_time: $FORK_FIX_HEADER_TIME
    posa_time: $FORK_POSA_TIME
EOF

awk \
  -v v1_http="$V1_HTTP" -v v1_ws="$V1_WS" -v v1_p2p="$V1_P2P" \
  -v v2_http="$V2_HTTP" -v v2_ws="$V2_WS" -v v2_p2p="$V2_P2P" \
  -v v3_http="$V3_HTTP" -v v3_ws="$V3_WS" -v v3_p2p="$V3_P2P" \
  -v s1_http="$S1_HTTP" -v s1_ws="$S1_WS" -v s1_p2p="$S1_P2P" '
  function print_ports(node) {
    print "    ports:"
    if (node == "node0") {
      print "      - \"" v1_http ":8545\""
      print "      - \"" v1_ws ":8546\" # WS"
      print "      - \"" v1_p2p ":30303\" # P2P"
    } else if (node == "node1") {
      print "      - \"" v2_http ":8545\""
      print "      - \"" v2_ws ":8546\" # WS"
      print "      - \"" v2_p2p ":30303\" # P2P"
    } else if (node == "node2") {
      print "      - \"" v3_http ":8545\""
      print "      - \"" v3_ws ":8546\" # WS"
      print "      - \"" v3_p2p ":30303\" # P2P"
    } else if (node == "node3") {
      print "      - \"" s1_http ":8545\""
      print "      - \"" s1_ws ":8546\" # WS"
      print "      - \"" s1_p2p ":30303\" # P2P"
    }
  }

  /^  node[0-3]:$/ {
    current_node = substr($1, 1, length($1) - 1)
    skip_ports = 0
    inserted = 0
  }

  current_node != "" && /^  [A-Za-z0-9_-]+:$/ && $0 !~ /^  node[0-3]:$/ {
    current_node = ""
    skip_ports = 0
    inserted = 0
  }

  current_node != "" && skip_ports {
    if ($0 ~ /^      -/) {
      next
    }
    skip_ports = 0
  }

  current_node != "" && $0 ~ /^    ports:/ {
    skip_ports = 1
    next
  }

  current_node != "" && !inserted && $0 ~ /^    volumes:/ {
    print_ports(current_node)
    inserted = 1
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
