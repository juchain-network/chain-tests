# chain-tests

`chain-tests` is the integration test project for a custom Congress-based chain.
It validates:

- end-to-end behavior of system contracts on real local chain topologies
- interaction between consensus and system contracts
- critical consensus paths (validator updates, punish, rewards, governance)
- PoA -> PoSA upgrade fork liveness
- both `native (pm2)` and `docker compose` runtimes
- both runtime implementations: `geth` and `reth`

## 1. Dependency boundary

This repository only consumes **compiled artifacts** from external repos:

- `chain`: geth binary (default: `<chain_root>/build/bin/geth`)
- `rchain`: reth binary (default: `<reth_root>/target/release/congress-node`)
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
- for docker runtime: Docker + Docker Compose

## 3. Quick start

### 3.1 Build external artifacts

Build contracts:

```bash
cd ../chain-contract
forge build
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
  backend: native # or docker
  impl_mode: single # single | mixed
  impl: geth # geth | reth

validator_auth:
  mode: auto # auto | private_key | keystore

paths:
  chain_root: ../chain-1.16/chain-1.16
  reth_root: ../rchain
  chain_contract_root: ../chain-contract
```

Path fields support absolute paths and relative paths. Relative paths are resolved
from the repository root (`chain-tests/`), not from `config/`.

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
  - when `INIT_MODE=smoke`: `poa_shanghai | poa_shanghai_cancun | poa_shanghai_cancun_fixheader | poa_shanghai_cancun_fixheader_posa`
  - when `INIT_MODE=upgrade`: `shanghaiTime | cancunTime | posaTime | fixHeaderTime | allStaggered | allSame`
- After `make init`, lifecycle commands operate on the generated runtime session snapshot:
  - default snapshot path: `data/runtime_session.yaml`
  - machine-readable copy: `data/runtime_session.json`
- If no runtime session exists, lifecycle commands (`run/stop/status/logs/net-*`) fail and require `make init` first.

### 3.4 Run tests

```bash
make test-all
```

## 4. Runtime backend switch

Backend is controlled by `config/test_env.yaml`:

- `runtime.backend: native` -> pm2 multi-process local nodes (faster local feedback)
- `runtime.backend: docker` -> docker compose multi-node runtime (better CI consistency)

Unified network commands:

```bash
make net-up
make net-down
make net-reset
make net-ready
```

Lifecycle commands are bound to the latest initialized runtime session object; editing
`config/test_env.yaml` after `make init` does not change the active object until next `make init`.

## 5. Common test commands

### 5.1 Smoke (standalone)

```bash
make test-smoke
make test-smoke-single
make test-smoke-matrix-single
make test-smoke-matrix-multi
make test-smoke-matrix-all
```

`test-smoke-single` key variables:

- `SMOKE_SINGLE_IMPL`: `geth | reth` (default `geth`)
- `SMOKE_SINGLE_AUTH_MODE`: `auto | private_key | keystore` (default `auto`)
- `SMOKE_SINGLE_GENESIS_MODE`: optional `poa | posa | smoke | upgrade`
- `SMOKE_SINGLE_FORK_TARGET`: required when `SMOKE_SINGLE_GENESIS_MODE=smoke|upgrade`
- `SMOKE_SINGLE_OBSERVE_SECONDS`: liveness observe window (default `60`)
- `SMOKE_SINGLE_TEST_TIMEOUT`: test timeout (default `12m`)

Examples:

```bash
# single-node smoke with defaults (geth + poa)
make test-smoke-single

# single-node smoke on reth
SMOKE_SINGLE_IMPL=reth make test-smoke-single

# single-node smoke on reth + keystore auth
SMOKE_SINGLE_IMPL=reth SMOKE_SINGLE_AUTH_MODE=keystore make test-smoke-single

# single-node static-fork genesis smoke
SMOKE_SINGLE_GENESIS_MODE=smoke SMOKE_SINGLE_FORK_TARGET=poa_shanghai_cancun make test-smoke-single
```

Smoke matrix is **static genesis fork-profile liveness** (no runtime upgrade scheduling).
Default `SMOKE_CASES`:
- `poa`
- `poa_shanghai`
- `poa_shanghai_cancun`
- `poa_shanghai_cancun_fixheader`
- `poa_shanghai_cancun_fixheader_posa`

Matrix examples:

```bash
# multi topology with selected static-fork cases
SMOKE_CASES=poa,poa_shanghai_cancun make test-smoke-matrix-multi

# single topology static-fork matrix
SMOKE_CASES=poa,poa_shanghai,poa_shanghai_cancun make test-smoke-matrix-single

# single + multi matrix with explicit report directory
SMOKE_REPORT_DIR=reports/smoke_matrix_custom make test-smoke-matrix-all
```

### 5.2 Grouped tests

```bash
make test-config
make test-governance
make test-staking
make test-delegation
make test-punish
make test-rewards
make test-epoch
```

Run only selected groups:

```bash
GROUPS=config,governance,staking make ci-groups
```

Run specific tests by pattern/package:

```bash
make ci-tests RUN='TestI_PublicQueryCoverage' PKGS=./tests/rewards
```

### 5.3 Full regression

```bash
make test-all
```

Notes:

- `test-all` runs all non-smoke tests case-by-case (smoke is separate)
- `make test` runs a single-pass `go test -run .` and expects network already ready

### 5.4 Fork/upgrade liveness matrix (dynamic)

```bash
make test-fork-single
make test-fork-multi
make test-fork-all
```

Optional variables:

- `FORK_CASES` (default: `poa,upgrade:shanghaiTime,upgrade:cancunTime,upgrade:posaTime,upgrade:fixHeaderTime,upgrade:allStaggered,upgrade:allSame,posa`)
- `FORK_DELAY_SECONDS`
- `FORK_TEST_TIMEOUT`
- `FORK_REPORT_DIR`

Examples:

```bash
# only POA and one dynamic upgrade target
FORK_CASES=poa,upgrade:cancunTime make test-fork-all

# custom upgrade delay and timeout
FORK_CASES=upgrade:allStaggered FORK_DELAY_SECONDS=60 FORK_TEST_TIMEOUT=30m make test-fork-multi

# fixed report output directory
FORK_REPORT_DIR=reports/fork_custom make test-fork-single
```

### 5.5 PoSA / full regression

```bash
make test-posa-multi
make test-interop-sync
make test-interop-state-root
make test-regression-all
```

### 5.6 Performance / soak

```bash
make test-perf-tiers
make test-soak-24h
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
- `runtime_nodes.nodeX`: per-node impl selection in mixed mode
- `validator_auth.mode`: `auto | private_key | keystore` (reth validator auth)
- `network.fork_target`:
  - smoke static profiles:
    - `poa | poa_shanghai | poa_shanghai_cancun | poa_shanghai_cancun_fixheader | poa_shanghai_cancun_fixheader_posa`
  - upgrade dynamic targets:
  - `shanghaiTime | cancunTime | posaTime | fixHeaderTime`
  - `allStaggered` (all four fork timestamps are non-zero and increase by 60s)
  - `allSame` (all four fork timestamps are equal and non-zero)
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
- Or set `validator_auth.mode=private_key` to force `--validator-private-key`.

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
make ci-pr-gate
make ci-nightly-full
make ci-weekly-soak
make ci-release-gate
```
