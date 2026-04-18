# Fork Capability Implementation Blueprint

## 1. Objective

This blueprint turns the fork capability plan into an implementation-oriented design for `chain-tests`.

The target is a durable framework that:
- starts with Shanghai and Cancun concrete capability tests
- reuses existing repository primitives where possible
- remains structurally valid through the chain fork ladder `Shanghai -> Cancun -> fixHeader -> posa -> Prague -> Osaka -> bpo1 -> bpo2`
- can absorb new concrete capability items later without redesigning the harness

This document is deliberately more operational than `docs/fork_capability_test_plan.md`.

---

## 2. Current reusable building blocks already present in the repository

The framework does **not** start from zero. Several existing pieces can be reused directly.

## 2.1 `internal/context/CIContext`

Already provides:
- stable RPC bootstrap with retry
- funded transactor creation
- nonce management
- chain progress checks
- epoch-boundary avoidance helpers
- transaction waiting / mining helpers
- access to system contract bindings

This should remain the default execution substrate for fork capability tests.

### Why this matters
Fork capability tests still run on the same chain and hit the same runtime quirks as the existing integration tests:
- epoch boundary sensitivity
- nonce races
- startup stabilization
- multi-node local runtime variance

Reusing `CIContext` avoids rebuilding those mechanics.

---

## 2.2 `internal/testkit/fork_schedule.go`

Already provides ordered fork schedule logic for:
- shanghai
- cancun
- fixHeader
- posa
- prague
- osaka

This is the right source for understanding:
- enabled fork prefix ordering
- upgrade schedule validity
- long-range fork progression assumptions

The fork capability suite should follow the same ladder where relevant.

---

## 2.3 `internal/testkit/fork_surface.go`

This file is especially important because it already contains capability-style checks for later forks.

Existing helpers include:
- `VerifyForkRPCSurface(...)`
  - validates block / RPC surface expectations for Cancun, fixHeader, Prague, and Osaka
- `VerifyOsakaTxGasCap(...)`
  - validates the Osaka max transaction gas cap behavior

### Implication
The Prague / Osaka roadmap is not completely abstract. The repository already contains:
- a usable RPC surface check path
- at least one Osaka-specific behavioral check

That means the fork capability framework should be designed to *pull these into the new suite*, not duplicate them elsewhere.

---

## 2.4 Existing deployment and ABI patterns

The current test suites already demonstrate:
- deploying fresh contracts with generated Go bindings
- calling arbitrary ABI-encoded selectors
- using `eth_call` for negative and dry-run checks
- reading `eth_getBlockByNumber` directly through RPC

This means the fork capability framework can combine two existing styles:
- contract-binding-based checks where a probe contract exists
- raw-RPC / raw-bytecode checks where no binding is needed

---

## 3. Design goal: one framework, three implementation modes

The harness should support three execution modes because later forks will not all look the same.

## 3.1 Contract probe mode
Use a small probe contract and call one or more methods.

Best for:
- `PUSH0`
- `MCOPY`
- `TSTORE/TLOAD`
- future semantic checks that need controlled runtime behavior

## 3.2 Raw transaction / raw bytecode mode
Send raw calldata or bytecode directly without requiring a generated binding.

Best for:
- opcode gating checks where a minimal creation/runtime bytecode is easier than a full contract build
- transaction format checks
- special negative-path cases

## 3.3 RPC surface mode
Read block / RPC state and assert exposed fields / policy behavior.

Best for:
- Cancun blob-related header fields
- Prague request-hash surface
- Osaka tx-gas cap check
- future fork metadata checks

### Why this split matters
If the framework assumes every capability needs a probe contract, Prague / Osaka support will become clumsy. Some later-fork checks are better expressed as:
- block field visibility
- RPC method behavior
- transaction acceptance/rejection policy

So the framework should treat “probe contract” as one tool, not the only tool.

---

## 4. Proposed harness architecture

Recommended package layout under `internal/testkit/forkcap/`:

```text
internal/testkit/forkcap/
  registry.go        # capability metadata + ordered fork ladder
  suite.go           # suite composition
  harness.go         # top-level execution helpers shared by tests
  probe.go           # deploy/call helpers for small probe contracts
  rawtx.go           # raw transaction / raw bytecode helpers
  surface.go         # wrappers around existing fork_surface helpers
  report.go          # consistent deferred/skip/error formatting
  gating.go          # pre-fork vs post-fork execution orchestration
  selection.go       # CASE / suite selection helpers
```

## 4.1 `harness.go`
Should expose a small execution context wrapper around `CIContext`.

Responsibilities:
- provide the selected RPC / client
- expose helper methods for:
  - latest block
  - block-by-number RPC fetch
  - chain progress wait
  - funding and signer selection
- centralize common timeout defaults

This file should not know specific fork capability logic.

---

## 4.2 `probe.go`
Responsibilities:
- deploy a small probe contract
- call probe methods
- optionally support precompiled bytecode deployment if that is easier than generated bindings

The design should support both of these paths:

### Path A: generated Go binding
Use when a probe contract is stable enough to justify generated binding code.

### Path B: ABI + bytecode pack only
Use when the probe is tiny or evolving quickly and a full generated binding is unnecessary.

For the first implementation, Path B is likely enough for small forkcap probes.

---

## 4.3 `rawtx.go`
Responsibilities:
- send legacy transactions in a controlled way
- construct intentionally invalid / not-yet-enabled execution payloads where needed
- support future blob tx implementation once txpool policy is lifted

This file is where future blob transaction work will eventually live, but it can remain partial now.

---

## 4.4 `surface.go`
Responsibilities:
- wrap and expose the already existing helpers in `internal/testkit/fork_surface.go`
- normalize output for fork capability suite reporting

Initial wrappers should likely include:
- `CheckForkRPCSurface(...)`
- `CheckOsakaTxGasCap(...)`

These wrappers should live in `forkcap`, even if they call through to `testkit/fork_surface.go`, so the suite has one coherent API surface.

---

## 4.5 `gating.go`
Responsibilities:
- encode the common pattern:
  - run pre-fork expectation
  - run post-fork expectation
- centralize expected failure semantics

This is important because many capabilities will follow the same shape:
- before fork: reject
- after fork: accept

Without a shared gate runner, that logic will be duplicated across each test.

---

## 4.6 `report.go`
Responsibilities:
- render deferred reasons consistently
- distinguish:
  - deferred by chain policy
  - deferred by missing repository implementation
  - skipped due to fork selection mismatch
- keep logs short and explicit

This matters more as Prague / Osaka placeholders accumulate before concrete cases land.

---

## 5. Probe contract strategy

## 5.1 Keep probe contracts minimal

Recommended first probe set:

```text
contracts/forkcaps/
  shanghai/
    Push0Probe.sol
  cancun/
    McopyProbe.sol
    TransientStorageProbe.sol
```

### `Push0Probe.sol`
Purpose:
- expose one deterministic path that forces `PUSH0`
- return a simple value proving correct semantics

### `McopyProbe.sol`
Purpose:
- copy bytes in memory
- return copied bytes or hash
- keep logic short and deterministic

### `TransientStorageProbe.sol`
Purpose:
- write with `TSTORE`
- read with `TLOAD`
- expose same-tx and cross-tx behavior checks

## 5.2 Do not over-commit early to generated Go bindings

For forkcap probes, generated bindings may be overkill in the first pass.

Better initial approach:
- keep Solidity / Yul probe sources in the repo
- store ABI + bytecode artifacts in a controlled location when needed
- use generic ABI packing and raw deployment helpers for early implementation

Later, if a probe stabilizes and grows, it can be promoted to generated binding style.

### Why this matters
Generated bindings are excellent for large, stable contracts. Forkcap probes are usually:
- small
- experimental
- subject to change as the exact capability assertion evolves

The implementation should not force the overhead of full binding churn too early.

---

## 6. Fork capability matrix through Osaka

This section defines the practical roadmap, including what is already partially available.

## 6.1 Shanghai layer

### Concrete V1 case
- `push0_execution`

### Implementation mode
- contract probe or raw-bytecode gate check

### Assertions
- pre-Shanghai reject
- post-Shanghai success
- semantic result equals zero

### Dependency level
- no Prague/Osaka dependency
- good first proof that the framework works

---

## 6.2 Cancun layer

### Concrete V1 cases
- `mcopy_execution`
- `transient_storage_lifecycle`
- `cancun_header_surface`
- `blob_tx_submission` (deferred)

### Implementation modes
- probe contracts for `MCOPY`, `TSTORE/TLOAD`
- RPC surface mode for `cancun_header_surface`
- deferred placeholder for blob tx

### Assertions
- `MCOPY`: pre-fork reject, post-fork success, result correctness
- `TSTORE/TLOAD`: pre-fork reject, post-fork success, same-tx visibility, cross-tx non-persistence
- header surface: blob-related fields appear correctly post-Cancun
- blob tx: deferred with explicit reason

### Important note
Because blob tx is currently blocked by txpool policy, Cancun coverage should be split into:
- Cancun capability coverage that is runnable now
- Cancun blob capability slot that is visible but deferred

That split should remain explicit in reports.

---

## 6.3 Prague layer

### What already exists conceptually
`internal/testkit/fork_surface.go` already expects Prague-related surface through:
- `requestsHash`
- Prague blob schedule values in `eth_config`

### Near-term Prague coverage strategy
Before concrete Prague-specific probe contracts exist, the forkcap suite can already support:
- `prague_rpc_surface`
- inherited Cancun + Shanghai coverage
- deferred Prague placeholder entry for anything else not yet encoded

### Suggested Prague V1 concrete path
Use RPC surface mode first, not contract probes first.

Reason:
- the repository already has Prague surface awareness
- surface checks are cheaper to stabilize
- they establish a real Prague layer without waiting on full semantic probe inventory

### Proposed Prague milestone order
1. add a Prague suite placeholder in registry
2. wrap Prague-relevant parts of `VerifyForkRPCSurface(...)`
3. promote that wrapper into a named Prague capability item
4. leave remaining Prague items deferred until target set is confirmed

---

## 6.4 Osaka layer

### What already exists conceptually
The repository already contains Osaka-aware logic in `internal/testkit/fork_surface.go`:
- `VerifyOsakaTxGasCap(...)`

The forkcap package now also has authrpc/JWT plumbing to reach engine endpoints directly when needed.

### Current implemented Osaka coverage
The executable forkcap suite now validates:
1. `osaka_p256verify_precompile`
2. `osaka_engine_getpayload_transition`
3. `osaka_engine_blob_api_transition`
4. `osaka_tx_gas_cap`

### Current Osaka blob-engine coverage
`osaka_engine_blob_api_transition` now proves a real authrpc Osaka behavior change without requiring successful blob submission:
- pre-Osaka: `engine_getBlobsV2` and `engine_getBlobsV3` both return `null` for a missing blob request
- post-Osaka: `engine_getBlobsV2` still returns `null`, while `engine_getBlobsV3` exposes partial-response semantics as `[null]`

This is narrower than full blob-submission coverage, but it is a real Osaka-only engine API transition on the running chain.

### Remaining blob-related gap
End-to-end blob transaction submission remains deferred because the current chain still blocks blob transactions at the txpool layer. The active Osaka blob-engine capability therefore focuses on observable engine API behavior for missing-blob queries, not successful blob ingestion.

### Implementation mode
- precompile semantic mode for `P256VERIFY`
- authrpc/JWT engine gate mode for `engine_getPayloadV4` / `engine_getPayloadV5`
- authrpc/JWT blob-engine mode for `engine_getBlobsV2` / `engine_getBlobsV3` partial-response transition
- raw transaction mode for gas-cap rejection check

### Why Osaka started with these
- the repository already had supporting code for gas-cap verification
- `P256VERIFY` gives a real pre/post semantic gate on the live chain
- `engine_getPayloadV4` versus `engine_getPayloadV5` gives a real authrpc fork gate that is actually observable on this chain today
- `engine_getBlobsV2` versus `engine_getBlobsV3` gives a second real Osaka authrpc transition even while blob submission is still policy-blocked

### Osaka engine and probe coverage can expand later
If Osaka later exposes additional blob-engine or semantic execution probes, those can be added without changing the forkcap harness shape.

---

## 6.5 Current capability matrix

| Fork layer | Capability | Status | Current proof surface |
| --- | --- | --- | --- |
| Shanghai | `push0_execution` | active | raw-bytecode execution gate |
| Cancun | `mcopy_execution` | active | raw-bytecode execution gate |
| Cancun | `transient_storage_lifecycle` | active | raw-bytecode execution gate |
| Cancun | `cancun_header_surface` | active | block / RPC surface wrapper |
| Cancun | `blob_tx_submission` | deferred | txpool policy currently blocks blob success path |
| fixHeader | `fixheader_rpc_surface` | active | block / RPC surface wrapper |
| posa | `posa_contract_surface` | active | canonical PoSA system-contract deployment surface |
| Prague | `prague_rpc_surface` | active | block / RPC surface wrapper |
| Prague | `prague_setcode_tx` | active | semantic transaction gate + on-chain delegation-code effect |
| Prague | `prague_capability_matrix` | deferred | placeholder for later Prague semantic expansion |
| Osaka | `osaka_engine_getpayload_transition` | active | authrpc/JWT engine gate |
| Osaka | `osaka_engine_blob_api_transition` | active | authrpc/JWT blob-engine partial-response transition |
| Osaka | `osaka_p256verify_precompile` | active | precompile semantic gate |
| Osaka | `osaka_tx_gas_cap` | active | transaction validation gate |
| Osaka | `osaka_capability_matrix` | deferred | placeholder for later Osaka semantic expansion |
| BPO1 | `bpo1_blob_schedule` | active | exact fork-transition check on `eth_config` blob schedule |
| BPO2 | `bpo2_blob_schedule` | active | exact fork-transition check on `eth_config` blob schedule |

This matrix is the current truth surface for forkcap.
If a capability is not listed as `active`, the suite should not imply that it is already proven on the running chain.

## 7. Execution baseline used by the current implementation

The current concrete forkcap implementation intentionally uses a **single-node geth runtime** for the first opcode and protocol capability checks.

Reasons:
- deterministic opcode-gating baseline
- fewer runtime variables while proving the probe/gate/semantic model
- avoids cross-implementation ambiguity during the first capability bring-up

This does **not** mean the framework is geth-only forever.
It means the current forkcap line is intentionally stabilized on the narrowest reliable baseline.

### Current implementation boundary
For now, forkcap does **not** attempt to cover non-geth runtime differences.
That includes `reth` / `rchain` implementation-specific behavior, mixed-mode parity, or cross-client divergence analysis.
Those are explicitly out of the current execution scope and should be added later as a separate expansion milestone, after the geth-backed capability surface remains stable.

Future expansion can add:
- `reth` / `rchain` verification where appropriate
- mixed-mode verification where capability parity matters
- client-difference reporting when the same capability behaves differently across implementations

without changing the core forkcap harness shape.

---

## 8. Recommended step-by-step execution plan

The chosen execution priority is:

> **first prove the probe / gate / semantic path with Shanghai `PUSH0`, then extend the same harness into Cancun, and only after that absorb the broader Prague / Osaka surface checks into the forkcap suite.**

This ordering favors validating the hardest new framework seam first:
- deploy or execute a minimal probe
- assert pre-fork rejection
- assert post-fork success
- check returned semantics on the live chain

If that seam is not solid, later surface-only coverage will give a false sense of progress.

## Step 1: stabilize the metadata and selection layer
Deliverables:
- ordered registry through Osaka
- suite inheritance through Osaka
- clear deferred reasons
- stable `make test-forkcap FORK=...` command surface

Status:
- largely in place already

---

## Step 2: implement the common execution harness
Deliverables:
- `harness.go`
- `gating.go`
- `probe.go`
- `report.go`
- selection helpers

Verification:
- package unit tests for registry / suite logic
- compile check for harness package

This step should only build the generic machinery needed by the first concrete probe. It should avoid premature Prague / Osaka special cases.

---

## Step 3: implement the first concrete Shanghai path
Deliverables:
- `Push0Probe` or equivalent minimal raw-bytecode/Yul path
- `TestK_ForkcapCapability_Push0`

Verification:
- run against a configured pre-/post-Shanghai path
- explicit pre-fork fail and post-fork pass
- semantic result equals zero

This is the first proof that the framework is more than metadata.
It is also the proof that the forkcap suite can do what existing business tests do not: validate a fork-gated EVM capability directly.

### Design preference for Step 3
Start with the lightest mechanism that gives deterministic opcode control:
- raw bytecode or Yul is preferred if it is simpler and more stable than a full Solidity binding
- a generated Go binding is not required for the first Shanghai case

---

## Step 4: implement the first concrete Cancun semantic paths
Deliverables:
- `McopyProbe`
- `TransientStorageProbe`
- `TestK_ForkcapCapability_Mcopy`
- `TestK_ForkcapCapability_TransientStorage`

Verification:
- pre-/post-gate checks
- semantic assertions
- cross-transaction clearing for transient storage

At the end of this step, the framework should have proven:
- one Shanghai gate + semantic probe
- two Cancun gate + semantic probes

That is enough to lock in the core forkcap execution model.

---

## Step 5: absorb existing surface checks into forkcap
Deliverables:
- `surface.go` wrappers for `VerifyForkRPCSurface(...)`
- `surface.go` wrapper for `VerifyOsakaTxGasCap(...)`
- named forkcap tests for:
  - Cancun surface
  - Prague surface
  - Osaka gas-cap
- direct authrpc/JWT plumbing for Osaka engine checks
- a real Osaka authrpc gate based on `engine_getPayloadV4` / `engine_getPayloadV5`

This is the point where the framework becomes genuinely useful through Osaka, because it combines:
- concrete semantic probe coverage in the early forks
- reusable protocol surface coverage in the later forks
- a live authrpc access path for Osaka engine-level checks

This ordering keeps the framework honest: later-fork coverage is added *after* the probe path is proven, and authrpc claims stay deferred until the chain actually exhibits the intended Osaka-specific behavior.

---

## Step 6: keep blob tx deferred, but architect for later activation
Deliverables:
- preserve `blob_tx_submission` in the registry
- preserve explicit deferred reporting
- leave `rawtx.go` with a future extension point for blob tx construction

Do **not** force partial blob implementation while the chain policy still forbids it.

---

## Step 7: promote Prague / Osaka placeholders into concrete capability items incrementally
Deliverables:
- replace placeholder-only entries with real items when a target capability is worth testing on the live chain
- Prague now includes a real `SetCodeTx` semantic capability alongside `requestsHash` surface coverage
- Osaka now includes real authrpc `getPayloadV4` / `getPayloadV5` and `getBlobsV2` / `getBlobsV3` engine gates alongside P256 and tx-gas-cap checks

Rule:
- every new real Prague / Osaka item must land without changing the command surface or suite inheritance model
- every new real Prague / Osaka item should first be classified as probe, rawtx, surface, or authrpc-engine mode

---

## 8. Reporting model

Each capability run should end in one of these states:
- PASS
- FAIL
- DEFERRED
- SKIP (selection mismatch / intentionally not part of selected suite instance)

### 8.1 When to use DEFERRED
Use `DEFERRED` when:
- the capability belongs to the selected suite
- the repository recognizes it as part of the roadmap
- current execution is intentionally blocked by policy or missing implementation

Examples:
- Cancun blob tx under current txpool policy
- Prague/Osaka placeholders before concrete cases exist

### 8.2 When to use SKIP
Use `SKIP` when:
- the user selected a different suite
- the test file is shared but not relevant for the current fork selection

This distinction will matter in CI summaries.

---

## 9. Open design constraints to preserve

The future implementation should preserve these constraints:

1. **One fork ladder**
   - do not reintroduce ad hoc per-test switch logic for suite inheritance

2. **One reporting vocabulary**
   - pass / fail / deferred / skip should mean the same thing across the chain fork ladder from Shanghai through BPO2

3. **One command surface**
   - do not add new one-off commands just because Prague / Osaka / BPO upgrades arrive

4. **One harness API**
   - probe, rawtx, and surface modes can differ internally, but tests should feel consistent

5. **No fake completeness**
   - deferred items must stay visible
   - do not silently remove blob / Prague / Osaka placeholders when they are still real roadmap obligations

---

## 10. Proposed immediate next implementation scope

To keep momentum without overcommitting, the next implementation slice should be:

### 10.1 Build the harness foundation
- `harness.go`
- `gating.go`
- `surface.go`
- consistent deferred reporting

### 10.2 Make existing Osaka/Prague-aware logic visible through forkcap
- wrap `VerifyForkRPCSurface(...)`
- wrap `VerifyOsakaTxGasCap(...)`

### 10.3 Implement the first semantic probe only after the harness is stable
- start with `PUSH0`

This sequence is slightly different from a naive Shanghai-first path, because the repository already has real Osaka/Prague-capable surface checks waiting to be absorbed.

That is the shortest path to a framework that is not only *planned* through Osaka, but already *structurally useful* through Osaka.

---

## 11. Summary

The core design choice is:

> Treat fork capability testing as a single ordered framework that spans Shanghai, Cancun, Prague, and Osaka, rather than a collection of one-off fork-specific implementations.

The repository already contains enough reusable primitives to justify that approach today:
- `CIContext`
- fork schedule validation
- Cancun / Prague / Osaka surface checks
- Osaka gas-cap check

So the correct next move is not to wait for every later-fork semantic probe to be known.
The correct next move is to:
- finalize the ordered harness model
- absorb existing later-fork checks into it
- then populate concrete semantic capability cases incrementally

That is how this framework stays stable long enough to support Osaka without a redesign.
saka without a redesign.
 stable long enough to support Osaka without a redesign.
saka without a redesign.
 harness model
- absorb existing later-fork checks into it
- then populate concrete semantic capability cases incrementally

That is how this framework stays stable long enough to support Osaka without a redesign.
saka without a redesign.
 stable long enough to support Osaka without a redesign.
saka without a redesign.
s to:
- finalize the ordered harness model
- absorb existing later-fork checks into it
- then populate concrete semantic capability cases incrementally

That is how this framework stays stable long enough to support Osaka without a redesign.
saka without a redesign.
 stable long enough to support Osaka without a redesign.
saka without a redesign.
out a redesign.
saka without a redesign.
