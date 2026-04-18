# Fork Capability Test Plan

## 1. Purpose

This document defines a new test surface for `chain-tests`: **fork capability integration tests**.

These tests validate that a real running local chain environment actually exposes the protocol capabilities expected at a given fork boundary.

This is intentionally different from the existing integration suites:

- existing business / consensus integration tests validate JuChain-specific correctness
  - system contract flows
  - Congress consensus behavior
  - epoch transitions
  - punish / rewards / governance / staking behavior
- existing smoke / fork / upgrade suites validate runtime startup, liveness, and upgrade survival
- fork capability tests validate protocol-level EVM / geth-style capabilities on the real chain

Examples:
- Shanghai capability: `PUSH0` is actually accepted and executable after the fork
- Cancun capability: `MCOPY`, `TSTORE`, and `TLOAD` are actually accepted and executable after the fork
- Cancun surface: block / RPC responses expose Cancun-era header fields as expected

These tests are not intended to replace:
- geth internal Go unit tests
- EVM interpreter unit tests
- existing chain business integration tests

They fill a different gap: **real network capability verification on this chain's actual runtime environment**.

---

## 2. Motivation

Today `chain-tests` primarily answers:
- does the chain stay live?
- do consensus and system contracts behave correctly?
- do fork / upgrade paths preserve chain correctness?

It does **not** directly answer:
- does this running chain actually support the opcode set expected for a given fork?
- does fork gating reject new instructions before activation and allow them after activation?
- do fork-era block header / RPC surfaces appear correctly on the live chain?
- do protocol-level features remain available after custom chain integration changes?

That gap matters because a chain can:
- remain live after a fork transition
- still pass business flows
- but expose incomplete or regressed protocol capability at the VM / RPC layer

Fork capability tests make that failure mode visible.

---

## 3. Scope

### 3.1 In scope

This test surface verifies, on a real local running chain:

1. **Fork gate behavior**
   - capability is rejected before the target fork
   - capability is accepted after the target fork

2. **Capability execution**
   - the target opcode / EVM feature actually executes successfully
   - the execution result matches expected semantics

3. **Surface behavior**
   - block / RPC surfaces reflect the fork state where applicable

4. **Capability composition by fork level**
   - later fork suites include earlier fork capabilities
   - example: Cancun suite includes Shanghai capability coverage

5. **Long-range suite continuity**
   - the suite model must remain usable across the chain fork ladder from Shanghai through BPO2 without redesigning the command surface or registry model

### 3.2 Out of scope

These tests do not aim to validate:
- JuChain system contract business flows
- Congress-specific governance / staking / punish logic
- internal geth implementation details already covered by upstream tests
- formal EVM conformance coverage for every edge case of every opcode
- gas benchmarking or performance benchmarking

This is **not** a benchmark suite. It is a capability / compliance-style integration suite.

---

## 4. Relationship to existing test surfaces

The intended layering is:

1. **Business / consensus integration tests**
   - validate chain-specific correctness
   - current directories such as `tests/config`, `tests/governance`, `tests/staking`, `tests/punish`, `tests/rewards`, `tests/epoch`

2. **Liveness / fork survival / upgrade tests**
   - validate startup, runtime survival, upgrade transition, fork matrix viability
   - current directories such as `tests/smoke`, `tests/fork`, `tests/posa`, `tests/interop`

3. **Fork capability integration tests**
   - validate that the protocol capability itself exists on the live chain

The new suite should complement the current `test-fork` matrix:

- `test-fork` proves the chain stays alive under the selected fork / upgrade profile
- `test-forkcap` should prove the activated fork actually exposes the expected protocol features

Both are needed for a complete answer.

---

## 5. Long-term support target: chain fork ladder through BPO2

The fork capability framework must be designed around the chain's current runtime schedule order:

- Shanghai
- Cancun
- fixHeader
- posa
- Prague
- Osaka
- bpo1
- bpo2

This is the actual order enforced by the current chain configuration and scheduler validation.
If roadmap language lists the forks differently, forkcap should still execute in the runtime order above so pre/post proofs match the real chain.

The capability suite axis should follow that ordered ladder:

- `shanghai`
- `cancun`
- `fixheader`
- `posa`
- `prague`
- `osaka`
- `bpo1`
- `bpo2`

### 5.1 Why this matters now

If the suite model is only shaped around Shanghai and Cancun, it will likely encode assumptions that break when later forks arrive:
- duplicated directory layout
- one-off command names
- ad hoc skip logic
- missing inheritance model
- no place to record deferred future capabilities

That is avoidable if the registry, suite model, and command surface are built around an ordered fork ladder from the start.

### 5.2 Expected inheritance rule

Later suites inherit all earlier fork capability layers where the capability remains semantically valid.

Required composition rule:
- Shanghai suite = Shanghai capabilities
- Cancun suite = Shanghai + Cancun capabilities
- fixHeader suite = Shanghai + Cancun + fixHeader capabilities
- posa suite = Shanghai + Cancun + fixHeader + posa capabilities
- Prague suite = prior layers + Prague capabilities
- Osaka suite = prior layers + Osaka capabilities
- bpo1 suite = prior layers + BPO1 capabilities
- bpo2 suite = prior layers + BPO2 capabilities

This rule should hold even when:
- some later-fork capability cases are deferred
- a capability is present as a placeholder only
- a fork introduces no immediately implemented cases yet
- an exact fork-transition check such as a blob schedule change intentionally skips in later suites once a newer fork supersedes it

The suite definition should stay stable anyway.

---

## 6. Design principles

### 6.1 Capability-first, not fork-directory duplication

Do not create copied stacks such as:
- `tests/shanghai/...`
- `tests/cancun/...`
- `tests/prague/...`
- `tests/osaka/...`

That would duplicate contracts, scripts, and assertions.

Instead:
- define **atomic capability cases**
- compose them into **fork suites**

Example:
- Shanghai suite includes `push0`
- Cancun suite includes `push0 + mcopy + transient storage + cancun surface`
- Prague suite inherits all prior layers and adds Prague-specific capability items
- Osaka suite inherits all prior layers and adds Osaka-specific capability items

This gives the desired property that later fork suites include earlier fork capabilities, without cloning code.

### 6.2 Real-chain verification only

These tests must execute against the running local chain environment used by `chain-tests`.

They should not reduce to:
- compiler-only checks
- ABI generation checks
- isolated unit tests that never touch the live node

### 6.3 Fork-before / fork-after assertions

For each capability, prefer both:
- **before fork**: reject / fail
- **after fork**: accept / pass

A pure post-fork success case is weaker because it cannot prove that fork gating is correct.

### 6.4 Probe contracts should stay small

Capability contracts should be minimal and purpose-built.

Good:
- one probe for `PUSH0`
- one probe for `MCOPY`
- one probe for `TSTORE/TLOAD`

Avoid large “kitchen sink” capability contracts that test many unrelated features at once. They are harder to diagnose when they fail.

### 6.5 Go remains the primary test harness

The current repository is Go-first. The new capability suite should keep:
- Go as the main test runner
- Go for RPC interaction and assertions
- Solidity / Yul only for minimal probe contracts where needed

Python may still be useful for auxiliary generation tooling in the future, but it should not become the primary execution path unless a capability truly requires it.

### 6.6 Deferred is a first-class status, not an omission

The suite must be able to carry capability items that are:
- planned
- recognized as part of a fork layer
- intentionally not implemented yet
- still visible in reports and docs

This is required both for:
- current Cancun blob transaction handling
- future Prague / Osaka capability rollout

---

## 7. Capability taxonomy

To remain maintainable through Osaka, capability cases should be grouped by **type**, not only by fork.

Recommended capability classes:

### 7.1 Gate tests
Validate pre-fork rejection and post-fork acceptance.

Examples:
- `PUSH0`
- `MCOPY`
- `TSTORE`
- `TLOAD`

### 7.2 Semantic tests
Validate runtime semantics after activation.

Examples:
- `MCOPY` copies the expected bytes
- transient storage is visible within a transaction and cleared afterward

### 7.3 Surface tests
Validate externally visible protocol / RPC / header surfaces.

Examples:
- Cancun-era blob-related header fields appear on post-Cancun blocks
- future Prague / Osaka RPC or block surface changes appear as expected

### 7.4 Transaction-format tests
Validate end-to-end handling of new transaction types or payload models.

Examples:
- blob tx submission / inclusion once enabled
- any future Prague / Osaka transaction surface additions

### 7.5 Deferred tests
Track capabilities that belong in the suite but are temporarily blocked by explicit chain policy or missing infrastructure.

Examples:
- blob transaction submission / inclusion under current txpool restrictions
- Prague / Osaka capability placeholders before concrete target set is finalized in this repository

This taxonomy is important because later forks may add:
- no new opcodes, but new surface rules
- no new transaction type, but new validation rules
- no immediate runnable feature in the repository, but still a new suite boundary

The test model must still have a place for them.

---

## 8. Proposed repository structure

```text
contracts/forkcaps/
  shanghai/
    Push0Probe.sol
  cancun/
    McopyProbe.sol
    TransientStorageProbe.sol
    BlobHashProbe.sol
  prague/
    # reserved for future Prague probes
  osaka/
    # reserved for future Osaka probes

internal/testkit/forkcap/
  harness.go
  deploy.go
  bytecode.go
  call.go
  fork_gate.go
  header.go
  registry.go
  suite.go
  report.go
  skip.go

tests/forkcaps/
  common_test.go
  shanghai_suite_test.go
  cancun_suite_test.go
  prague_suite_test.go
  osaka_suite_test.go
  capability_push0_test.go
  capability_mcopy_test.go
  capability_transient_storage_test.go
  capability_header_surface_test.go
  capability_blob_tx_placeholder_test.go
```

### 8.1 Directory responsibilities

#### `contracts/forkcaps/`
Contains small probe contracts used only for fork capability validation.

#### `internal/testkit/forkcap/`
Contains reusable harness logic:
- deploy probe contract
- submit transaction
- perform `eth_call`
- wait for block / receipt
- inspect block header / RPC surfaces
- register capability metadata
- assemble fork suites
- render deferred / skipped reasons consistently

#### `tests/forkcaps/`
Contains test entrypoints and composed fork suites.

---

## 9. Capability registry model

Each capability should be treated as a first-class test item with metadata such as:
- name
- minimum fork
- category
- before-fork expectation
- after-fork expectation
- required contract / bytecode probe
- whether it is active, deferred, or skipped by design
- rationale when deferred

Illustrative examples:
- `push0_execution`
- `mcopy_execution`
- `transient_storage_lifecycle`
- `cancun_header_surface`
- `blob_tx_submission`
- `prague_capability_matrix`
- `osaka_capability_matrix`

The important part is the shape, not the exact struct field names.

A later implementation should make it easy to:
- run one capability directly
- run all capabilities for a given fork suite
- mark one capability as deferred without removing it from the suite definition
- render inherited deferred items in Prague / Osaka suites cleanly

### 9.1 Ordered fork ladder

The registry should understand a fixed ordered protocol ladder:
- `shanghai`
- `cancun`
- `prague`
- `osaka`

Filtering logic should be order-based, not hardcoded pairwise.

That means:
- requesting `cancun` includes `shanghai + cancun`
- requesting `prague` includes `shanghai + cancun + prague`
- requesting `osaka` includes `shanghai + cancun + prague + osaka`

This rule should live in common harness code, not be re-implemented in test files.

### 9.2 Deferred future-fork placeholders

The registry should explicitly allow future-fork placeholders such as:
- `prague_capability_matrix`
- `osaka_capability_matrix`

These are not meant to stay vague forever. Their purpose is:
- keep suite inheritance stable
- make future work visible in current reports
- avoid redesigning the registry when Prague / Osaka work begins

---

## 10. Fork suite composition model

The suite model should be incremental.

### 10.1 Baseline
Initially may be empty or contain only generic sanity checks.

### 10.2 Shanghai suite
Includes:
- `push0_execution`

### 10.3 Cancun suite
Includes:
- `push0_execution`
- `mcopy_execution`
- `transient_storage_lifecycle`
- `cancun_header_surface`
- `blob_tx_submission` (deferred for now)

### 10.4 Prague suite
Includes:
- all Shanghai capabilities
- all Cancun capabilities
- Prague placeholders now
- Prague concrete capabilities later when the target set is finalized

### 10.5 Osaka suite
Includes:
- all Shanghai capabilities
- all Cancun capabilities
- all Prague capabilities / placeholders
- Osaka placeholders now
- Osaka concrete capabilities later when the target set is finalized

This preserves the intended rule:
- later fork capability suites include all earlier fork capability layers

without duplicating code or contracts.

---

## 11. Initial concrete capability targets

## 11.1 Shanghai

### Capability: `PUSH0`

Goal:
- verify the live chain rejects `PUSH0` before Shanghai activation
- verify the live chain accepts and executes `PUSH0` after Shanghai activation
- verify observable semantics are correct

Recommended execution style:
- use explicit Yul / assembly or carefully controlled bytecode
- avoid relying purely on whatever Solidity happens to emit automatically

Minimum assertions for V1:
- pre-Shanghai execution fails
- post-Shanghai execution succeeds
- returned value is `0`

---

## 11.2 Cancun

### Capability: `MCOPY`

Goal:
- verify `MCOPY` is gated by Cancun activation
- verify the copy result is correct after activation

Minimum assertions for V1:
- pre-Cancun execution fails
- post-Cancun execution succeeds
- copied memory result matches expected bytes or hash

### Capability: `TSTORE/TLOAD`

Goal:
- verify transient storage opcodes are gated by Cancun activation
- verify transient storage semantics in a live transaction

Minimum assertions for V1:
- pre-Cancun execution fails
- post-Cancun execution succeeds
- `TSTORE` followed by `TLOAD` in the same transaction returns the stored value
- the transient value does not persist across transactions

### Capability: Cancun header / RPC surface

Goal:
- verify Cancun-era block / RPC surface appears correctly on the live chain

Minimum assertions for V1:
- block query returns Cancun-era blob-related header fields after Cancun activation
- pre-Cancun and post-Cancun behavior is distinguishable and consistent with the chain's selected implementation behavior

This case remains useful even while blob transaction support is deferred, because it validates the external visibility of the Cancun protocol surface.

---

## 12. Prague and Osaka planning model

At this stage, Prague and Osaka should be treated as **planned suite layers with concrete capability slots added incrementally**.

### 12.1 What is fixed now

Fixed now:
- command surface supports selecting `prague` and `osaka`
- registry model supports ordered suite inheritance through `osaka`
- documentation reserves future capability layers explicitly
- reports can carry deferred Prague / Osaka capability items without pretending they are implemented

### 12.2 What is not fixed now

Not fixed now:
- the exact concrete Prague capability list for this repository
- the exact concrete Osaka capability list for this repository
- which of those capabilities will be meaningful on this chain as real-chain integration tests versus upstream-only unit concerns

That uncertainty is normal. The framework should absorb it without redesign.

### 12.3 Required workflow for Prague / Osaka activation

When Prague or Osaka implementation work becomes relevant, the process should be:

1. identify the protocol capability set worth validating on the live chain
2. classify each item as:
   - gate
   - semantic
   - surface
   - transaction-format
3. register capability items in the shared registry
4. add minimal probe contracts only where needed
5. add explicit deferred rationale for anything blocked by chain policy or missing infra
6. keep the suite inheritance model unchanged

The key point is that new fork support should be mostly **data entry plus concrete capability implementation**, not a new architecture pass.

---

## 13. Blob transaction status

### 13.1 Current repository decision

**Blob transaction success-path testing is deferred for now.**

Reason:
- the current chain temporarily forbids blob transactions at the txpool layer

Therefore:
- Cancun capability planning should still include blob transaction support as a first-class capability item
- but current implementation should treat blob transaction submission / inclusion tests as **deferred**
- blob transaction support is **not** part of the current pass criteria for the initial fork capability suite

### 13.2 Required documentation behavior

The test plan and the future test output should state explicitly that:
- blob transactions are currently disabled by chain policy at the txpool layer
- blob transaction cases are intentionally deferred
- skipped / deferred output in this area is expected, not an unexplained gap

### 13.3 Deferred capability entry

Keep a placeholder capability item such as:
- `blob_tx_submission`

Its current expected behavior should be something like:
- skipped / deferred by design
- rationale recorded in report output

Do not silently omit blob transaction support from the plan. The deferred status should remain visible.

---

## 14. Harness expectations

The future harness under `internal/testkit/forkcap/` should be able to support at least:
- deploying minimal probe contracts
- executing probe methods or direct bytecode entrypoints
- forcing or selecting the intended fork profile through existing `make init` fork configuration
- waiting for receipts and block progress
- querying block headers through RPC
- rendering clear failure messages that identify:
  - which capability failed
  - whether failure happened in pre-fork or post-fork phase
  - what result was expected vs observed
- rendering clear deferred messages that identify:
  - which capability is deferred
  - why it is deferred
  - whether the deferral is temporary policy, missing infra, or future-fork placeholder

The harness should reuse the repository's existing integration testing style where possible, rather than inventing a second orchestration framework.

---

## 15. Proposed command surface

The new test surface should have its own entrypoint instead of being folded into business groups.

Recommended command family:

```bash
make test-forkcap FORK=shanghai
make test-forkcap FORK=cancun
make test-forkcap FORK=prague
make test-forkcap FORK=osaka
make test-forkcap FORK=all
```

Optional future selector:

```bash
make test-forkcap CASE=push0_execution
```

### 15.1 Why not `test-group`

Capability suites are not business groups.

They should not be mixed into the current group semantics:
- config
- governance
- staking
- delegation
- punish
- rewards
- epoch

Keeping fork capability tests separate makes the execution model and reporting clearer.

---

## 16. CI and regression integration

### 16.1 Initial expectation

Do not immediately overload the default core path with a large new capability matrix.

A reasonable rollout is:
- implement the capability suite first
- keep it as an explicit opt-in command surface
- later decide which small subset belongs in PR / nightly / release gates

### 16.2 Likely progression

#### Early stage
- explicit local execution only
- no mandatory CI gate yet

#### Middle stage
- PR or nightly may include a small critical subset
- probably Shanghai and Cancun only at first

#### Later stage
- nightly / full regression can include the complete Shanghai / Cancun / Prague / Osaka capability suites as they become real

### 16.3 Blob transaction handling in CI

Until txpool policy changes, blob transaction cases should report as:
- skipped
- deferred by design
- with explicit reason attached

They should not fail the suite under current policy.

### 16.4 Future-fork deferred handling in CI

Prague / Osaka placeholders should also report cleanly as deferred until concrete coverage lands.
They should remain visible, but they should not fail current CI merely because the repository has not implemented them yet.

---

## 17. Implementation roadmap

To keep the plan realistic, the work should be phased.

### Phase 0: framework stabilization
- keep the dedicated `test-forkcap` command surface
- keep ordered suite selection through `shanghai -> cancun -> prague -> osaka`
- keep registry support for deferred future-fork placeholders
- keep docs aligned with current chain policy

### Phase 1: Shanghai foundation
- implement `PUSH0` gate + semantics
- prove pre-fork / post-fork structure on a real chain

### Phase 2: Cancun foundation
- implement `MCOPY` gate + semantics
- implement `TSTORE/TLOAD` gate + lifetime semantics
- implement Cancun header / RPC surface checks
- keep blob tx deferred with explicit reason

### Phase 3: reporting hardening
- surface deferred status consistently in test output
- keep suite inheritance visible in reports
- ensure `make test-forkcap FORK=osaka` can still run and report inherited / deferred state coherently even before Osaka concrete cases exist

### Phase 4: Prague activation
- replace `prague_capability_matrix` placeholder with concrete Prague capability cases once target set is agreed
- maintain inheritance from Shanghai and Cancun automatically

### Phase 5: Osaka activation
- replace `osaka_capability_matrix` placeholder with concrete Osaka capability cases once target set is agreed
- maintain inheritance from all prior layers automatically

### Phase 6: blob tx activation
- once txpool policy allows it, replace Cancun blob placeholder with real coverage:
  - submission
  - pool acceptance
  - inclusion
  - receipt visibility
  - multi-node consistency

The important point is that later phases are **incremental population of the same framework**, not a second redesign.

---

## 18. Governance rules for future additions

To keep this suite healthy through Osaka, add these rules:

1. every new capability must declare its minimum fork
2. every deferred capability must declare an explicit reason
3. every new fork layer must be added through the ordered registry, not ad hoc switch logic in test files
4. later suites must inherit earlier layers automatically
5. chain-policy blocks and repository-not-yet-implemented blocks must be distinguished in reasons
6. avoid adding broad scenario contracts before atomic probe coverage exists

These rules matter more as the suite grows.

---

### 18.1 Current capability matrix

| Fork layer | Capability | Status | What is actually proved now |
| --- | --- | --- | --- |
| Shanghai | `push0_execution` | active | Pre-fork execution rejection; post-fork `PUSH0` executes and returns the expected zero word |
| Cancun | `mcopy_execution` | active | Pre-fork rejection; post-fork `MCOPY` execution succeeds on the running chain |
| Cancun | `transient_storage_lifecycle` | active | Pre-fork rejection; post-fork `TSTORE` / `TLOAD` lifecycle succeeds on the running chain |
| Cancun | `cancun_header_surface` | active | Cancun-era header / RPC surface is observable when Cancun is enabled |
| Cancun | `blob_tx_submission` | deferred | Blob tx success path remains blocked by current txpool policy |
| fixHeader | `fixheader_rpc_surface` | active | FixHeader-era header surface such as zero `parentBeaconBlockRoot`, plus post-fork baseFee validation, is observable when FixHeader is enabled |
| posa | `posa_contract_surface` | active | Pre-PoSA system contract set absent; post-PoSA validators/proposal/punish/staking contracts are deployed at canonical addresses |
| posa | `posa_proposal_wiring_surface` | active | Post-PoSA proposal contract is initialized and wired to the canonical validators / punish / staking contract addresses |
| Prague | `prague_rpc_surface` | active | Prague-era RPC / header surface such as `requestsHash` is observable when Prague is enabled |
| Prague | `prague_eth_config_precompile_surface` | active | Prague BLS12-381 precompile set appears in `eth_config.current.precompiles`, while Osaka-only `P256VERIFY` remains absent |
| Prague | `prague_setcode_tx` | active | Pre-Prague `SetCodeTx` rejection; post-Prague acceptance and delegation-code installation |
| Prague | `prague_capability_matrix` | deferred | Reserved slot for additional Prague semantic probes not yet implemented |
| Osaka | `osaka_engine_getpayload_transition` | active | Pre-Osaka `getPayloadV4` succeeds and `getPayloadV5` rejects; post-Osaka polarity flips |
| Osaka | `osaka_engine_blob_api_transition` | active | Pre-Osaka `getBlobsV2`/`V3` return `null`; post-Osaka `getBlobsV3` exposes partial-response semantics as `[null]` |
| Osaka | `osaka_eth_config_precompile_surface` | active | `eth_config.current.precompiles` exposes Osaka-only `P256VERIFY` at `0x0100` |
| Osaka | `osaka_modexp_gas_semantics` | active | Empty-input MODEXP call succeeds with `21300` gas pre-Osaka but requires at least `21600` gas after Osaka |
| Osaka | `osaka_p256verify_precompile` | active | Pre-Osaka precompile gate absent; post-Osaka `P256VERIFY` returns true for a valid secp256r1 vector |
| Osaka | `osaka_tx_gas_cap` | active | Pre-Osaka oversized tx remains accepted; post-Osaka oversized tx is rejected by gas-cap gate |
| Osaka | `osaka_capability_matrix` | deferred | Reserved slot for additional Osaka semantic probes not yet implemented |
| bpo1 | `bpo1_blob_schedule` | active | BPO1 blob schedule becomes the current `eth_config` schedule at the BPO1 boundary |
| bpo2 | `bpo2_blob_schedule` | active | BPO2 blob schedule becomes the current `eth_config` schedule at the BPO2 boundary |

Interpretation rules:
- `active` means the current harness proves a real pre-/post-fork difference or fork-gated surface on the running chain.
- `deferred` means the capability remains visible in reports with an explicit reason, but the suite does not currently claim proof.
- deferred items are part of roadmap honesty; they are not hidden just because the chain or harness cannot prove them yet.

## 19. Summary

This plan adds a protocol capability verification layer to `chain-tests` and explicitly shapes it to survive across the chain fork ladder from Shanghai through BPO2.

It should answer a question the repository does not answer today:

> On a real running local chain, does the selected fork actually expose the EVM and protocol capabilities expected at that fork boundary?

For the current concrete scope:
- Shanghai validates `PUSH0`
- Cancun validates `MCOPY`, `TSTORE/TLOAD`, and Cancun-era block / RPC surface
- fixHeader validates fixHeader-era header / RPC surface plus post-fork baseFee validation
- posa validates PoSA system-contract surface, proposal-contract wiring, and harness-configured proposal parameter exposure on the running chain
- Prague validates Prague-era RPC surface (`requestsHash`) and `SetCodeTx` fork gating plus delegation-code installation on the running chain
- Osaka validates `P256VERIFY` precompile activation, `eth_config` Osaka-only precompile exposure, MODEXP gas-threshold changes, authrpc `engine_getPayloadV4` / `engine_getPayloadV5` fork gating, authrpc `engine_getBlobsV2` / `engine_getBlobsV3` blob-API partial-response transition, and max transaction gas-cap enforcement through the forkcap suite
- bpo1 validates blob schedule transition through `eth_config`
- bpo2 validates blob schedule transition through `eth_config`
- blob transactions remain explicitly deferred because they are currently disabled at the txpool layer

For the long-term scope:
- the same registry and suite model must remain valid through the chain ladder `Shanghai -> Cancun -> fixHeader -> posa -> Prague -> Osaka -> bpo1 -> bpo2`
- later fork layers should be added by populating capability items, not redesigning the framework
- non-geth runtime coverage (`reth` / `rchain`, mixed-mode parity, and cross-client comparison) is intentionally deferred and is not part of the current forkcap success criteria

That is the point of this refinement: make the initial implementation small, but make the structure durable.
a

That is the point of this refinement: make the initial implementation small, but make the structure durable.
