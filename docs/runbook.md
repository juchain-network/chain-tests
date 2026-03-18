# chain-tests Runbook

## 1. Scope
This runbook covers local and CI operations for `chain-tests` regression, fork matrix, and perf/soak workflows.

## 2. Prerequisites
- Compiled geth binary under `<chain_root>/build/bin/geth`
- Compiled contract artifacts under `<chain_contract_root>/out`
- `go`, `node`, `python3`, `jq`, `pm2` (native mode), `docker` (docker mode)

## 3. Environment configuration
1. Copy config template:
   - `cp /Users/litian/code/work/github/chain-tests/config/test_env.yaml.example /Users/litian/code/work/github/chain-tests/config/test_env.yaml`
2. Set path roots:
   - `paths.chain_root`
   - `paths.chain_contract_root`
3. Select backend:
   - `runtime.backend: native` for fast local loop
   - `runtime.backend: docker` for CI parity

## 4. Core operations
- Start clean network:
  - `make reset`
- Stop network:
  - `make stop`
- Runtime status:
  - `make status`
- Runtime logs:
  - `make logs`

## 5. Regression commands
- Smoke:
  - `make test-smoke`
  - `TOPOLOGY=all MATRIX=1 make test-smoke`
- Business groups:
  - `make test-group GROUP=all`
  - `make ci MODE=groups GROUPS=config,governance,staking`
- Fork matrix (single + multi):
  - `TOPOLOGY=all make test-fork`
- PoSA deep scenarios:
  - `make test-scenario SCENARIO=posa`
- Full orchestrated regression:
  - `make test-regression SCOPE=full`

## 6. Performance and soak
- TPS profile:
  - `make test-perf MODE=tiers`
- 24h soak:
  - `make test-perf MODE=soak`

Expected artifacts:
- `summary.md`
- `metrics.csv`
- `verdict.json`
- `verdict.json.failed_reasons`
- `verdict.json.top_slow_windows`
- `verdict.json.resource_peaks`

## 7. CI profiles
- PR gate:
  - `make ci PROFILE=pr`
- Nightly full:
  - `make ci PROFILE=nightly`
- Weekly soak:
  - `make ci PROFILE=weekly-soak`
- Release gate:
  - `make ci PROFILE=release`

## 8. Troubleshooting
### 8.1 Bytecode mismatch
- Run in upstream contract repo: `forge build`
- Rebuild geth in chain repo
- Run: `make clean && make init`

### 8.2 Fork schedule errors
- Regenerate genesis via `make init`
- Avoid manual edits to `data/genesis.json`

### 8.3 Node lag or stall
- Check `reports/*/report.md` slow cases and group duration tables
- Verify runtime logs and peer status
- Restart network: `make net-reset`

## 9. Rollback
1. Pin previous geth binary and contract artifact versions.
2. Reset local data:
   - `make clean`
3. Re-init and rerun smoke:
   - `make reset && make test-smoke`
