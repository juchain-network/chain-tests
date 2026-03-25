#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
DATA_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.data_dir" "./data")")"
TEMPLATE_GENESIS="$(to_abs_path "./templates/genesis.tpl.json")"
PRIMARY_RPC="$(cfg_get "$CONFIG_FILE" "network.external_rpc" "http://localhost:18545")"
RUNTIME_SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"

is_true() {
    case "$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')" in
        1|true|yes|on) return 0 ;;
        *) return 1 ;;
    esac
}

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
BOOTSTRAP_SIGNER_MODE="${BOOTSTRAP_SIGNER_MODE:-$(cfg_get "$CONFIG_FILE" "network.bootstrap.signer_mode" "separate")}"
BOOTSTRAP_FEE_MODE="${BOOTSTRAP_FEE_MODE:-$(cfg_get "$CONFIG_FILE" "network.bootstrap.fee_mode" "validator")}"
BOOTSTRAP_VALIDATOR_BALANCE_WEI="${BOOTSTRAP_VALIDATOR_BALANCE_WEI:-$(cfg_get "$CONFIG_FILE" "network.bootstrap.validator_balance_wei" "1000000000000000000000000000")}"
BOOTSTRAP_SIGNER_BALANCE_WEI="${BOOTSTRAP_SIGNER_BALANCE_WEI:-$(cfg_get "$CONFIG_FILE" "network.bootstrap.signer_balance_wei" "1000000000000000000")}"
BOOTSTRAP_FEE_BALANCE_WEI="${BOOTSTRAP_FEE_BALANCE_WEI:-$(cfg_get "$CONFIG_FILE" "network.bootstrap.fee_balance_wei" "1000000000000000000")}"
UPGRADE_OVERRIDE_POSA_TIME="${UPGRADE_OVERRIDE_POSA_TIME:-$(cfg_get "$CONFIG_FILE" "network.upgrade_override.posa_time" "")}"
UPGRADE_OVERRIDE_POSA_VALIDATORS_RAW="${UPGRADE_OVERRIDE_POSA_VALIDATORS:-}"
UPGRADE_OVERRIDE_POSA_SIGNERS_RAW="${UPGRADE_OVERRIDE_POSA_SIGNERS:-}"
CFG_UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON="$(cfg_get_json "$CONFIG_FILE" "network.upgrade_override.posa_validators" "[]")"
CFG_UPGRADE_OVERRIDE_POSA_SIGNERS_JSON="$(cfg_get_json "$CONFIG_FILE" "network.upgrade_override.posa_signers" "[]")"
TOPOLOGY="${TOPOLOGY:-${INIT_TOPOLOGY:-}}"
INIT_MODE="${INIT_MODE:-}"
INIT_TARGET="${INIT_TARGET:-}"
INIT_DELAY_SECONDS="${INIT_DELAY_SECONDS:-}"
GENESIS_MODE="${GENESIS_MODE:-$(cfg_get "$CONFIG_FILE" "network.genesis_mode" "posa")}"
FORK_TARGET="${FORK_TARGET:-$(cfg_get "$CONFIG_FILE" "network.fork_target" "")}"
FORK_DELAY_SECONDS="${FORK_DELAY_SECONDS:-$(cfg_get "$CONFIG_FILE" "network.fork_delay_seconds" "120")}"
if [ -n "$INIT_MODE" ]; then
    GENESIS_MODE="$INIT_MODE"
fi
if [ -n "$INIT_TARGET" ]; then
    FORK_TARGET="$INIT_TARGET"
fi
if [ -n "$INIT_DELAY_SECONDS" ]; then
    FORK_DELAY_SECONDS="$INIT_DELAY_SECONDS"
fi
FORK_SCHEDULER_SCRIPT="$SCRIPT_DIR/fork/set_fork_schedule.py"
POA_ALLOC_TEMPLATE="$(to_abs_path "./templates/alloc_poa_system_contracts.json")"
RUNTIME_IMPL_MODE="$(cfg_get "$CONFIG_FILE" "runtime.impl_mode" "single")"
DEFAULT_RUNTIME_IMPL="$(cfg_get "$CONFIG_FILE" "runtime.impl" "geth")"
SESSION_BACKEND="${RUNTIME_BACKEND:-$(cfg_get "$CONFIG_FILE" "runtime.backend" "native")}"
VALIDATOR_AUTH_MODE="$(cfg_get "$CONFIG_FILE" "validator_auth.mode" "auto")"
VALIDATOR_KEYSTORE_PASSWORD_FILE_CFG="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_file" "")"
VALIDATOR_KEYSTORE_PASSWORD_ENV="$(cfg_get "$CONFIG_FILE" "validator_auth.keystore.password_env" "")"

CHAIN_ROOT="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "paths.chain_root" "../chain")")"
RETH_ROOT="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "paths.reth_root" "../rchain")")"
RETH_BYTECODE_FILE_CFG="$(cfg_get "$CONFIG_FILE" "paths.reth_bytecode_file" "")"
if [ -n "$RETH_BYTECODE_FILE_CFG" ]; then
    RETH_BYTECODE_FILE="$(to_abs_path "$RETH_BYTECODE_FILE_CFG")"
else
    RETH_BYTECODE_FILE="$RETH_ROOT/crates/congress-core/src/bytecode.rs"
fi

GETH_NATIVE_BIN_CFG="$(cfg_get "$CONFIG_FILE" "binaries.geth_native" "$(cfg_get "$CONFIG_FILE" "native.geth_binary" "")")"
RETH_NATIVE_BIN_CFG="$(cfg_get "$CONFIG_FILE" "binaries.reth_native" "$(cfg_get "$CONFIG_FILE" "native.reth_binary" "")")"
GETH_DOCKER_BIN_CFG="$(cfg_get "$CONFIG_FILE" "binaries.geth_docker" "$(cfg_get "$CONFIG_FILE" "paths.chain_docker_binary" "")")"
RETH_DOCKER_BIN_CFG="$(cfg_get "$CONFIG_FILE" "binaries.reth_docker" "")"

REPORTS_DIR="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "network.reports_dir" "./reports")")"
DOCKER_COMPOSE_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "docker.compose_file" "./docker/docker-compose.yml")")"
DOCKER_RUNTIME_COMPOSE_FILE="$DATA_DIR/docker-compose.runtime.yml"

NODE0_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node0" "")"
NODE1_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node1" "")"
NODE2_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node2" "")"
NODE3_IMPL_CFG="$(cfg_get "$CONFIG_FILE" "runtime_nodes.node3" "")"
BLACKLIST_ENABLED_RAW="$(cfg_get "$CONFIG_FILE" "blacklist.enabled" "false")"
BLACKLIST_MODE="$(cfg_get "$CONFIG_FILE" "blacklist.mode" "mock")"
BLACKLIST_CONTRACT_ADDR="$(cfg_get "$CONFIG_FILE" "blacklist.contract_address" "0x1db0EDE439708A923431DC68fd3F646c0A4D4e6E")"
BLACKLIST_ALERT_FAIL_OPEN_RAW="$(cfg_get "$CONFIG_FILE" "blacklist.alert_fail_open" "true")"
BLACKLIST_MOCK_PREDEPLOY_RAW="$(cfg_get "$CONFIG_FILE" "blacklist.mock.predeploy" "true")"
BLACKLIST_MOCK_CODE_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "blacklist.mock.code_file" "./templates/blacklist/mock_runtime_hex.txt")")"
BLACKLIST_MOCK_ABI_FILE="$(to_abs_path "$(cfg_get "$CONFIG_FILE" "blacklist.mock.abi_file" "./templates/blacklist/mock_abi.json")")"
BLACKLIST_ALLOC_SCRIPT="$SCRIPT_DIR/add_blacklist_alloc.js"

BLACKLIST_ENABLED=false
BLACKLIST_ALERT_FAIL_OPEN=false
BLACKLIST_MOCK_PREDEPLOY=false
if is_true "$BLACKLIST_ENABLED_RAW"; then BLACKLIST_ENABLED=true; fi
if is_true "$BLACKLIST_ALERT_FAIL_OPEN_RAW"; then BLACKLIST_ALERT_FAIL_OPEN=true; fi
if is_true "$BLACKLIST_MOCK_PREDEPLOY_RAW"; then BLACKLIST_MOCK_PREDEPLOY=true; fi

normalize_impl() {
    local impl="${1:-}"
    case "$impl" in
        "" ) echo "" ;;
        geth|reth) echo "$impl" ;;
        *) die "unsupported runtime implementation: $impl (expected geth|reth)" ;;
    esac
}

resolve_node_impl() {
    local idx="$1"
    local node_cfg="$2"
    local resolved=""
    case "$RUNTIME_IMPL_MODE" in
        single)
            resolved="$DEFAULT_RUNTIME_IMPL"
            ;;
        mixed)
            resolved="${node_cfg:-$DEFAULT_RUNTIME_IMPL}"
            ;;
        *)
            die "runtime.impl_mode must be single|mixed, got: $RUNTIME_IMPL_MODE"
            ;;
    esac
    normalize_impl "$resolved"
}

V1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_http" "18545")"
V1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_ws" "18546")"
V1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator1_p2p" "40401")"
V2_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_http" "18547")"
V2_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_ws" "18548")"
V2_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator2_p2p" "40403")"
V3_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_http" "18549")"
V3_WS="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_ws" "18553")"
V3_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.validator3_p2p" "40405")"
S1_HTTP="$(cfg_get "$CONFIG_FILE" "native.ports.sync_http" "18551")"
S1_WS="$(cfg_get "$CONFIG_FILE" "native.ports.sync_ws" "18555")"
S1_P2P="$(cfg_get "$CONFIG_FILE" "native.ports.sync_p2p" "40407")"

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

if [ "$BLACKLIST_ENABLED" = true ] && [ "$BLACKLIST_MODE" = "mock" ] && [ "$BLACKLIST_MOCK_PREDEPLOY" = true ]; then
    [ -f "$BLACKLIST_MOCK_CODE_FILE" ] || die "blacklist mock runtime code file not found: $BLACKLIST_MOCK_CODE_FILE"
    [ -f "$BLACKLIST_MOCK_ABI_FILE" ] || die "blacklist mock abi file not found: $BLACKLIST_MOCK_ABI_FILE"
    [ -f "$BLACKLIST_ALLOC_SCRIPT" ] || die "missing blacklist alloc script: $BLACKLIST_ALLOC_SCRIPT"
fi

if ! [[ "$NETWORK_EPOCH" =~ ^[0-9]+$ ]] || [ "$NETWORK_EPOCH" -le 0 ]; then
    die "network.epoch must be a positive integer, got: $NETWORK_EPOCH"
fi

DEFAULT_RUNTIME_IMPL="$(normalize_impl "$DEFAULT_RUNTIME_IMPL")"
case "$SESSION_BACKEND" in
    native|docker)
        ;;
    *)
        die "runtime backend must be native|docker, got: $SESSION_BACKEND"
        ;;
esac
case "$TOPOLOGY" in
    ""|single|multi)
        ;;
    *)
        die "TOPOLOGY must be one of single|multi, got: $TOPOLOGY"
        ;;
esac
if [ "$TOPOLOGY" = "single" ] && [ "$SESSION_BACKEND" = "docker" ]; then
    die "single topology currently supports native backend only; set runtime.backend=native or use TOPOLOGY=multi"
fi
case "$RUNTIME_IMPL_MODE" in
    single|mixed)
        ;;
    *)
        die "runtime.impl_mode must be single|mixed, got: $RUNTIME_IMPL_MODE"
        ;;
esac
case "$VALIDATOR_AUTH_MODE" in
    auto|private_key|keystore)
        ;;
    *)
        die "validator_auth.mode must be auto|private_key|keystore, got: $VALIDATOR_AUTH_MODE"
        ;;
esac

VALIDATOR_KEYSTORE_PASSWORD_VALUE="${VALIDATOR_KEYSTORE_PASSWORD_VALUE:-}"
if [ -z "$VALIDATOR_KEYSTORE_PASSWORD_VALUE" ] && [ -n "$VALIDATOR_KEYSTORE_PASSWORD_FILE_CFG" ]; then
    VALIDATOR_KEYSTORE_PASSWORD_FILE_ABS="$(to_abs_path "$VALIDATOR_KEYSTORE_PASSWORD_FILE_CFG")"
    [ -f "$VALIDATOR_KEYSTORE_PASSWORD_FILE_ABS" ] || die "validator_auth.keystore.password_file not found: $VALIDATOR_KEYSTORE_PASSWORD_FILE_ABS"
    VALIDATOR_KEYSTORE_PASSWORD_VALUE="$(tr -d '\r' < "$VALIDATOR_KEYSTORE_PASSWORD_FILE_ABS" | head -n 1)"
fi
if [ -z "$VALIDATOR_KEYSTORE_PASSWORD_VALUE" ] && [ -n "$VALIDATOR_KEYSTORE_PASSWORD_ENV" ]; then
    VALIDATOR_KEYSTORE_PASSWORD_VALUE="$(printenv "$VALIDATOR_KEYSTORE_PASSWORD_ENV" || true)"
fi
if [ -z "$VALIDATOR_KEYSTORE_PASSWORD_VALUE" ]; then
    VALIDATOR_KEYSTORE_PASSWORD_VALUE="123456"
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
rm -f "$DATA_DIR/genkeys.go" "$DATA_DIR/genkeystore.go"

echo "=== Generating Network Configuration ==="
echo "Using epoch: $NETWORK_EPOCH"
echo "Using test profile: $TEST_PROFILE"
echo "Requested topology: ${TOPOLOGY:-auto}"
echo "Using genesis mode: $GENESIS_MODE"
echo "Using runtime impl: mode=$RUNTIME_IMPL_MODE default=$DEFAULT_RUNTIME_IMPL auth=$VALIDATOR_AUTH_MODE"
echo "Using bootstrap identities: signer_mode=$BOOTSTRAP_SIGNER_MODE fee_mode=$BOOTSTRAP_FEE_MODE (Hardhat funder=0 validators=1..3 signers=4..6)"
echo "Using runtime backend: $SESSION_BACKEND"
echo "Using blacklist: enabled=$BLACKLIST_ENABLED mode=$BLACKLIST_MODE addr=$BLACKLIST_CONTRACT_ADDR"
if [ "$GENESIS_MODE" = "upgrade" ]; then
    echo "Using upgrade fork target: $FORK_TARGET (delay=${FORK_DELAY_SECONDS}s)"
elif [ "$GENESIS_MODE" = "smoke" ]; then
    echo "Using smoke static fork case: $FORK_TARGET"
fi
if [ -n "$UPGRADE_OVERRIDE_POSA_TIME" ] || [ -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS_RAW" ] || [ -n "$UPGRADE_OVERRIDE_POSA_SIGNERS_RAW" ] || [ "$CFG_UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON" != "[]" ] || [ "$CFG_UPGRADE_OVERRIDE_POSA_SIGNERS_JSON" != "[]" ]; then
    echo "Using upgrade override inputs: posaTime=${UPGRADE_OVERRIDE_POSA_TIME:-<empty>}"
fi

generate_key() {
    local seed="$1"
    local output
    if ! output=$(cd "$ROOT_DIR" && go run ./cmd/genkeys "$seed"); then
        echo "❌ genkeys failed for seed '${seed}'" >&2
        exit 1
    fi
    echo "$output"
}

generate_hardhat_key() {
    local index="$1"
    local output
    if ! output=$(cd "$ROOT_DIR" && go run ./cmd/genhardhat "$index"); then
        echo "❌ genhardhat failed for index ${index}" >&2
        exit 1
    fi
    echo "$output"
}

generate_keystore() {
    local priv="$1"
    local password="$2"
    local out_dir="$3"
    local address_file="$4"
    local output
    if ! output=$(cd "$ROOT_DIR" && go run ./cmd/genkeystore "$priv" "$password" "$out_dir" "$address_file"); then
        echo "❌ genkeystore failed for out_dir '${out_dir}'" >&2
        exit 1
    fi
    echo "$output"
}

validate_bootstrap_mapping() {
    local validators_json="$1"
    local signers_json="$2"
    python3 - "$validators_json" "$signers_json" <<'PY'
import json
import sys

validators = json.loads(sys.argv[1])
signers = json.loads(sys.argv[2])

if not validators or not signers:
    raise SystemExit("bootstrap validator/signer set must not be empty")
if len(validators) != len(signers):
    raise SystemExit(f"bootstrap validator/signer length mismatch: {len(validators)} != {len(signers)}")
if len(validators) > 21:
    raise SystemExit(f"bootstrap validator/signer count exceeds max 21: {len(validators)}")

def validate(name, values):
    seen = set()
    for idx, value in enumerate(values):
        if not isinstance(value, str) or not value.startswith("0x") or len(value) != 42:
            raise SystemExit(f"{name}[{idx}] is not a hex address: {value!r}")
        if value.lower() == "0x0000000000000000000000000000000000000000":
            raise SystemExit(f"{name}[{idx}] must not be zero address")
        lowered = value.lower()
        if lowered in seen:
            raise SystemExit(f"{name}[{idx}] duplicates address {value}")
        seen.add(lowered)

validate("validators", validators)
validate("signers", signers)
PY
}

csv_addresses_to_json() {
    local raw="${1:-}"
    python3 - "$raw" <<'PY'
import json
import sys

raw = sys.argv[1].strip()
if not raw:
    print("[]")
    raise SystemExit(0)

items = []
for item in raw.split(","):
    value = item.strip()
    if value:
        items.append(value)

print(json.dumps(items))
PY
}

json_addresses_to_csv() {
    local raw_json="${1:-[]}"
    python3 - "$raw_json" <<'PY'
import json
import sys

try:
    items = json.loads(sys.argv[1] or "[]")
except Exception:
    items = []

if not isinstance(items, list):
    items = []

print(",".join(str(item).strip() for item in items if str(item).strip()))
PY
}

validate_optional_upgrade_override() {
    local validators_json="$1"
    local signers_json="$2"
    local mode="$3"
    local target="$4"
    python3 - "$validators_json" "$signers_json" "$mode" "$target" <<'PY'
import json
import sys

validators = json.loads(sys.argv[1] or "[]")
signers = json.loads(sys.argv[2] or "[]")
mode = (sys.argv[3] or "").strip().lower()
target = (sys.argv[4] or "").strip()

if not validators and not signers:
    raise SystemExit(0)

if not validators or not signers:
    raise SystemExit("upgrade override validators/signers must be provided together")

if mode not in {"upgrade", "smoke"}:
    raise SystemExit(f"upgrade override only supports upgrade/smoke migration modes, got: {mode or '<empty>'}")

if mode == "smoke" and target != "poa_shanghai_cancun_fixheader_posa":
    raise SystemExit(
        "upgrade override in smoke mode only supports fork target poa_shanghai_cancun_fixheader_posa"
    )

def validate(name, values):
    seen = set()
    for idx, value in enumerate(values):
        if not isinstance(value, str) or not value.startswith("0x") or len(value) != 42:
            raise SystemExit(f"{name}[{idx}] is not a hex address: {value!r}")
        if value.lower() == "0x0000000000000000000000000000000000000000":
            raise SystemExit(f"{name}[{idx}] must not be zero address")
        lowered = value.lower()
        if lowered in seen:
            raise SystemExit(f"{name}[{idx}] duplicates address {value}")
        seen.add(lowered)

validate("override.validators", validators)
validate("override.signers", signers)
PY
}

# Validators + sync node
if [ "$TOPOLOGY" = "single" ]; then
    NUM_VALIDATORS=1
    NUM_NODES=1
elif [ "$TOPOLOGY" = "multi" ]; then
    NUM_VALIDATORS="$(cfg_get "$CONFIG_FILE" "network.validator_count" "3")"
    NUM_NODES="$(cfg_get "$CONFIG_FILE" "network.node_count" "4")"
elif [ -n "${TEST_NETWORK_VALIDATOR_COUNT:-}" ]; then
    NUM_VALIDATORS="$TEST_NETWORK_VALIDATOR_COUNT"
    if [ -n "${TEST_NETWORK_NODE_COUNT:-}" ]; then
        NUM_NODES="$TEST_NETWORK_NODE_COUNT"
    else
        NUM_NODES="$(cfg_get "$CONFIG_FILE" "network.node_count" "4")"
    fi
elif [ -n "${TEST_NETWORK_NODE_COUNT:-}" ]; then
    NUM_NODES="$TEST_NETWORK_NODE_COUNT"
    NUM_VALIDATORS="$(cfg_get "$CONFIG_FILE" "network.validator_count" "3")"
else
    NUM_VALIDATORS="$(cfg_get "$CONFIG_FILE" "network.validator_count" "3")"
    NUM_NODES="$(cfg_get "$CONFIG_FILE" "network.node_count" "4")"
fi

TOPOLOGY_MODE=""
NODE_IPS=("172.28.0.10" "172.28.0.11" "172.28.0.12" "172.28.0.13")
VAL_ADDRS=()
VAL_PRIVS=()
SIGNER_ADDRS=()
SIGNER_PRIVS=()
FEE_ADDRS=()
ENODES=()
NODE_PUBS=()
RUNTIME_NODE_IMPLS=()
VAL_KEYSTORE_FILES=()
VAL_KEYSTORE_ADDRS=()

if ! [[ "$NUM_VALIDATORS" =~ ^[0-9]+$ ]] || [ "$NUM_VALIDATORS" -le 0 ]; then
    die "network.validator_count must be a positive integer, got: $NUM_VALIDATORS"
fi
if ! [[ "$NUM_NODES" =~ ^[0-9]+$ ]] || [ "$NUM_NODES" -lt "$NUM_VALIDATORS" ]; then
    die "network.node_count must be >= validator_count, got node_count=$NUM_NODES validator_count=$NUM_VALIDATORS"
fi
if [ "$NUM_NODES" -gt 4 ]; then
    die "network.node_count currently supports up to 4 nodes, got: $NUM_NODES"
fi
TOPOLOGY_MODE="multi"
if [ "$NUM_NODES" -eq 1 ] && [ "$NUM_VALIDATORS" -eq 1 ]; then
    TOPOLOGY_MODE="single"
fi
echo "Resolved topology: $TOPOLOGY_MODE (nodes=$NUM_NODES, validators=$NUM_VALIDATORS)"
if ! [[ "$FORK_DELAY_SECONDS" =~ ^[0-9]+$ ]]; then
    die "network.fork_delay_seconds must be an unsigned integer in seconds, got: $FORK_DELAY_SECONDS"
fi
case "$BOOTSTRAP_SIGNER_MODE" in
    same_address|separate)
        ;;
    *)
        die "network.bootstrap.signer_mode must be one of same_address|separate, got: $BOOTSTRAP_SIGNER_MODE"
        ;;
esac
case "$BOOTSTRAP_FEE_MODE" in
    validator|signer)
        ;;
    *)
        die "network.bootstrap.fee_mode must be one of validator|signer, got: $BOOTSTRAP_FEE_MODE"
        ;;
esac
for balance_spec in \
    "network.bootstrap.validator_balance_wei:$BOOTSTRAP_VALIDATOR_BALANCE_WEI" \
    "network.bootstrap.signer_balance_wei:$BOOTSTRAP_SIGNER_BALANCE_WEI" \
    "network.bootstrap.fee_balance_wei:$BOOTSTRAP_FEE_BALANCE_WEI"; do
    key="${balance_spec%%:*}"
    value="${balance_spec#*:}"
    if ! [[ "$value" =~ ^[0-9]+$ ]]; then
        die "$key must be an unsigned integer in wei, got: $value"
    fi
done
case "$BLACKLIST_MODE" in
    mock|real)
        ;;
    *)
        die "blacklist.mode must be one of mock|real, got: $BLACKLIST_MODE"
        ;;
esac
if ! [[ "$BLACKLIST_CONTRACT_ADDR" =~ ^0x[0-9a-fA-F]{40}$ ]]; then
    die "blacklist.contract_address must be a hex address, got: $BLACKLIST_CONTRACT_ADDR"
fi
case "$GENESIS_MODE" in
    poa|posa|upgrade|smoke)
        ;;
    *)
        die "network.genesis_mode must be one of poa|posa|upgrade|smoke, got: $GENESIS_MODE"
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
if [ "$GENESIS_MODE" = "smoke" ]; then
    case "$FORK_TARGET" in
        poa|poa_shanghai|poa_shanghai_cancun|poa_shanghai_cancun_fixheader|poa_shanghai_cancun_fixheader_posa)
            ;;
        *)
            die "FORK_TARGET must be one of poa|poa_shanghai|poa_shanghai_cancun|poa_shanghai_cancun_fixheader|poa_shanghai_cancun_fixheader_posa when GENESIS_MODE=smoke, got: ${FORK_TARGET:-<empty>}"
            ;;
    esac
fi
if [ -n "$UPGRADE_OVERRIDE_POSA_TIME" ] && ! [[ "$UPGRADE_OVERRIDE_POSA_TIME" =~ ^[0-9]+$ ]]; then
    die "network.upgrade_override.posa_time must be an unsigned integer timestamp, got: $UPGRADE_OVERRIDE_POSA_TIME"
fi

if [ -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS_RAW" ]; then
    UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON="$(csv_addresses_to_json "$UPGRADE_OVERRIDE_POSA_VALIDATORS_RAW")"
else
    UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON="$CFG_UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON"
fi
if [ -n "$UPGRADE_OVERRIDE_POSA_SIGNERS_RAW" ]; then
    UPGRADE_OVERRIDE_POSA_SIGNERS_JSON="$(csv_addresses_to_json "$UPGRADE_OVERRIDE_POSA_SIGNERS_RAW")"
else
    UPGRADE_OVERRIDE_POSA_SIGNERS_JSON="$CFG_UPGRADE_OVERRIDE_POSA_SIGNERS_JSON"
fi

validate_optional_upgrade_override "$UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON" "$UPGRADE_OVERRIDE_POSA_SIGNERS_JSON" "$GENESIS_MODE" "$FORK_TARGET"
UPGRADE_OVERRIDE_POSA_VALIDATORS_CSV="$(json_addresses_to_csv "$UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON")"
UPGRADE_OVERRIDE_POSA_SIGNERS_CSV="$(json_addresses_to_csv "$UPGRADE_OVERRIDE_POSA_SIGNERS_JSON")"

# Generate Keys
echo "Generating keys..."
if [ "$NUM_VALIDATORS" -gt 3 ] && [ "$BOOTSTRAP_SIGNER_MODE" = "separate" ]; then
    die "separate bootstrap signer mapping currently supports up to 3 validators (Hardhat indices 1..3 for validators, 4..6 for signers), got: $NUM_VALIDATORS"
fi
# Funder
IFS=',' read -r FUNDER_ADDR FUNDER_PRIV FUNDER_PUB <<< "$(generate_hardhat_key 0)"
# Trim any potential whitespace/newlines
FUNDER_ADDR=$(echo "$FUNDER_ADDR" | tr -d '[:space:]')
[ -n "$FUNDER_ADDR" ] || die "failed to generate funder key"
echo "Funder: $FUNDER_ADDR"

NODE_IMPL_CONFIGS=("$NODE0_IMPL_CFG" "$NODE1_IMPL_CFG" "$NODE2_IMPL_CFG" "$NODE3_IMPL_CFG")
for i in $(seq 0 $((NUM_NODES-1))); do
    node_cfg=""
    if [ "$i" -lt "${#NODE_IMPL_CONFIGS[@]}" ]; then
        node_cfg="${NODE_IMPL_CONFIGS[$i]}"
    fi
    node_impl="$(resolve_node_impl "$i" "$node_cfg")"
    RUNTIME_NODE_IMPLS+=("$node_impl")
done

if { [ -n "$UPGRADE_OVERRIDE_POSA_TIME" ] || [ -n "$UPGRADE_OVERRIDE_POSA_VALIDATORS_CSV" ] || [ -n "$UPGRADE_OVERRIDE_POSA_SIGNERS_CSV" ]; }; then
    for node_impl in "${RUNTIME_NODE_IMPLS[@]}"; do
        if [ "$node_impl" = "reth" ]; then
            die "upgrade override currently supports geth runtime only; reth node detected"
        fi
    done
fi

# We generate for 0..3 (4 nodes). 0-2 are validators, 3 is sync.
for i in $(seq 0 $((NUM_NODES-1))); do
    # Create node directory
    mkdir -p "$DATA_DIR/node$i/keystore"
    mkdir -p "$DATA_DIR/node$i/geth" 
    
    # Node P2P Key
    IFS=',' read -r NODE_ADDR NODE_PRIV NODE_PUB <<< "$(generate_key "node-p2p-$i")"
    [ -n "$NODE_PRIV" ] || die "failed to generate p2p key for node$i"
    printf '%s' "$NODE_PRIV" > "$DATA_DIR/node$i/nodekey"
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
        IFS=',' read -r ADDR PRIV PUB <<< "$(generate_hardhat_key "$((i + 1))")"
        ADDR=$(echo "$ADDR" | tr -d '[:space:]')
        [ -n "$ADDR" ] || die "failed to generate validator key for node$i"
        SIGNER_ADDR="$ADDR"
        SIGNER_PRIV="$PRIV"
        if [ "$BOOTSTRAP_SIGNER_MODE" = "separate" ]; then
            IFS=',' read -r SIGNER_ADDR SIGNER_PRIV SIGNER_PUB <<< "$(generate_hardhat_key "$((i + 4))")"
            SIGNER_ADDR=$(echo "$SIGNER_ADDR" | tr -d '[:space:]')
            [ -n "$SIGNER_ADDR" ] || die "failed to generate signer key for node$i"
        fi
        FEE_ADDR="$ADDR"
        if [ "$BOOTSTRAP_FEE_MODE" = "signer" ]; then
            FEE_ADDR="$SIGNER_ADDR"
        fi
        VAL_ADDRS+=("$ADDR")
        VAL_PRIVS+=("$PRIV")
        SIGNER_ADDRS+=("$SIGNER_ADDR")
        SIGNER_PRIVS+=("$SIGNER_PRIV")
        FEE_ADDRS+=("$FEE_ADDR")
        echo "Validator $i: cold=$ADDR signer=$SIGNER_ADDR fee=$FEE_ADDR"
        echo "$PRIV" > "$DATA_DIR/node$i/validator.key"
        echo "$ADDR" > "$DATA_DIR/node$i/validator.addr"
        echo "$SIGNER_PRIV" > "$DATA_DIR/node$i/signer.key"
        echo "$SIGNER_ADDR" > "$DATA_DIR/node$i/signer.addr"
        printf '%s\n' "$VALIDATOR_KEYSTORE_PASSWORD_VALUE" > "$DATA_DIR/node$i/password.txt"

        keystore_file="$(generate_keystore "$SIGNER_PRIV" "$VALIDATOR_KEYSTORE_PASSWORD_VALUE" "$DATA_DIR/node$i/keystore" "$DATA_DIR/node$i/keystore.addr")"
        VAL_KEYSTORE_FILES+=("$keystore_file")
        VAL_KEYSTORE_ADDRS+=("$(tr -d '[:space:]' < "$DATA_DIR/node$i/keystore.addr")")
    else
        echo "Node $i: Sync Node (No validator key)"
    fi
done

BOOTSTRAP_VALIDATORS_JSON="$(printf '%s\n' "${VAL_ADDRS[@]}" | jq -R . | jq -cs .)"
BOOTSTRAP_SIGNERS_JSON="$(printf '%s\n' "${SIGNER_ADDRS[@]}" | jq -R . | jq -cs .)"
BOOTSTRAP_FEE_ADDRS_JSON="$(printf '%s\n' "${FEE_ADDRS[@]}" | jq -R . | jq -cs .)"
validate_bootstrap_mapping "$BOOTSTRAP_VALIDATORS_JSON" "$BOOTSTRAP_SIGNERS_JSON"

cat > "$DATA_DIR/runtime_nodes.yaml" <<EOF
runtime:
  impl_mode: "$RUNTIME_IMPL_MODE"
  impl: "$DEFAULT_RUNTIME_IMPL"
validator_auth:
  mode: "$VALIDATOR_AUTH_MODE"
nodes:
EOF

for i in $(seq 0 $((NUM_NODES-1))); do
    impl="${RUNTIME_NODE_IMPLS[$i]}"
    role="sync"
    if [ "$i" -lt "$NUM_VALIDATORS" ]; then
        role="validator"
    fi
    cat >> "$DATA_DIR/runtime_nodes.yaml" <<EOF
  node$i:
    role: "$role"
    impl: "$impl"
    datadir: "$DATA_DIR/node$i"
    nodekey: "$DATA_DIR/node$i/nodekey"
EOF
    if [ "$i" -lt "$NUM_VALIDATORS" ]; then
        cat >> "$DATA_DIR/runtime_nodes.yaml" <<EOF
    validator_key: "$DATA_DIR/node$i/validator.key"
    validator_address: "$(tr -d '[:space:]' < "$DATA_DIR/node$i/validator.addr")"
    signer_key: "$DATA_DIR/node$i/signer.key"
    signer_address: "$(tr -d '[:space:]' < "$DATA_DIR/node$i/signer.addr")"
    fee_address: "${FEE_ADDRS[$i]}"
    keystore_file: "${VAL_KEYSTORE_FILES[$i]}"
    keystore_address: "${VAL_KEYSTORE_ADDRS[$i]}"
    password_file: "$DATA_DIR/node$i/password.txt"
EOF
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
USE_POA_ALLOC=false
case "$GENESIS_MODE" in
    poa|upgrade)
        USE_POA_ALLOC=true
        ;;
    smoke)
        case "$FORK_TARGET" in
            poa|poa_shanghai|poa_shanghai_cancun|poa_shanghai_cancun_fixheader)
                USE_POA_ALLOC=true
                ;;
            poa_shanghai_cancun_fixheader_posa)
                USE_POA_ALLOC=false
                ;;
            *)
                die "unsupported smoke fork target for alloc selection: ${FORK_TARGET:-<empty>}"
                ;;
        esac
        ;;
esac

if [ "$USE_POA_ALLOC" = true ]; then
    if [ ! -f "$POA_ALLOC_TEMPLATE" ]; then
        die "missing PoA alloc template: $POA_ALLOC_TEMPLATE"
    fi
    cp "$POA_ALLOC_TEMPLATE" "$DATA_DIR/sys_contracts.json"
else
    CHAIN_CONTRACT_ROOT="$CHAIN_CONTRACT_ROOT" \
    CHAIN_CONTRACT_OUT="$CONTRACT_OUT_DIR" \
    node "$SCRIPT_DIR/build_alloc.js" > "$DATA_DIR/sys_contracts.json"
    if [ $? -ne 0 ]; then
        echo "❌ Failed to generate system contracts alloc"
        exit 1
    fi
fi

# Add Funder and bootstrap identities to alloc
echo "Merging alloc with funder and bootstrap validator/signer balances..."
ALLOC_JSON=$(
    BOOTSTRAP_VALIDATORS_JSON="$BOOTSTRAP_VALIDATORS_JSON" \
    BOOTSTRAP_SIGNERS_JSON="$BOOTSTRAP_SIGNERS_JSON" \
    BOOTSTRAP_FEE_ADDRESSES_JSON="$BOOTSTRAP_FEE_ADDRS_JSON" \
    UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON="$UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON" \
    UPGRADE_OVERRIDE_POSA_SIGNERS_JSON="$UPGRADE_OVERRIDE_POSA_SIGNERS_JSON" \
    BOOTSTRAP_VALIDATOR_BALANCE_WEI="$BOOTSTRAP_VALIDATOR_BALANCE_WEI" \
    BOOTSTRAP_SIGNER_BALANCE_WEI="$BOOTSTRAP_SIGNER_BALANCE_WEI" \
    BOOTSTRAP_FEE_BALANCE_WEI="$BOOTSTRAP_FEE_BALANCE_WEI" \
    node "$SCRIPT_DIR/merge_alloc.js" "$DATA_DIR/sys_contracts.json" "$FUNDER_ADDR"
)
if [ $? -ne 0 ]; then
    echo "❌ Failed to merge alloc"
    exit 1
fi

VANITY="0000000000000000000000000000000000000000000000000000000000000000"
VALIDATORS_HEX=""
for addr in "${SIGNER_ADDRS[@]}"; do
    # Remove 0x prefix
    VALIDATORS_HEX+="${addr:2}"
done
SUFFIX="0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" # 65 bytes zeros

EXTRA_DATA="0x${VANITY}${VALIDATORS_HEX}${SUFFIX}"

# Replace placeholders
echo "$ALLOC_JSON" > "$DATA_DIR/alloc.json"

if [ "$BLACKLIST_ENABLED" = true ] && [ "$BLACKLIST_MODE" = "mock" ] && [ "$BLACKLIST_MOCK_PREDEPLOY" = true ]; then
    echo "Injecting blacklist mock alloc: addr=$BLACKLIST_CONTRACT_ADDR code=$BLACKLIST_MOCK_CODE_FILE"
    node "$BLACKLIST_ALLOC_SCRIPT" "$DATA_DIR/alloc.json" "$BLACKLIST_CONTRACT_ADDR" "$BLACKLIST_MOCK_CODE_FILE" > "$DATA_DIR/alloc.with_blacklist.json"
    mv "$DATA_DIR/alloc.with_blacklist.json" "$DATA_DIR/alloc.json"
fi

jq --slurpfile allocList "$DATA_DIR/alloc.json" \
   --arg extra "$EXTRA_DATA" \
   --arg epoch "$NETWORK_EPOCH" \
   --argjson initialValidators "$BOOTSTRAP_VALIDATORS_JSON" \
   --argjson initialSigners "$BOOTSTRAP_SIGNERS_JSON" \
   '.alloc = $allocList[0]
    | .extraData = $extra
    | .config.congress.epoch = ($epoch | tonumber)
    | .config.congress.initialValidators = $initialValidators
    | .config.congress.initialSigners = $initialSigners' \
   "$TEMPLATE_GENESIS" > "$DATA_DIR/genesis.json"

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
FORK_RUNTIME_SHANGHAI_TIME="$FORK_SHANGHAI_TIME"
FORK_RUNTIME_CANCUN_TIME="$FORK_CANCUN_TIME"
FORK_RUNTIME_FIX_HEADER_TIME="$FORK_FIX_HEADER_TIME"
FORK_RUNTIME_POSA_TIME="$FORK_POSA_TIME"
FORK_RUNTIME_SCHEDULED_TIME="$FORK_SCHEDULED_TIME"
if [ -n "$UPGRADE_OVERRIDE_POSA_TIME" ]; then
    FORK_RUNTIME_POSA_TIME="$UPGRADE_OVERRIDE_POSA_TIME"
    case "$FORK_EFFECTIVE_TARGET" in
        posaTime|poa_shanghai_cancun_fixheader_posa)
            FORK_RUNTIME_SCHEDULED_TIME="$UPGRADE_OVERRIDE_POSA_TIME"
            ;;
    esac
fi

python3 - "$DATA_DIR/genesis.json" "$BOOTSTRAP_SIGNERS_JSON" "$BOOTSTRAP_VALIDATORS_JSON" <<'PY'
import json
import sys

genesis_path, expected_signers_json, expected_validators_json = sys.argv[1:]
expected_signers = json.loads(expected_signers_json)
expected_validators = json.loads(expected_validators_json)

with open(genesis_path, "r", encoding="utf-8") as fh:
    genesis = json.load(fh)

congress = genesis.get("config", {}).get("congress", {})
actual_validators = congress.get("initialValidators", [])
actual_signers = congress.get("initialSigners", [])
if actual_validators != expected_validators:
    raise SystemExit(f"genesis initialValidators mismatch: {actual_validators} != {expected_validators}")
if actual_signers != expected_signers:
    raise SystemExit(f"genesis initialSigners mismatch: {actual_signers} != {expected_signers}")

extra = genesis.get("extraData", "")
if not isinstance(extra, str) or not extra.startswith("0x"):
    raise SystemExit("genesis extraData is missing or invalid")
hexdata = extra[2:]
if len(hexdata) < (64 + 130):
    raise SystemExit("genesis extraData too short for congress signers")
signer_hex = hexdata[64:-130]
actual_extra_signers = [f"0x{signer_hex[i:i+40]}" for i in range(0, len(signer_hex), 40) if signer_hex[i:i+40]]
if sorted(addr.lower() for addr in actual_extra_signers) != sorted(addr.lower() for addr in expected_signers):
    raise SystemExit(f"genesis extraData signer set mismatch: {actual_extra_signers} != {expected_signers}")
PY

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
    signer_address: "${SIGNER_ADDRS[$i]}"
    signer_private_key: "${SIGNER_PRIVS[$i]}"
    fee_address: "${FEE_ADDRS[$i]}"
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
  scheduled_time: $FORK_RUNTIME_SCHEDULED_TIME
  delay_seconds: $FORK_EFFECTIVE_DELAY_SECONDS
  schedule:
    shanghai_time: $FORK_RUNTIME_SHANGHAI_TIME
    cancun_time: $FORK_RUNTIME_CANCUN_TIME
    fix_header_time: $FORK_RUNTIME_FIX_HEADER_TIME
    posa_time: $FORK_RUNTIME_POSA_TIME
  override:
    posa_time: ${UPGRADE_OVERRIDE_POSA_TIME:-}
    posa_validators: $UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON
    posa_signers: $UPGRADE_OVERRIDE_POSA_SIGNERS_JSON

blacklist:
  enabled: $BLACKLIST_ENABLED
  mode: "$BLACKLIST_MODE"
  contract_address: "$BLACKLIST_CONTRACT_ADDR"
  alert_fail_open: $BLACKLIST_ALERT_FAIL_OPEN
  mock:
    predeploy: $BLACKLIST_MOCK_PREDEPLOY
    code_file: "$BLACKLIST_MOCK_CODE_FILE"
    abi_file: "$BLACKLIST_MOCK_ABI_FILE"

runtime:
  backend: "$(cfg_get "$CONFIG_FILE" "runtime.backend" "native")"
  impl_mode: "$RUNTIME_IMPL_MODE"
  impl: "$DEFAULT_RUNTIME_IMPL"

validator_auth:
  mode: "$VALIDATOR_AUTH_MODE"
  keystore:
    password_env: "$VALIDATOR_KEYSTORE_PASSWORD_ENV"
    password_file: "$VALIDATOR_KEYSTORE_PASSWORD_FILE_CFG"

runtime_nodes:
EOF

for i in $(seq 0 $((NUM_NODES-1))); do
    role="sync"
    if [ "$i" -lt "$NUM_VALIDATORS" ]; then
        role="validator"
    fi
    cat >> "$DATA_DIR/test_config.yaml" <<EOF
  - name: "node$i"
    role: "$role"
    impl: "${RUNTIME_NODE_IMPLS[$i]}"
EOF
    if [ "$i" -lt "$NUM_VALIDATORS" ]; then
        cat >> "$DATA_DIR/test_config.yaml" <<EOF
    validator_key: "$DATA_DIR/node$i/validator.key"
    validator_address: "${VAL_ADDRS[$i]}"
    signer_key: "$DATA_DIR/node$i/signer.key"
    signer_address: "${SIGNER_ADDRS[$i]}"
    fee_address: "${FEE_ADDRS[$i]}"
EOF
    fi
done

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

awk \
  -v enabled="$BLACKLIST_ENABLED" \
  -v addr="$BLACKLIST_CONTRACT_ADDR" \
  -v mode="$BLACKLIST_MODE" \
  -v alert="$BLACKLIST_ALERT_FAIL_OPEN" \
  -v override_posa_time="${UPGRADE_OVERRIDE_POSA_TIME:-}" \
  -v override_posa_validators="$UPGRADE_OVERRIDE_POSA_VALIDATORS_CSV" \
  -v override_posa_signers="$UPGRADE_OVERRIDE_POSA_SIGNERS_CSV" '
  /^  node[0-3]:$/ {
    in_node = 1
  }
  in_node && /^  [A-Za-z0-9_-]+:$/ && $0 !~ /^  node[0-3]:$/ {
    in_node = 0
  }
  { print }
  in_node && /^    environment:/ {
    print "      - BLACKLIST_ENABLED=" enabled
    print "      - BLACKLIST_CONTRACT_ADDR=" addr
    print "      - BLACKLIST_MODE=" mode
    print "      - BLACKLIST_ALERT_FAIL_OPEN=" alert
    print "      - UPGRADE_OVERRIDE_POSA_TIME=" override_posa_time
    print "      - UPGRADE_OVERRIDE_POSA_VALIDATORS=" override_posa_validators
    print "      - UPGRADE_OVERRIDE_POSA_SIGNERS=" override_posa_signers
  }
' "$DATA_DIR/docker-compose.runtime.yml" > "$DATA_DIR/docker-compose.runtime.with_blacklist.yml"
mv "$DATA_DIR/docker-compose.runtime.with_blacklist.yml" "$DATA_DIR/docker-compose.runtime.yml"

SESSION_ID="$(date +%Y%m%d_%H%M%S)_$RANDOM"
SESSION_CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
SESSION_TOPOLOGY="$TOPOLOGY_MODE"
if [ -z "$SESSION_TOPOLOGY" ]; then
    SESSION_TOPOLOGY="multi"
fi

mkdir -p "$(dirname "$RUNTIME_SESSION_FILE")"
SESSION_JSON_FILE="${RUNTIME_SESSION_FILE%.yaml}.json"
if [ "$SESSION_JSON_FILE" = "$RUNTIME_SESSION_FILE" ]; then
    SESSION_JSON_FILE="${RUNTIME_SESSION_FILE}.json"
fi

cat > "$RUNTIME_SESSION_FILE" <<EOF
session:
  id: "$SESSION_ID"
  created_at: "$SESSION_CREATED_AT"
  source_config: "$CONFIG_FILE"

runtime:
  backend: "$SESSION_BACKEND"
  topology: "$SESSION_TOPOLOGY"
  impl_mode: "$RUNTIME_IMPL_MODE"
  impl: "$DEFAULT_RUNTIME_IMPL"

network:
  node_count: $NUM_NODES
  validator_count: $NUM_VALIDATORS
  epoch: $NETWORK_EPOCH
  external_rpc: "$PRIMARY_RPC"
  data_dir: "$DATA_DIR"
  reports_dir: "$REPORTS_DIR"
  genesis_mode: "$GENESIS_MODE"
  fork_target: "$FORK_EFFECTIVE_TARGET"
  fork_delay_seconds: $FORK_EFFECTIVE_DELAY_SECONDS

bootstrap:
  signer_mode: "$BOOTSTRAP_SIGNER_MODE"
  fee_mode: "$BOOTSTRAP_FEE_MODE"
  validator_balance_wei: "$BOOTSTRAP_VALIDATOR_BALANCE_WEI"
  signer_balance_wei: "$BOOTSTRAP_SIGNER_BALANCE_WEI"
  fee_balance_wei: "$BOOTSTRAP_FEE_BALANCE_WEI"
  validators: $BOOTSTRAP_VALIDATORS_JSON
  signers: $BOOTSTRAP_SIGNERS_JSON
  fee_addresses: $BOOTSTRAP_FEE_ADDRS_JSON

fork:
  mode: "$GENESIS_MODE"
  target: "$FORK_EFFECTIVE_TARGET"
  scheduled_time: $FORK_RUNTIME_SCHEDULED_TIME
  delay_seconds: $FORK_EFFECTIVE_DELAY_SECONDS
  schedule:
    shanghai_time: $FORK_RUNTIME_SHANGHAI_TIME
    cancun_time: $FORK_RUNTIME_CANCUN_TIME
    fix_header_time: $FORK_RUNTIME_FIX_HEADER_TIME
    posa_time: $FORK_RUNTIME_POSA_TIME
  override:
    posa_time: ${UPGRADE_OVERRIDE_POSA_TIME:-}
    posa_validators: $UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON
    posa_signers: $UPGRADE_OVERRIDE_POSA_SIGNERS_JSON

paths:
  chain_root: "$CHAIN_ROOT"
  reth_root: "$RETH_ROOT"
  chain_contract_root: "$CHAIN_CONTRACT_ROOT"
  chain_contract_out: "$CONTRACT_OUT_DIR"
  reth_bytecode_file: "$RETH_BYTECODE_FILE"

binaries:
  geth_native: "$(to_abs_path "$GETH_NATIVE_BIN_CFG")"
  reth_native: "$(to_abs_path "$RETH_NATIVE_BIN_CFG")"
  geth_docker: "$(to_abs_path "$GETH_DOCKER_BIN_CFG")"
  reth_docker: "$(to_abs_path "$RETH_DOCKER_BIN_CFG")"

runtime_nodes:
EOF

for i in $(seq 0 $((NUM_NODES-1))); do
cat >> "$RUNTIME_SESSION_FILE" <<EOF
  node$i: "${RUNTIME_NODE_IMPLS[$i]}"
EOF
done

cat >> "$RUNTIME_SESSION_FILE" <<EOF

validator_auth:
  mode: "$VALIDATOR_AUTH_MODE"
  keystore:
    password_env: "$VALIDATOR_KEYSTORE_PASSWORD_ENV"
    password_file: "$VALIDATOR_KEYSTORE_PASSWORD_FILE_CFG"

native:
  manager: "$(cfg_get "$CONFIG_FILE" "native.manager" "pm2")"
  init_script: "$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.init_script" "./scripts/native/pm2_init.sh")")"
  ecosystem_file: "$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.ecosystem_file" "./native/ecosystem.config.js")")"
  env_file: "$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.env_file" "./data/native/.env")")"
  pm2_namespace: "$(cfg_get "$CONFIG_FILE" "native.pm2_namespace" "ju-chain")"
  external_rpc: "$(cfg_get "$CONFIG_FILE" "native.external_rpc" "$PRIMARY_RPC")"

docker:
  compose_file: "$DOCKER_COMPOSE_FILE"
  runtime_compose_file: "$DOCKER_RUNTIME_COMPOSE_FILE"
  project_name: "$(cfg_get "$CONFIG_FILE" "docker.project_name" "juchain-it")"
  external_rpc: "$(cfg_get "$CONFIG_FILE" "docker.external_rpc" "$PRIMARY_RPC")"

artifacts:
  genesis_file: "$DATA_DIR/genesis.json"
  test_config_file: "$DATA_DIR/test_config.yaml"
  runtime_nodes_file: "$DATA_DIR/runtime_nodes.yaml"
  runtime_session_json: "$SESSION_JSON_FILE"
EOF

RUNTIME_NODES_JSON='{}'
for i in $(seq 0 $((NUM_NODES-1))); do
    RUNTIME_NODES_JSON="$(printf '%s' "$RUNTIME_NODES_JSON" | jq -c --arg key "node$i" --arg val "${RUNTIME_NODE_IMPLS[$i]}" '. + {($key): $val}')"
done

jq -n \
  --arg session_id "$SESSION_ID" \
  --arg created_at "$SESSION_CREATED_AT" \
  --arg source_config "$CONFIG_FILE" \
  --arg backend "$SESSION_BACKEND" \
  --arg topology "$SESSION_TOPOLOGY" \
  --arg impl_mode "$RUNTIME_IMPL_MODE" \
  --arg impl "$DEFAULT_RUNTIME_IMPL" \
  --arg external_rpc "$PRIMARY_RPC" \
  --arg data_dir "$DATA_DIR" \
  --arg reports_dir "$REPORTS_DIR" \
  --arg genesis_mode "$GENESIS_MODE" \
  --arg fork_target "$FORK_EFFECTIVE_TARGET" \
  --arg bootstrap_signer_mode "$BOOTSTRAP_SIGNER_MODE" \
  --arg bootstrap_fee_mode "$BOOTSTRAP_FEE_MODE" \
  --arg bootstrap_validator_balance "$BOOTSTRAP_VALIDATOR_BALANCE_WEI" \
  --arg bootstrap_signer_balance "$BOOTSTRAP_SIGNER_BALANCE_WEI" \
  --arg bootstrap_fee_balance "$BOOTSTRAP_FEE_BALANCE_WEI" \
  --arg chain_root "$CHAIN_ROOT" \
  --arg reth_root "$RETH_ROOT" \
  --arg chain_contract_root "$CHAIN_CONTRACT_ROOT" \
  --arg chain_contract_out "$CONTRACT_OUT_DIR" \
  --arg reth_bytecode_file "$RETH_BYTECODE_FILE" \
  --arg geth_native "$(to_abs_path "$GETH_NATIVE_BIN_CFG")" \
  --arg reth_native "$(to_abs_path "$RETH_NATIVE_BIN_CFG")" \
  --arg geth_docker "$(to_abs_path "$GETH_DOCKER_BIN_CFG")" \
  --arg reth_docker "$(to_abs_path "$RETH_DOCKER_BIN_CFG")" \
  --arg validator_auth_mode "$VALIDATOR_AUTH_MODE" \
  --arg validator_password_env "$VALIDATOR_KEYSTORE_PASSWORD_ENV" \
  --arg validator_password_file "$VALIDATOR_KEYSTORE_PASSWORD_FILE_CFG" \
  --arg native_manager "$(cfg_get "$CONFIG_FILE" "native.manager" "pm2")" \
  --arg native_init_script "$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.init_script" "./scripts/native/pm2_init.sh")")" \
  --arg native_ecosystem_file "$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.ecosystem_file" "./native/ecosystem.config.js")")" \
  --arg native_env_file "$(to_abs_path "$(cfg_get "$CONFIG_FILE" "native.env_file" "./data/native/.env")")" \
  --arg native_pm2_namespace "$(cfg_get "$CONFIG_FILE" "native.pm2_namespace" "ju-chain")" \
  --arg native_external_rpc "$(cfg_get "$CONFIG_FILE" "native.external_rpc" "$PRIMARY_RPC")" \
  --arg docker_compose_file "$DOCKER_COMPOSE_FILE" \
  --arg docker_runtime_compose_file "$DOCKER_RUNTIME_COMPOSE_FILE" \
  --arg docker_project_name "$(cfg_get "$CONFIG_FILE" "docker.project_name" "juchain-it")" \
  --arg docker_external_rpc "$(cfg_get "$CONFIG_FILE" "docker.external_rpc" "$PRIMARY_RPC")" \
  --arg genesis_file "$DATA_DIR/genesis.json" \
  --arg test_config_file "$DATA_DIR/test_config.yaml" \
  --arg runtime_nodes_file "$DATA_DIR/runtime_nodes.yaml" \
  --arg runtime_session_json "$SESSION_JSON_FILE" \
  --argjson node_count "$NUM_NODES" \
  --argjson validator_count "$NUM_VALIDATORS" \
  --argjson epoch "$NETWORK_EPOCH" \
  --argjson fork_scheduled_time "$FORK_RUNTIME_SCHEDULED_TIME" \
  --argjson fork_delay_seconds "$FORK_EFFECTIVE_DELAY_SECONDS" \
  --argjson shanghai_time "$FORK_RUNTIME_SHANGHAI_TIME" \
  --argjson cancun_time "$FORK_RUNTIME_CANCUN_TIME" \
  --argjson fix_header_time "$FORK_RUNTIME_FIX_HEADER_TIME" \
  --argjson posa_time "$FORK_RUNTIME_POSA_TIME" \
  --arg override_posa_time "${UPGRADE_OVERRIDE_POSA_TIME:-}" \
  --argjson bootstrap_validators "$BOOTSTRAP_VALIDATORS_JSON" \
  --argjson bootstrap_signers "$BOOTSTRAP_SIGNERS_JSON" \
  --argjson bootstrap_fee_addresses "$BOOTSTRAP_FEE_ADDRS_JSON" \
  --argjson override_posa_validators "$UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON" \
  --argjson override_posa_signers "$UPGRADE_OVERRIDE_POSA_SIGNERS_JSON" \
  --argjson runtime_nodes "$RUNTIME_NODES_JSON" \
  '{
    session: {
      id: $session_id,
      created_at: $created_at,
      source_config: $source_config
    },
    runtime: {
      backend: $backend,
      topology: $topology,
      impl_mode: $impl_mode,
      impl: $impl
    },
    network: {
      node_count: $node_count,
      validator_count: $validator_count,
      epoch: $epoch,
      external_rpc: $external_rpc,
      data_dir: $data_dir,
      reports_dir: $reports_dir,
      genesis_mode: $genesis_mode,
      fork_target: $fork_target,
      fork_delay_seconds: $fork_delay_seconds
    },
    bootstrap: {
      signer_mode: $bootstrap_signer_mode,
      fee_mode: $bootstrap_fee_mode,
      validator_balance_wei: $bootstrap_validator_balance,
      signer_balance_wei: $bootstrap_signer_balance,
      fee_balance_wei: $bootstrap_fee_balance,
      validators: $bootstrap_validators,
      signers: $bootstrap_signers,
      fee_addresses: $bootstrap_fee_addresses
    },
    fork: {
      mode: $genesis_mode,
      target: $fork_target,
      scheduled_time: $fork_scheduled_time,
      delay_seconds: $fork_delay_seconds,
      schedule: {
        shanghai_time: $shanghai_time,
        cancun_time: $cancun_time,
        fix_header_time: $fix_header_time,
        posa_time: $posa_time
      },
      override: {
        posa_time: (if ($override_posa_time | length) == 0 then null else ($override_posa_time | tonumber) end),
        posa_validators: $override_posa_validators,
        posa_signers: $override_posa_signers
      }
    },
    paths: {
      chain_root: $chain_root,
      reth_root: $reth_root,
      chain_contract_root: $chain_contract_root,
      chain_contract_out: $chain_contract_out,
      reth_bytecode_file: $reth_bytecode_file
    },
    binaries: {
      geth_native: $geth_native,
      reth_native: $reth_native,
      geth_docker: $geth_docker,
      reth_docker: $reth_docker
    },
    runtime_nodes: $runtime_nodes,
    validator_auth: {
      mode: $validator_auth_mode,
      keystore: {
        password_env: $validator_password_env,
        password_file: $validator_password_file
      }
    },
    native: {
      manager: $native_manager,
      init_script: $native_init_script,
      ecosystem_file: $native_ecosystem_file,
      env_file: $native_env_file,
      pm2_namespace: $native_pm2_namespace,
      external_rpc: $native_external_rpc
    },
    docker: {
      compose_file: $docker_compose_file,
      runtime_compose_file: $docker_runtime_compose_file,
      project_name: $docker_project_name,
      external_rpc: $docker_external_rpc
    },
    artifacts: {
      genesis_file: $genesis_file,
      test_config_file: $test_config_file,
      runtime_nodes_file: $runtime_nodes_file,
      runtime_session_json: $runtime_session_json
    }
  }' > "$SESSION_JSON_FILE"

echo "ℹ️  Configuration generated at $DATA_DIR"
echo "ℹ️  Runtime session snapshot: $RUNTIME_SESSION_FILE"
