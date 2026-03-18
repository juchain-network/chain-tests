# chain-tests AGENTS Guide

## 1. Project Goal

`chain-tests` is the main integration test repository for a custom Congress-based chain.
Its primary goals are to validate:

- end-to-end system contract behavior on real local multi-node networks
- integration behavior between consensus and system contracts
- regression stability of critical consensus paths, including validator-set updates, punishment, rewards, and governance parameter changes
- both `docker` and `native` multi-node runtime backends, selectable by configuration

This repository is the primary integration test workspace. It should focus on test orchestration, test cases, reports, and reusable test assets. It should not become the main development repository for system contracts.

---

## 2. Related Repositories And Boundaries

### 2.1 System Contracts Repository

- Suggested local path: `../chain-contract`
- Responsibility: system contract source code, Foundry unit tests, genesis-related scripts, and generated Go bindings

### 2.2 Existing Integration Test Prototype

- Suggested local path: `../chain-contract/test-integration`
- Responsibility: the current runnable local 4-node integration test prototype, including Makefile, Docker setup, Go tests, and CI runner
- Migration note: `chain-tests` should reuse its stable structure and test grouping strategy first, then extract and refactor into an independent test project

### 2.3 Custom Geth Repository

- Reference subpath: `../chain/accounts`
- Repository root: `../chain`
- Responsibility: node implementation, consensus execution, and system-transaction protection logic
- Dependency boundary: `chain-tests` only consumes compiled binaries from `chain`; it must not depend on `chain/local-test` scripts or configs

### 2.4 Congress Consensus Implementation

- Core file: `../chain/consensus/congress/congress.go`
- Current implementation-side constants:
  - default `epochLength = 86400`
  - `maxValidators = 21`
  - system contract addresses:
    - Validators: `0x...f010`
    - Punish: `0x...f011`
    - Proposal: `0x...f012`
    - Staking: `0x...f013`

### 2.5 chain-contract Artifact Boundary

- `chain-tests` depends only on compiled artifacts from `chain-contract`, such as `out/` artifacts and bytecode
- This repository must not build contract sources inside `chain-contract`; compiled outputs must be prepared ahead of time and referenced through configuration

---

## 3. Recommended Repository Layout

Keep a structure that stays close to the existing `test-integration` prototype:

```text
chain-tests/
├── AGENTS.md
├── Makefile
├── ci.go
├── docker/
├── scripts/
├── internal/
│   ├── context/
│   └── config/
├── tests/
├── templates/
├── data/                 # runtime-generated, not committed
└── reports/              # test report output
```

---

## 4. Test Execution Baseline

### 4.0 Runtime Backend Selection

Use a dual-backend runtime model:

- `native`:
  - recommended default for local development
  - uses `pm2` to manage multiple local `geth` processes
  - uses repository-owned files such as `scripts/native/pm2_init.sh` and `native/ecosystem.config.js`
  - advantages: faster startup/shutdown, no image build step, easier log inspection
- `docker`:
  - preferred for CI and environment consistency
  - continues to use `docker compose`
  - advantages: stronger dependency isolation and better cross-machine consistency

Select the runtime backend through `config/test_env.yaml`:

- `runtime.backend: native` -> use native process orchestration
- `runtime.backend: docker` -> use Docker orchestration

Dependency paths should be managed through `config/test_env.yaml` under `paths.*`, using repository-relative paths where possible, for example:

- `paths.chain_root: ../chain`
- `paths.chain_contract_root: ../chain-contract`
- `paths.chain_contract_out: ../chain-contract/out`

Epoch should be configured through `config/test_env.yaml` using `network.epoch`, for example `30` or `60`, and should be applied when generating `genesis.json`.
You may also override it for one initialization run with `make init EPOCH=60`.

Use `tests.profile` such as `fast`, `default`, or `edge` to control timing windows like cooldowns, lasting periods, and unbonding periods, instead of scattering hardcoded values.
High-frequency polling parameters should be tuned through `data/test_config.yaml`, for example `test.timing.retry_poll_ms` and `test.timing.block_poll_ms`, before changing code-level constants.

### 4.1 Local Network Topology Baseline

- Use a 4-node network baseline
- expose RPC through `http://localhost:18545`
- generate the following before startup:
  - node keys and node configs
  - `genesis.json`, including system contract alloc entries
  - `data/test_config.yaml`, including test accounts and RPC configuration

### 4.2 Recommended Command Conventions

- Initialize and start:
  - `make reset` which is equivalent to `clean + init + run + ready`
- Run full regression:
  - `make test-regression SCOPE=core`
  - `make test-regression SCOPE=full`
  - `make ci MODE=groups BUDGET=1`
  - `make ci MODE=tests BUDGET=1 RUN='TestI_PublicQueryCoverage' PKGS=./tests/rewards`
  - `make ci-budget-suggest`
  - `make ci-budget-suggest-json`
  - `make ci-budget-suggest-save`
  - `make ci-budget-drift-check`
  - `make ci-budget-selftest`
  - `make ci-budget-enforced`
  - `BUDGET_RECOMMEND_MIN_GROUP_SAMPLES` controls the minimum sample threshold used by budget recommendation logic
- Run business groups:
  - `make test-group GROUP=config`
  - `make test-group GROUP=governance`
  - `make test-group GROUP=staking`
  - `make test-group GROUP=delegation`
  - `make test-group GROUP=punish`
  - `make test-group GROUP=rewards`
  - `make test-group GROUP=epoch`
- Observe runtime:
  - `make init`
  - `make run`
  - `make ready`
  - `make stop`
  - `make reset`
  - `make status`
  - `make logs`

---

## 5. Hard Constraints For Consensus / Contract Integration

1. Epoch activation delay
Validator-set changes usually take effect at the next epoch boundary. Assertions for validator additions or removals must wait until the next epoch.

2. Physical node count constraint
When the local environment has only 4 physical nodes, do not set active-validator thresholds above what the network can actually sustain, or the chain may stall.

3. Proposal -> vote -> register ordering
A candidate validator must first pass proposal approval and then register within the valid window. Skipping steps should fail.

4. Protected system transactions
Methods such as `distributeBlockReward`, `punish`, and `updateActiveValidatorSet` must not be called directly through ordinary external transactions. Integration tests should verify effects, not bypass the protection model.

5. Transaction parameters
Prefer the legacy gas path using `GasPrice` to avoid local-chain incompatibilities with EIP-1559 behavior.

6. Nonce concurrency safety
When sending transactions concurrently, always use a shared context abstraction such as `CIContext` to manage nonce allocation and avoid flaky behavior.

---

## 6. Change Synchronization Rules

When system contracts change, the integration test side must also do the following:

1. build contracts in `chain-contract` with `forge build`
2. generate or update Go bindings
3. regenerate system contract bytecode and `genesis.json`
4. reset local test data and restart the network
5. rerun affected test groups

When Congress consensus logic changes in `congress.go`, you must:

1. rebuild the custom `geth` binary
2. replace the runtime binary used by the test network
3. restart from a clean data directory before rerunning regression

---

## 7. Test Development Rules

1. Test naming
Use stable grouped prefixes such as `TestA_`, `TestB_`, and so on, so CI grouping and failure localization stay predictable.

2. Assertion strategy
First assert chain-level state such as block progress and transaction inclusion, then assert business state such as validator set, stake, proposal result, or rewards.

3. Isolation
For highly coupled scenarios such as punishment, exit, and epoch transitions, prefer one isolated chain per test or per group reset to avoid cascading state pollution.

4. Observability
Failure logs must include the test name, transaction hash, current block height, and a snapshot of critical configuration values.

5. Prohibited practices
- do not rely on manual intervention to repair state
- do not keep retrying on a dirty data directory until a flaky test passes
- do not treat consensus-bypassing paths that would never happen in production as valid success criteria

---

## 8. Current Priorities

1. Migrate the stable capabilities from `chain-contract/test-integration` into `chain-tests` first, then refactor
2. Prioritize high-risk Congress regression coverage:
  - validator-set updates at epoch boundaries
  - punishment and recovery
  - dynamic governance parameter changes affecting consensus behavior
  - upgrade and initialization guard paths
3. Build repeatable, traceable local CI report output
4. Maintain dual runtime backends:
  - `native (pm2)` for fast local feedback
  - `docker` for stable CI regression
  - both must reuse the same test cases and assertion logic

---

## 9. Recommended Rollout Order For Dual Backend Support

1. Define a unified config file: `config/test_env.yaml`
2. Provide unified orchestration entrypoints under `scripts/network/*.sh`:
  - `docker.sh` to wrap compose `up/down/logs/ready`
  - `native.sh` to wrap pm2 `init/start/stop/logs/ready`
  - `dispatch.sh` to select the backend based on config
3. Keep `Makefile` as a thin wrapper over `dispatch.sh`, without hardcoding backend-specific commands
4. Default CI to `runtime.backend=docker` and local development to `runtime.backend=native`

Current migration-stage notes:

- the `native` backend already supports node start/stop and readiness checks through pm2 orchestration
- integration test data generation still primarily depends on `gen_network_config.sh`, and native-chain config is not yet fully unified with that flow
- until account and genesis configuration are fully unified:
  - prefer `native` for local debugging
  - prefer `docker` for automated regression
