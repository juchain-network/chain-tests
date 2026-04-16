# chain-tests

`chain-tests` is the integration test project for a custom Congress-based chain.
It validates:

- end-to-end behavior of system contracts on real local chain topologies
- interaction between consensus and system contracts
- critical consensus paths (validator updates, punish, rewards, governance)
- PoA -> PoSA upgrade fork liveness
- native `pm2` runtime
- both runtime implementations: `geth` and `reth`

## 1. Dependency boundary

This repository only consumes **compiled artifacts** from external repos:

- `chain`: geth binary (default: `<chain_root>/build/bin/geth`)
- `reth`: reth binary (default: `<reth_root>/target/release/congress-node`)
- `chain-contract`: compiled contract artifacts (default: `<chain_contract_root>/out`)

Common path configuration:

- `paths.chain_root`
- `paths.reth_root`
- `paths.chain_contract_root`

## 2. Requirements

Recommended local tools:

- Go
- Node.js
- `python3`, `curl`, `jq` (`yq` optional)
- for native runtime: `pm2`

## 3. Quick start

### 3.1 Build external artifacts

Build contracts:

```bash
cd ../chain-contract
forge build
```

Sync the generated Go bindings in this repository from the latest external artifacts:

```bash
make sync-contract-clients
```

Useful overrides:

```bash
make sync-contract-clients CONTRACT_CLIENT_SOURCE_ROOT=../chain-contracts
make sync-contract-clients CONTRACT_CLIENT_BUILD=1
make sync-contract-clients ABIGEN=../chain/build/bin/abigen
```

Build geth:

```bash
cd ../chain
make geth
```

Build reth (optional, for `runtime.impl=reth` or mixed mode):

```bash
cd ../rchain
cargo build -p congress-node --release
```

### 3.2 Initialize config

```bash
cd /Users/litian/code/work/github/chain-tests
make init-config
```

Edit `config/test_env.yaml`:

```yaml
runtime:
  backend: native
  impl_mode: single # single | mixed
  impl: geth # geth | reth

validator_auth:
  mode: auto # auto | keystore

paths:
  chain_root: ../chain-1.16/chain-1.16
  reth_root: ../rchain
  chain_contract_root: ../chain-contract

runtime_nodes:
  node0:
    impl: geth
    binary: ""
  node1:
    impl: geth
    binary: ""
  node2:
    impl: geth
    binary: ""
  node3:
    impl: reth
    binary: /opt/juchain/bin/congress-node
```

Path fields support absolute paths and relative paths. Relative paths are resolved
from the repository root (`chain-tests/`), not from `config/`.

Native binary selection order:

1. `runtime_nodes.nodeX.binary`
2. `binaries.geth_native` / `binaries.reth_native`
3. derived default path from `paths.chain_root` / `paths.reth_root`

Notes:

- `runtime.impl_mode=single`: all nodes use `runtime.impl`; `runtime_nodes.*.binary` may still override a specific node's native binary.
- `runtime.impl_mode=mixed`: every `runtime_nodes.nodeX.impl` must be set explicitly.
- This native per-node binary override is intended for mixed geth/reth or multi-version geth groupings.

### 3.3 Start network

```bash
make reset
make status
```

Initialization supports runtime object parameters (init-only):

```bash
TOPOLOGY=single INIT_MODE=poa make init
TOPOLOGY=multi INIT_MODE=smoke INIT_TARGET=poa_shanghai_cancun make init
TOPOLOGY=multi INIT_MODE=upgrade INIT_TARGET=cancunTime INIT_DELAY_SECONDS=60 make init
```

Notes:

- `TOPOLOGY/INIT_MODE/INIT_TARGET/INIT_DELAY_SECONDS` only affect `make init`.
- `TOPOLOGY`: `single | multi`
- `INIT_MODE`: `poa | posa | smoke | upgrade`
- `INIT_TARGET`:
  - when `INIT_MODE=smoke`: `poa | poa_shanghai | poa_shanghai_cancun | poa_shanghai_cancun_fixheader | poa_shanghai_cancun_fixheader_posa | poa_shanghai_cancun_fixheader_posa_prague | poa_shanghai_cancun_fixheader_posa_prague_osaka`
  - when `INIT_MODE=upgrade`: `shanghaiTime | cancunTime | fixHeaderTime | posaTime | pragueTime | osakaTime | allStaggered | allSame`
- After `make init`, lifecycle commands operate on the generated runtime session snapshot:
  - default snapshot path: `data/runtime_session.yaml`
  - machine-readable copy: `data/runtime_session.json`
- If no runtime session exists, lifecycle commands (`run/stop/status/logs/net-*`) fail and require `make init` first.

### 3.4 Run tests

```bash
make test-regression SCOPE=core
```

## 4. Runtime model

The project is native-only. Network lifecycle is managed through `pm2`.

Public lifecycle commands:

```bash
make init
make run
make ready
make stop
make reset
make status
make logs
```

Lifecycle commands are bound to the latest initialized runtime session object; editing
`config/test_env.yaml` after `make init` does not change the active object until next `make init`.

Override rule: if a Make variable is not explicitly passed, commands use `config/test_env.yaml`.
CLI variables act only as temporary overrides for that invocation.

## 5. Common test commands

### 5.1 Smoke (standalone)

```bash
make test-smoke
TOPOLOGY=single make test-smoke
TOPOLOGY=multi MATRIX=1 make test-smoke
TOPOLOGY=all MATRIX=1 make test-smoke
```

`test-smoke` single-node mode key variables:

- `SMOKE_SINGLE_IMPL`: optional override `geth | reth` (empty -> use `config/test_env.yaml` runtime settings)
- `SMOKE_SINGLE_AUTH_MODE`: optional override `auto | keystore` (empty -> use `config/test_env.yaml` validator_auth.mode)
- `SMOKE_SINGLE_GENESIS_MODE`: optional `poa | posa | smoke | upgrade`
- `SMOKE_SINGLE_FORK_TARGET`: required when `SMOKE_SINGLE_GENESIS_MODE=smoke|upgrade`
- `SMOKE_SINGLE_OBSERVE_SECONDS`: optional liveness observe window override (empty -> use `tests.smoke.observe_seconds` from config)
- `SMOKE_SINGLE_TEST_TIMEOUT`: test timeout (default `12m`)

Examples:

```bash
# single-node smoke using config defaults (runtime/backend/auth from test_env.yaml)
TOPOLOGY=single make test-smoke

# single-node smoke on reth
TOPOLOGY=single SMOKE_SINGLE_IMPL=reth make test-smoke

# single-node smoke on reth + keystore auth
TOPOLOGY=single SMOKE_SINGLE_IMPL=reth SMOKE_SINGLE_AUTH_MODE=keystore make test-smoke

# single-node static-fork genesis smoke
TOPOLOGY=single SMOKE_SINGLE_GENESIS_MODE=smoke SMOKE_SINGLE_FORK_TARGET=poa_shanghai_cancun make test-smoke
```

Smoke matrix is **static genesis fork-profile liveness** (no runtime upgrade scheduling).
Default `SMOKE_CASES`:
- `poa`
- `poa_shanghai`
- `poa_shanghai_cancun`
- `poa_shanghai_cancun_fixheader`
- `poa_shanghai_cancun_fixheader_posa`
- `poa_shanghai_cancun_fixheader_posa_prague`
- `poa_shanghai_cancun_fixheader_posa_prague_osaka`

Notes:

- Smoke/fork matrices resolve runtime support from `runtime_capability.version_matrix` and skip cases whose required fork is above the weakest node in the selected topology.

Matrix examples:

```bash
# multi topology with selected static-fork cases
SMOKE_CASES=poa,poa_shanghai_cancun TOPOLOGY=multi MATRIX=1 make test-smoke

# single topology static-fork matrix
SMOKE_CASES=poa,poa_shanghai,poa_shanghai_cancun TOPOLOGY=single MATRIX=1 make test-smoke

# single + multi matrix with explicit report directory
SMOKE_REPORT_DIR=reports/smoke_matrix_custom TOPOLOGY=all MATRIX=1 make test-smoke
```

### 5.2 Business groups and targeted runs

```bash
make test-group GROUP=config
make test-group GROUP=governance
make test-group GROUP=staking
make test-group GROUP=delegation
make test-group GROUP=punish
make test-group GROUP=rewards
make test-group GROUP=epoch
make test-group GROUP=all
```

Run only selected groups through the shared CI group runner:

```bash
make ci MODE=groups GROUPS=config,governance,staking
```

Run specific tests by pattern/package:

```bash
make ci MODE=tests RUN='TestI_PublicQueryCoverage' PKGS=./tests/rewards
```

Enable runtime budget gates for groups or targeted tests:

```bash
make ci MODE=groups BUDGET=1
make ci MODE=tests BUDGET=1 RUN='TestI_PublicQueryCoverage' PKGS=./tests/rewards
```

### 5.3 Regression bundles

```bash
make test-regression
make test-regression SCOPE=full
```

Notes:

- `test-regression` defaults to `SCOPE=core` and runs all discovered tests case-by-case under the default local environment
- `SCOPE=core` is not a scenario harness: topology/epoch/upgrade-specific `TestZ_*` cases may skip there by design
- `SCOPE=full` orchestrates smoke + business groups + fork matrix + PoSA + interop aggregation
- `make test` runs a single-pass `go test -run .` and expects network already ready

### 5.4 Fork/upgrade liveness matrix (dynamic)

```bash
make test-fork
TOPOLOGY=single make test-fork
TOPOLOGY=all make test-fork
```

Optional variables:

- `FORK_CASES` (default: `poa,upgrade:shanghaiTime,upgrade:cancunTime,upgrade:fixHeaderTime,upgrade:posaTime,upgrade:pragueTime,upgrade:osakaTime,upgrade:allStaggered,upgrade:allSame,posa`)
- `FORK_DELAY_SECONDS`
- `FORK_TEST_TIMEOUT`
- `FORK_REPORT_DIR`

Notes:

- Upgrade cases are selected by the same `runtime_capability.version_matrix`; unsupported fork targets are reported as `SKIP` in matrix mode.

Examples:

```bash
# only POA and one dynamic upgrade target
FORK_CASES=poa,upgrade:cancunTime TOPOLOGY=all make test-fork

# custom upgrade delay and timeout
FORK_CASES=upgrade:allStaggered FORK_DELAY_SECONDS=60 FORK_TEST_TIMEOUT=30m TOPOLOGY=multi make test-fork

# fixed report output directory
FORK_REPORT_DIR=reports/fork_custom TOPOLOGY=single make test-fork
```

### 5.5 Scenario suites

```bash
make test-scenario SCENARIO=posa
make test-scenario SCENARIO=all
make test-scenario SCENARIO=interop CHECK=sync
make test-scenario SCENARIO=interop CHECK=state-root
make test-scenario SCENARIO=interop CHECK=all
make test-scenario SCENARIO=checkpoint
make test-scenario SCENARIO=rotation-punish
make test-scenario SCENARIO=add-validator-live
make test-scenario SCENARIO=add-validator-punish
make test-scenario SCENARIO=negative
make test-scenario SCENARIO=upgrade
```

Scenario-only coverage notes:

- `checkpoint` covers the single-validator separated-signer split checks:
  - `TestZ_CheckpointRuntimeRewardsStillUseOldSigner`
  - `TestZ_CheckpointTransitionSignerSplit`
- `rotation-punish` covers:
  - `TestZ_SignerRotationMissingNewSignerTriggersPunishAndJail`
- `add-validator-live` covers:
  - `TestZ_AddValidatorWithSeparateSignerBecomesActiveAndSealsBlocks`
- `add-validator-punish` covers:
  - `TestZ_AddValidatorMissingSignerTriggersPunishAndJail`
- `upgrade` covers:
  - `TestZ_UpgradeOverrideBootstrapMapping`
- `negative` covers the guarded upgrade negatives:
  - `TestZ_UnderfundedUpgradeDefersMigration`
  - `TestZ_OverrideDriftRestartKeepsStoredMapping`

### 5.6 Performance / soak

```bash
make test-perf MODE=tiers
make test-perf MODE=soak
```

Generated perf artifacts:
- `summary.md`
- `metrics.csv`
- `verdict.json`

`verdict.json` includes:
- `failed_reasons`
- `top_slow_windows`
- `resource_peaks`

## 6. Key configuration

See `config/test_env.yaml.example` for full options.
Important fields:

- `network.genesis_mode`: `poa | upgrade | posa | smoke`
- `runtime.impl_mode`: `single | mixed`
- `runtime.impl`: `geth | reth`
- `runtime_nodes.nodeX.impl`: per-node implementation in mixed mode
- `runtime_nodes.nodeX.binary`: optional per-node native binary override
- `binaries.geth_native` / `binaries.reth_native`: native binary defaults when node-level override is empty
- `runtime_capability.version_matrix`: version-to-max-fork map used by smoke/fork matrix skipping
  - keys support exact (`1.16.8`), prefix (`1.16`), wildcard patch (`1.16.x`), and fallback (`default`)
  - topology capability is the minimum max-fork across all selected nodes
- `validator_auth.mode`: `auto | keystore` (reth validator auth, keystore-only)
- `network.fork_target`:
  - smoke static profiles:
    - `poa | poa_shanghai | poa_shanghai_cancun | poa_shanghai_cancun_fixheader | poa_shanghai_cancun_fixheader_posa | poa_shanghai_cancun_fixheader_posa_prague | poa_shanghai_cancun_fixheader_posa_prague_osaka`
  - upgrade dynamic targets:
  - `shanghaiTime | cancunTime | fixHeaderTime | posaTime | pragueTime | osakaTime`
  - `allStaggered` (all six fork timestamps are non-zero and increase by 60s)
  - `allSame` (all six fork timestamps are equal and non-zero)
- `network.fork_delay_seconds`: fork delay in seconds for upgrade mode
- `network.epoch`: base epoch length (can be overridden per run: `make init EPOCH=60`)
- `tests.profile`: `fast | default | edge`
- `tests.epoch_overrides`: group/special-case epoch overrides for speed and stability
- `perf.*`: tier/soak defaults and thresholds
- `ci.*`: PR/nightly/weekly profile defaults

## 6.1 Lifecycle quick recipes

```bash
# 1) single-node static PoA object
TOPOLOGY=single INIT_MODE=poa make init
make run
make status

# 2) multi-node static smoke object (fixed genesis fork profile)
TOPOLOGY=multi INIT_MODE=smoke INIT_TARGET=poa_shanghai_cancun make init
make run

# 3) multi-node dynamic upgrade object
TOPOLOGY=multi INIT_MODE=upgrade INIT_TARGET=allStaggered INIT_DELAY_SECONDS=60 make init
make run
```

Important:

- lifecycle commands always operate on the object created by the latest `make init`
- editing `config/test_env.yaml` does not change the active object until next `make init`

## 7. Reports and logs

- runtime artifacts: `data/`
- test reports: `reports/ci_<timestamp>/report.md`
- machine-readable run summary: `reports/ci_<timestamp>/summary.json`
- machine-readable run manifest: `reports/ci_<timestamp>/manifest.json`
- fork matrix report: `reports/fork_<timestamp>/matrix.md` + `matrix.json`
- smoke matrix report: `reports/smoke_matrix_<timestamp>/**/matrix.md` + `matrix.json`
- full regression index: `reports/regression_<timestamp>/index.md` + `index.json`

View logs:

```bash
make logs
NODE=ju-chain-validator1 make logs
```

## 8. Troubleshooting

1. Bytecode consistency failure
- Ensure `chain-contract/out` is rebuilt and consensus-side bytecode is synced, then rebuild geth.

2. geth binary is older than source/artifacts
- Rebuild geth under `chain_root` and retry.

3. reth keystore startup failure
- Confirm `data/nodeX/keystore/*.json` and `data/nodeX/password.txt` exist after `make init`.
- `reth` now uses keystore-only validator auth in `chain-tests`; configure `validator_auth.mode=auto|keystore`.

3. Fork config errors (fork ordering / blobSchedule)
- Regenerate genesis via project scripts (`make init`) instead of manual genesis edits.

## 9. Change workflow recommendations

When system contracts change:

1. Run `forge build` in `chain-contract`
2. Refresh consumed artifacts
3. `make reset`
4. Re-run affected test groups

When Congress consensus logic changes (`congress.go`):

1. Rebuild geth
2. Replace runtime binary
3. Restart from clean data and run regression

## 10. CI profiles

```bash
make ci PROFILE=pr
make ci PROFILE=nightly
make ci PROFILE=weekly-soak
make ci PROFILE=release
```
