# chain-tests Runbook

## 1. Scope
This runbook covers:
- local bootstrap identities used by the default test environment
- base network lifecycle commands for single-node and multi-node `poa` / `posa` environments
- the main regression, performance, CI, and troubleshooting entrypoints

It does not try to document every fork, migration, or scenario-specific override.

## 2. Prerequisites
- Compiled geth binary under `<chain_root>/build/bin/geth`
- Compiled contract artifacts under `<chain_contract_root>/out`
- `go`, `node`, `python3`, `jq`
- `pm2` for native runtime

## 3. Environment configuration
1. Copy the config template:
   - `cp config/test_env.yaml.example config/test_env.yaml`
2. Set path roots in `config/test_env.yaml`:
   - `paths.chain_root`
   - `paths.chain_contract_root`
3. Keep `runtime.backend: native`

Current default facts in `config/test_env.yaml`:
- `runtime.backend: native`
- `network.node_count: 4`
- `network.validator_count: 3`
- `network.epoch: 30`
- `network.bootstrap.signer_mode: separate`
- `network.bootstrap.fee_mode: validator`

Topology notes:
- `TOPOLOGY=single` runs on native
- `TOPOLOGY=multi` runs on native
- If `TOPOLOGY` is omitted, the generator resolves topology from `network.node_count` and `network.validator_count`

## 4. Default bootstrap identities
Bootstrap identities are pinned to the Hardhat default mnemonic and fixed index mapping used by `scripts/gen_network_config.sh`:
- funder: index `0`
- validator cold addresses: index `1..3`
- separate signer hot addresses: index `4..6`

The current default mode is `signer_mode=separate` and `fee_mode=validator`.
If you switch to `same_address`, each validator's signer address/key becomes the same as its validator cold address/key.

### 4.1 Shared funder

| Role | Hardhat index | Address | Private key |
| --- | ---: | --- | --- |
| Funder | 0 | `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266` | `ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80` |

### 4.2 Single-node default mapping
`TOPOLOGY=single` uses the first validator/signer pair only.

| Role | Hardhat index | Address | Private key | Notes |
| --- | ---: | --- | --- | --- |
| Validator 1 (cold) | 1 | `0x70997970C51812dc3A010C7d01b50e0d17dc79C8` | `59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d` | Used for validator admin transactions |
| Signer 1 (hot) | 4 | `0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65` | `47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a` | Runtime block producer key |
| Fee receiver | 1 | `0x70997970C51812dc3A010C7d01b50e0d17dc79C8` | `59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d` | Same as validator because `fee_mode=validator` |

### 4.3 Multi-node default mapping
The default multi-node environment is `3 validators + 1 sync node`.
The sync node has no separate validator/signer business identity. Fee receivers still default to the matching validator cold addresses because `fee_mode=validator`.

| Role | Hardhat index | Address | Private key |
| --- | ---: | --- | --- |
| Validator 1 (cold) | 1 | `0x70997970C51812dc3A010C7d01b50e0d17dc79C8` | `59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d` |
| Signer 1 (hot) | 4 | `0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65` | `47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a` |
| Validator 2 (cold) | 2 | `0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC` | `5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a` |
| Signer 2 (hot) | 5 | `0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc` | `8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba` |
| Validator 3 (cold) | 3 | `0x90F79bf6EB2c4f870365E785982E1f101E93b906` | `7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6` |
| Signer 3 (hot) | 6 | `0x976EA74026E726554dB657fA54763abd0C3a0aa9` | `92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e` |

## 5. Base network lifecycle (`poa` / `posa`)
The public lifecycle surface is the `Network Commands` section in `Makefile`:
- `init`
- `run`
- `ready`
- `stop`
- `reset`
- `clean`
- `status`
- `logs`

For the base environment, the main selectors are:
- `TOPOLOGY=single|multi`
- `INIT_MODE=poa|posa`

Other init modes such as `smoke` and `upgrade` exist, but are outside this base runbook section.

### 5.1 What each command does

| Command | Purpose |
| --- | --- |
| `make init` | Generate keys, `genesis.json`, `data/test_config.yaml`, `data/runtime_session.yaml`, and native runtime config |
| `make run` | Start the prepared network and wait for RPC readiness |
| `make ready` | Re-run readiness wait against the current runtime session |
| `make status` | Show native pm2 status |
| `make logs` | Stream runtime logs; for multi-node runs you can pass `NODE=...` |
| `make stop` | Stop the current runtime session |
| `make clean` | Stop the runtime if needed and remove local runtime artifacts under `data/` |
| `make reset` | Equivalent to `clean + init + run + ready` |

Important lifecycle notes:
- `make run` already performs a readiness wait; `make ready` is mainly useful when attaching to an existing session
- `make stop`, `make status`, and `make logs` depend on `data/runtime_session.yaml`; if you see a "runtime session not found" error, run `make init` first
- When switching topology or genesis mode, prefer `make clean` before re-initializing

### 5.2 One-command startup examples

#### Single-node POA
```bash
make reset TOPOLOGY=single INIT_MODE=poa
```

#### Single-node PoSA
```bash
make reset TOPOLOGY=single INIT_MODE=posa
```

#### Multi-node POA
```bash
make reset TOPOLOGY=multi INIT_MODE=poa
```

#### Multi-node PoSA
```bash
make reset TOPOLOGY=multi INIT_MODE=posa
```

### 5.3 Step-by-step lifecycle examples

#### Single-node POA
```bash
make clean
make init TOPOLOGY=single INIT_MODE=poa
make run
make status
make logs
make stop
```

#### Single-node PoSA
```bash
make clean
make init TOPOLOGY=single INIT_MODE=posa
make run
make status
make logs
make stop
```

#### Multi-node POA
```bash
make clean
make init TOPOLOGY=multi INIT_MODE=poa
make run
make status
make logs
make stop
```

#### Multi-node PoSA
```bash
make clean
make init TOPOLOGY=multi INIT_MODE=posa EPOCH=240
make run
make status
make logs
make stop
```

### 5.4 Useful log examples
- Native single-node:
  - `make logs`
- Native multi-node pm2 logs:
  - `NODE=ju-chain-validator1 make logs`
  - `NODE=ju-chain-syncnode make logs`

## 6. Regression commands
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
- All scenarios in one pass:
  - `make test-scenario SCENARIO=all`
- Scenario-only coverage:
  - `make test-scenario SCENARIO=checkpoint`
  - `make test-scenario SCENARIO=rotation-punish`
  - `make test-scenario SCENARIO=add-validator-live`
  - `make test-scenario SCENARIO=add-validator-punish`
  - `make test-scenario SCENARIO=negative`
  - `make test-scenario SCENARIO=upgrade`
- Full orchestrated regression:
  - `make test-regression SCOPE=full`

Notes:
- `make test-regression SCOPE=core` uses the default local environment and may intentionally skip topology/epoch/upgrade-specific `TestZ_*` cases.
- Use the scenario commands above to cover long-epoch, single-validator checkpoint, and upgrade-only paths.

### 6.1 Fork capability runbook (`test-forkcap`)
`make test-forkcap` is the dedicated real-chain fork capability surface.
It is separate from business-group regression and is meant to prove fork-gated EVM / protocol behavior directly.

Current execution boundary:
- the runner always forces a temporary **single-node geth** config
- it runs both `pre` and `post` phases for the selected fork automatically
- it does **not** currently validate `reth` / `rchain` parity or mixed-mode behavior

Supported fork selector:
- `FORK=shanghai|cancun|fixheader|posa|prague|osaka|bpo1|bpo2|all`

Useful commands:
```bash
# run all forkcap suites
make test-forkcap FORK=all

# run one fork layer only
make test-forkcap FORK=shanghai
make test-forkcap FORK=cancun
make test-forkcap FORK=prague
make test-forkcap FORK=osaka

# run a narrow regex against the test names
make test-forkcap FORK=shanghai CASE='TestK_ForkcapCapability_Push0'
make test-forkcap FORK=cancun CASE='TestK_ForkcapCapability_(Mcopy|TransientStorage|CancunHeaderSurface)'
make test-forkcap FORK=osaka CASE='TestK_ForkcapCapability_(OsakaEngineGetPayloadTransition|OsakaEngineBlobAPITransition)'
```

Important `CASE` rule:
- `CASE` is passed to `go test -run` as a regular expression
- use Go test names or regex groups, not registry keys
- for example, prefer `CASE='TestK_ForkcapCapability_Push0'` instead of `CASE=push0_execution`

What the runner does for you:
1. rewrites the active config into a temporary single-node geth variant
2. initializes the selected `pre` fork phase
3. starts the chain and runs `./tests/forkcaps`
4. stops the chain
5. repeats the same flow for the `post` fork phase

Per-fork phase mapping:
- `shanghai`: `poa` -> `poa_shanghai`
- `cancun`: `poa_shanghai` -> `poa_shanghai_cancun`
- `fixheader`: `poa_shanghai_cancun` -> `poa_shanghai_cancun_fixheader`
- `posa`: `poa_shanghai_cancun_fixheader` -> `poa_shanghai_cancun_fixheader_posa`
- `prague`: `poa_shanghai_cancun_fixheader_posa` -> `poa_shanghai_cancun_fixheader_posa_prague`
- `osaka`: `poa_shanghai_cancun_fixheader_posa_prague` -> `poa_shanghai_cancun_fixheader_posa_prague_osaka`
- `bpo1`: `poa_shanghai_cancun_fixheader_posa_prague_osaka` -> `poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1`
- `bpo2`: `poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1` -> `poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2`

Report locations:
- top-level forkcap root: `reports/forkcap_<timestamp>/` unless `REPORT_DIR=...` is provided
- the runner creates per-phase subdirectories under that root: `<root>/<fork>/pre/` and `<root>/<fork>/post/`
- each phase invokes the shared CI runner, which writes the usual artifacts:
  - `report.md`
  - `summary.json`
  - `manifest.json`

How to read results:
- `PASS`: the harness proved the expected pre/post fork behavior on the running chain
- `FAIL`: the claimed capability did not match chain behavior or the harness hit a real setup/runtime error
- `SKIP`: expected in two cases:
  - selection skip: the current test does not belong to the chosen fork selection
  - deferred-by-design capability: the capability stays visible, but current chain policy or missing implementation means the suite does not claim proof yet

How to read active vs deferred surfaces:
- active capabilities are the ones the suite currently proves for real
- deferred capabilities stay visible on purpose and should include a reason
- today, `blob_tx_submission` is the main intentional deferred item because blob tx success-path coverage is blocked by txpool policy
- for the current authoritative list, use the capability matrix in:
  - `docs/fork_capability_test_plan.md`
  - `docs/fork_capability_implementation_blueprint.md`

When to use `test-forkcap` instead of other surfaces:
- use `test-forkcap` when the question is “does this fork-gated capability actually exist on the running chain?”
- use `test-fork` for upgrade/liveness matrix verification
- use `test-group` / `test-regression` for business behavior and contract/congress integration paths

## 7. Performance and soak
- TPS profile:
  - `make test-perf MODE=tiers PERF_TOPOLOGY=single PERF_SCOPE=single`
  - `make test-perf MODE=tiers PERF_TOPOLOGY=multi PERF_SCOPE=multi`
  - keep `PERF_TPS_TIERS` for fixed stair-step probes such as `40,50,60`
- Max TPS exploration:
  - `make test-perf MODE=max PERF_TOPOLOGY=single PERF_SCOPE=single`
  - `make test-perf MODE=max PERF_TOPOLOGY=multi PERF_SCOPE=multi`
  - max mode climbs from `PERF_MAX_BASE_TPS` (default `1000`) by `PERF_MAX_STEP` (default `100`) and runs each step for `PERF_MAX_STEP_DURATION` (default `90s`) until the first unstable step or `PERF_MAX_TARGET_TPS` (default `5000`)
- Sender scaling:
  - perf uses a single ingress RPC node for write traffic in both single-node and multi-node topologies
  - high TPS runs shard transactions across multiple funded sender accounts; override with `PERF_SENDER_ACCOUNTS`, or leave `0` for auto-sizing
- Multi-node health:
  - use `PERF_SCOPE=multi` when you want multi-node height-lag gating and convergence warmup
  - in `PERF_SCOPE=multi`, perf now waits for node convergence before measurement and fails tiers that leave backlog, exceed lag, or stall block growth
- Runtime lifecycle:
  - `test-perf` now rebuilds the requested perf topology before running (`PERF_TOPOLOGY=single|multi`, default `single`)
  - by default `make test-perf` stops the runtime on exit; set `PERF_AUTO_STOP=0` if you want to keep the environment running
- 24h soak:
  - `make test-perf MODE=soak PERF_TOPOLOGY=single PERF_SCOPE=single`

Expected artifacts:
- `summary.md`
- `metrics.csv`
- `verdict.json`
- `verdict.json.failed_reasons`
- `verdict.json.top_slow_windows`
- `verdict.json.resource_peaks`

## 8. Congress runtime Go coverage
Native real-network runs can collect Go coverage for `../chain/consensus/congress/...`.

Supported scope:
- geth only
- single-node or multi-node

Not supported:
- reth or mixed geth/reth runtime
- Solidity source coverage from real-chain execution

Example commands:
```bash
CHAIN_COVERAGE=1 make test-group GROUP=config
CHAIN_COVERAGE=1 CHAIN_COVERAGE_OUT_DIR=reports/coverage_rewards make test-group GROUP=rewards
CHAIN_COVERAGE=1 make ci PROFILE=pr
```

Multi-command session workflow:
```bash
make coverage-start CHAIN_COVERAGE_OUT_DIR=reports/coverage_combo
make test-group GROUP=config
make test-scenario SCENARIO=checkpoint
make coverage-merge
```

Session notes:
- `make coverage-start` creates `reports/.coverage_state/session.env`
- while the session file exists, `make test-group`, `make test-scenario`, `make test-smoke`, `make test-fork`, `make test-regression`, and `make ci` will append raw coverage into the same session
- `make clean` does not remove the session state or the cached coverage geth binary
- `make coverage-merge` merges all accumulated raw data and closes the session
- `make coverage-stop` closes the session without merging
- use `make coverage-status` to inspect the active session

Artifacts:
- `summary.txt`
- `func.txt`
- `package_percent.txt`
- `coverage.out`
- `merged/`
- `raw/` (unless `CHAIN_COVERAGE_KEEP_RAW=0`)

Notes:
- the test command exit code still follows the real test result
- if tests fail but node processes produced `covdata`, the coverage report is still merged and saved
- the coverage build uses a dedicated binary under `reports/.coverage_state/bin/`; it does not replace the normal `<chain_root>/build/bin/geth`

## 9. CI profiles
- PR gate:
  - `make ci PROFILE=pr`
- Nightly full:
  - `make ci PROFILE=nightly`
- Weekly soak:
  - `make ci PROFILE=weekly-soak`
- Release gate:
  - `make ci PROFILE=release`

## 10. Troubleshooting
### 10.1 Bytecode mismatch
- Run in the contract repo: `forge build`
- Rebuild geth in the chain repo
- Re-run from clean state:
  - `make clean && make init`

### 10.2 Fork schedule errors
- Regenerate genesis via `make init`
- Avoid manual edits to `data/genesis.json`

### 10.3 Runtime session missing
- Symptom: `make stop`, `make status`, or `make logs` reports that the runtime session is missing
- Fix:
  - `make init`
  - then `make run`

### 10.4 Node lag or stall
- Check `reports/*/report.md` slow cases and group duration tables
- Verify runtime logs and peer status
- Restart the network session:
  - `make stop && make run`

## 11. Rollback
1. Pin the previous geth binary and contract artifact versions
2. Reset local runtime data:
   - `make clean`
3. Re-init and rerun smoke:
   - `make reset && make test-smoke`
 Node lag or stall
- Check `reports/*/report.md` slow cases and group duration tables
- Verify runtime logs and peer status
- Restart the network session:
  - `make stop && make run`

## 11. Rollback
1. Pin the previous geth binary and contract artifact versions
2. Reset local runtime data:
   - `make clean`
3. Re-init and rerun smoke:
   - `make reset && make test-smoke`
