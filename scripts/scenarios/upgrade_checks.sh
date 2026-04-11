#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/network/lib.sh
source "$SCRIPT_DIR/../network/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"
SESSION_FILE="$(resolve_runtime_session_file "${RUNTIME_SESSION_FILE:-}")"

gen_addr() {
  local seed="$1"
  (
    cd "$ROOT_DIR"
    go run ./cmd/genkeys "$seed"
  ) | awk -F',' '{print $1}' | tr -d '[:space:]'
}

hardhat_addr() {
  local index="$1"
  (
    cd "$ROOT_DIR"
    go run ./cmd/genhardhat "$index"
  ) | awk -F',' '{print $1}' | tr -d '[:space:]'
}

cleanup() {
  if [[ -f "$SESSION_FILE" ]]; then
    bash "$ROOT_DIR/scripts/network/native.sh" down "$SESSION_FILE" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

override_validator="$(gen_addr "upgrade-validator-0")"
# For PoA -> PoSA migration, override.posaSigners must cover the live POA signer set.
# In the default single-node separated bootstrap layout, the live PoA signer is Hardhat index 4.
runtime_signer="$(hardhat_addr 4)"
override_time="$(( $(date +%s) + 45 ))"

echo "[scenario/upgrade] generate single-node upgrade config with CLI override"
TEST_ENV_CONFIG="$CONFIG_FILE" \
TOPOLOGY=single \
BOOTSTRAP_SIGNER_MODE=separate \
GENESIS_MODE=upgrade \
FORK_TARGET=posaTime \
FORK_DELAY_SECONDS=30 \
UPGRADE_OVERRIDE_POSA_TIME="$override_time" \
UPGRADE_OVERRIDE_POSA_VALIDATORS="$override_validator" \
UPGRADE_OVERRIDE_POSA_SIGNERS="$runtime_signer" \
bash "$ROOT_DIR/scripts/gen_network_config.sh" >/dev/null

python3 - "$ROOT_DIR/data/test_config.yaml" "$ROOT_DIR/data/runtime_session.yaml" "$override_time" "$override_validator" "$runtime_signer" <<'PY'
import sys

import yaml

cfg_path, session_path, override_time, override_validator, runtime_signer = sys.argv[1:]

with open(cfg_path, "r", encoding="utf-8") as fh:
    cfg = yaml.safe_load(fh) or {}
with open(session_path, "r", encoding="utf-8") as fh:
    session = yaml.safe_load(fh) or {}

for doc_name, doc in (("test_config", cfg), ("runtime_session", session)):
    fork = (doc.get("fork") or {})
    override = (fork.get("override") or {})
    if int(override.get("posa_time") or 0) != int(override_time):
        raise SystemExit(f"{doc_name} override.posa_time mismatch: {override.get('posa_time')} != {override_time}")
    if (override.get("posa_validators") or []) != [override_validator]:
        raise SystemExit(f"{doc_name} override.posa_validators mismatch")
    if (override.get("posa_signers") or []) != [runtime_signer]:
        raise SystemExit(f"{doc_name} override.posa_signers mismatch")
    if int((fork.get("schedule") or {}).get("posa_time") or 0) != int(override_time):
        raise SystemExit(f"{doc_name} effective posa_time mismatch")
    if int(fork.get("scheduled_time") or 0) != int(override_time):
        raise SystemExit(f"{doc_name} effective scheduled_time mismatch")
PY

echo "[scenario/upgrade] init/start native single node through main dispatcher"
bash "$ROOT_DIR/scripts/native/pm2_init.sh" "$SESSION_FILE"

ENV_FILE="$(cfg_get "$SESSION_FILE" "native.env_file" "$ROOT_DIR/data/native/.env")"
if ! grep -q "^UPGRADE_OVERRIDE_POSA_VALIDATORS=$override_validator$" "$ENV_FILE"; then
  die "native env missing override validator"
fi
if ! grep -q "^UPGRADE_OVERRIDE_POSA_SIGNERS=$runtime_signer$" "$ENV_FILE"; then
  die "native env missing override signer"
fi
if ! grep -q "^UPGRADE_OVERRIDE_POSA_TIME=$override_time$" "$ENV_FILE"; then
  die "native env missing override posa time"
fi

runtime_impl="$(cfg_get "$SESSION_FILE" "runtime.impl" "")"
if [[ "$runtime_impl" == "reth" ]]; then
  reth_chain_file="$(cfg_get "$SESSION_FILE" "artifacts.reth_chain_file" "$ROOT_DIR/data/reth_chain.json")"
  shanghai_time="$(cfg_get "$SESSION_FILE" "fork.schedule.shanghai_time" "0")"
  cancun_time="$(cfg_get "$SESSION_FILE" "fork.schedule.cancun_time" "0")"
  fix_header_time="$(cfg_get "$SESSION_FILE" "fork.schedule.fix_header_time" "0")"
  posa_time="$(cfg_get "$SESSION_FILE" "fork.schedule.posa_time" "0")"
  python3 - "$reth_chain_file" "$override_time" "$override_validator" "$runtime_signer" "$shanghai_time" "$cancun_time" "$fix_header_time" "$posa_time" <<'PY'
import json
import sys

(
    chain_file,
    override_time,
    override_validator,
    runtime_signer,
    shanghai_time,
    cancun_time,
    fix_header_time,
    posa_time,
) = sys.argv[1:]
with open(chain_file, "r", encoding="utf-8") as fh:
    genesis = json.load(fh)
config = genesis.get("config") or {}
congress = config.get("congress") or {}
expected_schedule = {
    "shanghaiTime": int(shanghai_time),
    "cancunTime": int(cancun_time),
    "fixHeaderTime": int(fix_header_time),
    "posaTime": int(posa_time),
}
for field, expected in expected_schedule.items():
    actual = int(config.get(field) or 0)
    if actual != expected:
        raise SystemExit(f"reth chain {field} mismatch: {actual} != {expected}")
if int(config.get("posaTime") or 0) != int(override_time):
    raise SystemExit(f"reth chain posaTime mismatch: {config.get('posaTime')} != {override_time}")
if (congress.get("initialValidators") or []) != [override_validator]:
    raise SystemExit("reth chain congress.initialValidators mismatch")
if (congress.get("initialSigners") or []) != [runtime_signer]:
    raise SystemExit("reth chain congress.initialSigners mismatch")
PY
fi

bash "$ROOT_DIR/scripts/network/native.sh" init "$SESSION_FILE"
bash "$ROOT_DIR/scripts/network/native.sh" up "$SESSION_FILE"
bash "$ROOT_DIR/scripts/network/native.sh" ready "$SESSION_FILE"

echo "[scenario/upgrade] verify migration mapping after fork"
(
  cd "$ROOT_DIR"
  go test ./tests/epoch -run TestZ_UpgradeOverrideBootstrapMapping -count=1
)

echo "[scenario/upgrade] 🟢 PASS"
