# JuChain Integration Test Plan

This document outlines the end-to-end integration test paths for JuChain system contracts. To ensure test efficiency, the execution order is crucial.

**Execution Strategy**:
1.  **Parameter Adjustment (Phase 0)**: First, use governance proposals to shorten system time parameters (e.g., unbonding period, jail period) to create suitable conditions for subsequent tests.
2.  **Core Functionalities (Phase 1)**: Execute the full flow of validator onboarding, staking, and delegation under the new parameters.
3.  **Boundaries & Exceptions (Phase 2)**: Interleave various exception scenario tests throughout the process.

**Signer Query Semantics**:
- `getTopSigners()` = current-block runtime signer set.
- `getTopSignersForEpochTransition()` = checkpoint-only signer set committed into `header.Extra` for the next epoch.
- On the checkpoint block itself, runtime hooks still resolve the old signer; the rotated signer becomes effective from the first block after the checkpoint.
- `getValidatorBySignerHistory()` only records signers that have actually entered the effective consensus signer set; a pre-activation rotated signer must still resolve to zero and must not be punishable through `submitDoubleSignEvidence(...)`.

**Bootstrap Generation Semantics**:
- Fresh PoSA genesis must write both `config.congress.initialValidators` and `config.congress.initialSigners`.
- `extraData` must commit the signer hot-address set, not the validator cold-address set.
- Bootstrap self-stake is funded from validator cold-address alloc; the test generator must not rely on prefunding the `Staking` contract.
- Local generation should support both `same_address` and `separate` signer modes so the same test cases can cover cold/hot identity split.

**Scenario Entrypoints**:
- `make test-scenario SCENARIO=bootstrap` validates same/separate bootstrap generation and the single-topology native dispatcher path.
- `make test-scenario SCENARIO=upgrade` validates CLI override propagation (`--override.posaTime`, `--override.posaValidators`, `--override.posaSigners`) and the PoA->PoSA override mapping path.
- `make test-scenario SCENARIO=checkpoint` validates all three checkpoint signer-split paths:
  - rewards still settle through the old runtime signer on the checkpoint block
  - punish checkpoint flow still resolves the old runtime signer, while `header.Extra` already commits the transition signer set
  - the first block after the checkpoint accepts the new signer as runtime `coinbase`
- `make test-scenario SCENARIO=negative` validates:
  - partial override inputs are rejected at generation time
  - fresh PoSA underfunded bootstrap does not progress past genesis
  - underfunded PoA->PoSA upgrade is deferred while the chain stays live
  - override drift restart either fails immediately or keeps the persisted stored mapping instead of activating the drifted override

---

## 0. System Config & Setup - **PRIORITY 1**

This phase both tests the "Update Config" functionality and prepares the environment for subsequent tests.

### 0.1 Normal Flow (Setup)
*   **[C-01] Shorten Key Period Parameters**
    *   **Goal**:
        *   `unbondingPeriod`: set to 100 blocks (was 7 days) -> allows testing withdrawals.
        *   `validatorUnjailPeriod`: set to 50 blocks (was 1 day) -> allows testing Unjail.
        *   `proposalCooldown`: set to 10 blocks (was 100) -> allows back-to-back proposals.
        *   `proposalLastingPeriod`: set to 200 blocks (was 7 days) -> ensures time for voting while easy to test expiry.
        *   `withdrawProfitPeriod`: set to 20 blocks (was 1 day) -> allows fast testing of profit withdrawals.
        *   `commissionUpdateCooldown`: set to 50 blocks (was 7 days) -> allows testing commission updates.
    *   **Steps**:
        1.  Validator initiates a ProposalType=2 proposal (CID corresponding to parameters above).
        2.  Other validators vote to pass.
        3.  Check if variable values in `Params` contract are updated.
    *   **Expected**: Parameters updated successfully, Event `LogPassProposal` triggered.

### 0.2 Parameter Validation (Boundaries)
This section covers validation logic for all configuration parameters.

*   **[C-02] General Validation**
    *   **Invalid Config ID**: Attempt to modify CID=20 (out of range). Expected: Revert "Invalid config ID".
    *   **Zero Value**: Attempt to set any parameter to 0. Expected: Revert "Config value must be positive".

*   **[C-03] Punishment Threshold Logic Conflicts (CID 1, 2, 3)**
    *   **Punish >= Remove**: Set `punishThreshold` (CID 1) >= current `removeThreshold`. Expected: Revert "punishThreshold must be < removeThreshold".
    *   **Remove <= Punish**: Set `removeThreshold` (CID 2) <= current `punishThreshold`. Expected: Revert "removeThreshold must be > punishThreshold".
    *   **Remove < Decrease**: Set `removeThreshold` (CID 2) < current `decreaseRate`. Expected: Revert "removeThreshold must be >= decreaseRate".
    *   **Decrease > Remove**: Set `decreaseRate` (CID 3) > current `removeThreshold`. Expected: Revert "decreaseRate must be <= removeThreshold".

*   **[C-04] Consensus & Safety Parameters (CID 9, 14)**
    *   **Max Validators Overflow**: Set `maxValidators` (CID 9) > 21 (Hardcoded consensus limit). Expected: Revert "maxValidators exceeds consensus limit".
    *   **Zero Burn Address**: Set `burnAddress` (CID 14) to 0 address. Expected: Revert "burnAddress must be non-zero".
    *   **Burn Address Overflow**: Set `burnAddress` (CID 14) > uint160 max. Expected: Revert "burnAddress invalid".

*   **[C-05] Economic Model Parameters (CID 12, 13, 17, 18)**
    *   **Reward > Slash**: Set `doubleSignRewardAmount` (CID 13) > current `doubleSignSlashAmount`. Expected: Revert "doubleSignRewardAmount must be <= doubleSignSlashAmount".
    *   **Slash < Reward**: Set `doubleSignSlashAmount` (CID 12) < current `doubleSignRewardAmount`. Expected: Revert "doubleSignSlashAmount must be >= doubleSignRewardAmount".
    *   **Invalid Base Ratio**: Set `baseRewardRatio` (CID 17) > 10000 (100%). Expected: Revert "baseRewardRatio must be <= 10000".
    *   **Invalid Max Commission**: Set `maxCommissionRate` (CID 18) > 10000 (100%). Expected: Revert "maxCommissionRate must be <= 10000".

---

## 1. Governance & Onboarding

*   **Prerequisite**: Use new parameters set in [C-01].

### 1.1 Normal Flow
*   **[G-01] New Validator Onboarding Flow**
    *   **Steps**: Existing validator initiates proposal (Add Validator) -> Vote passes (Agree > 50%) -> Candidate calls `registerValidator`.
    *   **Expected**: Candidate successfully registered, status becomes `Active`, `getHighestValidators` includes this address.
*   **[G-02] Remove Validator Flow**
    *   **Steps**: Initiate proposal (Remove Validator) -> Vote passes.
    *   **Expected**: Target validator `pass` status becomes false, marked as `Unpassed`. If in `currentValidatorSet`, they will be removed at next Epoch.
*   **[G-03] Validator Re-onboarding Flow**
    *   **Scenario**: For validators already removed (or slashed).
    *   **Steps**: Initiate proposal (Add Validator, target is old address) -> Vote passes -> Target calls `unjailValidator` or `registerValidator`.
    *   **Expected**: `pass` status reset to true, validator re-enters Active status.
*   **[G-04] Reject Proposal Flow**
    *   **Steps**: Initiate proposal -> Over 50% validators vote `false` (Reject).
    *   **Expected**: Proposal state ends, Event `LogRejectProposal` triggered, target state unchanged.

### 1.2 Combined Scenarios
*   **[G-13] Flip-Flop (Add & Remove)**
    *   **Steps**: Add V -> Vote passes -> Register -> Remove V -> Vote passes -> Add V -> Vote passes -> Revive.
    *   **Expected**: State switches correctly every time, no residue from previous rounds.
*   **[G-14] Parallel Governance**
    *   **Steps**: Initiate Add Validator proposal -> Initiate Config Update proposal -> Vote Config -> Vote Validator.
    *   **Expected**: Both proposals work independently without interference. Verify Nonce ensures unique IDs.
*   **[G-15] Dynamic Threshold**
    *   **Scenario**: Proposal needs 3 votes (out of 4).
    *   **Steps**: Proposal created -> 2 people vote -> V4 removed (Total becomes 3, threshold becomes 2) -> Check proposal state.
    *   **Expected**: The next `voteProposal` (or `results` query) should recognize the new threshold.

### 1.3 Security & Rate Limiting - **NEW**
*   **[G-16] Smooth Expansion**
    *   **Scenario**: Verify that at most 1 active validator can be added per Epoch.
    *   **Steps**:
        1.  Register validator A.
        2.  Register validator B.
        3.  Query `getTopValidators`.
    *   **Expected**: List length relative to `currentActiveSet` increases by at most 1.

### 1.4 Exceptions & Boundaries
*   **[G-05] Proposal Cooldown**
    *   **Steps**: Same validator initiates proposals within 10 blocks (new param).
    *   **Expected**: Revert "Proposal creation too frequent".
*   **[G-06] Duplicate Proposal**
    *   **Steps**:
        1. Initiate proposal A (Add V1).
        2. Before A ends, initiate proposal B (Add V1).
    *   **Expected**: Revert "Proposal already exists".
*   **[G-07] Front-running Register**
    *   **Steps**: Proposal votes = 50% (not reached >50% threshold), candidate attempts `registerValidator`.
    *   **Expected**: Revert "Must pass proposal first".
*   **[G-08] Invalid Voting Behavior**
    *   **Expired**: Vote on proposal exceeding `proposalLastingPeriod`. Expected: Revert "Proposal expired".
    *   **Double Vote**: Vote twice on same ID. Expected: Revert "You can't vote for a proposal twice".
    *   **Non-Existent**: Vote on random ID. Expected: Revert "Proposal does not exist".
*   **[G-09] Description Too Long**
    *   **Steps**: `createProposal` with `details` exceeding 3000 bytes.
    *   **Expected**: Revert "Details too long".
*   **[G-10] Duplicate Add of Existing Validator**
    *   **Steps**: Initiate Add proposal for validator already in `highestValidatorsSet` with `pass=true`.
    *   **Expected**: Revert "Validator is already in top validator set".
*   **[G-11] Ghost Removal**
    *   **Steps**: Initiate Remove proposal for random address -> Vote passes.
    *   **Expected**: Flow succeeds, Event `LogPassProposal` triggered, but no impact on on-chain validator set (No-op).
*   **[G-12] Last Man Standing**
    *   **Scenario**: Only 1 validator left in network.
    *   **Steps**: Initiate Remove self proposal -> Passes.
    *   **Expected**: Proposal passes, but actual removal skipped due to protection, validator remains in set.

---

## 2. Staking & Management

### 2.1 Normal Flow
*   **[S-01] Add Stake**
    *   **Steps**: Validator calls `addValidatorStake`.
    *   **Expected**: `selfStake` increases.
*   **[S-02] Decrease Stake**
    *   **Steps**: Validator calls `decreaseValidatorStake`.
    *   **Expected**: `selfStake` decreases, amount moved to `unbondingDelegations`.
*   **[S-03] Edit Info**
    *   **Steps**: Validator calls `createOrEditValidator`.
    *   **Expected**: Info updated successfully.
*   **[S-04] Update Commission**
    *   **Steps**: Validator calls `updateCommissionRate`.
    *   **Expected**: Rate updated.

### 2.2 Combined Scenarios
*   **[S-05] Reincarnation Flow**
    *   **Steps**: `resign` -> Proposal(Add) -> wait `validatorUnjailPeriod` -> `unjail`.
    *   **Expected**: Successfully "revived" and reactivated (via unjail + tryActive).
*   **[S-17] Stake Jitter**
    *   **Steps**: Add Stake -> Wait -> Decrease Stake -> Wait -> Add Stake.
    *   **Expected**: `accumulatedRewards` settled and accumulated before each change.
*   **[S-18] Mixed Stakes**
    *   **Steps**: V self-stakes -> D delegates to V -> V decreases stake -> D increases delegation.
    *   **Expected**: `totalStaked` remains consistent, distribution ratio dynamically adjusts.

### 2.3 Exceptions & Boundaries
*   **[S-06] Stake Below Minimum**
    *   **Steps**: `msg.value < minValidatorStake` during registration.
    *   **Expected**: Revert "Insufficient self-stake".
*   **[S-07] Partial Withdraw Underflow**
    *   **Steps**: `decreaseValidatorStake` such that remaining `selfStake` < `minValidatorStake`.
    *   **Expected**: Revert "Remaining stake below minimum, withdraw all stake instead".
*   **[S-08] Decrease Stake to Zero**
    *   **Steps**: `decreaseValidatorStake` with amount equal to current `selfStake`.
    *   **Expected**: Revert "Remaining stake below minimum" (suggest using Exit).
*   **[S-09] Frequent Commission Update**
    *   **Steps**: Update commission again during `commissionUpdateCooldown`.
    *   **Expected**: Revert "Commission update too frequent".
*   **[S-10] Non-Validator Operations**
    *   **Steps**: Unregistered address calls `addValidatorStake` or `updateCommissionRate`.
    *   **Expected**: Revert "Validator not registered".
*   **[S-11] Duplicate Registration**
    *   **Steps**: Registered validator calls `registerValidator` again.
    *   **Expected**: Revert "Already registered".
*   **[S-12] Zombie Register**
    *   **Steps**: Previously slashed/removed address (pass=false) attempts `registerValidator`.
    *   **Expected**: Revert "Must pass proposal first".
*   **[S-13] Zombie Action**
    *   **Steps**: Immediately call `addValidatorStake` after `exitValidator`.
    *   **Expected**: Revert "Validator not registered".
*   **[S-14] Jailed Constraints**
    *   **Steps**:
        1. Simulate Jail.
        2. `createOrEditValidator` (Success).
        3. `addValidatorStake` (Success).
        4. `updateCommissionRate` (Fail, Revert "Validator is jailed").
*   **[S-01b] Add Stake Zero**
    *   **Steps**: Validator calls `addValidatorStake` with `msg.value = 0`.
    *   **Expected**: Revert "Amount must be positive".
*   **[S-04b] Invalid Commission Rate**
    *   **Steps**: `updateCommissionRate(0)` or `updateCommissionRate(>maxCommissionRate)`.
    *   **Expected**: Revert "Commission rate must be greater than 0" / "Commission rate exceeds maximum allowed".

### 2.4 Additional Robustness & System Hooks
*   **[S-15] Proposal Expiry Boundary**
    *   **Steps**: Proposal passes -> wait `proposalLastingPeriod` -> attempt `registerValidator`.
    *   **Expected**: Revert "Proposal expired, must repropose".
*   **[S-16] Zero Delegated Rewards**
    *   **Steps**: Validator with `totalDelegated = 0` receives rewards.
    *   **Expected**: Rewards accumulate to validator (no delegator split).
*   **[S-19] Unbonding Limit (Validator Self-Stake)**
    *   **Steps**: Call `decreaseValidatorStake` 20 times; 21st fails.
    *   **Expected**: Revert "Too many unbonding entries".
*   **[S-20] Double-Sign Safety Window**
    *   **Steps**: Validator tries to `resign` within `doubleSignWindow` of last active block.
    *   **Expected**: Revert "Exit blocked in doubleSignWindow".
*   **[S-21] Initialization Protection**
    *   **Steps**: Call `initialize`/`initializeWithValidators` again.
    *   **Expected**:
        *   External transaction path: rejected by node with `forbidden system transaction`.
        *   Direct contract execution path (eth_call/simulation): reverts with `Already initialized`.
*   **[S-22] Distribute Rewards & Claim Cooldown**
    *   **Steps**: Miner calls `distributeRewards` -> validator claims -> distribute again -> claim before cooldown. Include one checkpoint-block execution.
    *   **Expected**:
        *   First claim success; second claim reverts "Must wait withdrawProfitPeriod blocks between claims".
        *   On the checkpoint block, `Staking.distributeRewards()` still resolves the validator through the old runtime signer, not the transition signer set.

---

## 3. Delegation & Rewards

### 3.1 Normal Flow
*   **[D-01] Full Delegation & Withdrawal**
    *   **Steps**: `delegate` -> `claimRewards` -> `undelegate` -> `withdrawUnbonded`.
    *   **Expected**: Rewards received, principal successfully withdrawn.
*   **[D-01b] Withdraw Unbonded (After Period)**
    *   **Steps**: `delegate` -> `undelegate` -> wait `unbondingPeriod` -> `withdrawUnbonded`.
    *   **Expected**: Unbonded entries cleared, principal withdrawn.
*   **[D-02] Validator Claims Commission**
    *   **Steps**: Wait `withdrawProfitPeriod` -> `claimValidatorRewards`.
    *   **Expected**: Successful claim.
*   **[D-02b] Claim Without Delegation**
    *   **Steps**: Call `claimRewards` without any delegation.
    *   **Expected**: Revert "No delegation found".

### 3.2 Combined Scenarios
*   **[D-03] Compound Delegation**
    *   **Steps**: `delegate` -> Wait -> `delegate`.
    *   **Expected**: Previous rewards auto-claimed on second delegation.
*   **[D-04] Multi-Delegator**
    *   **Steps**: A and B both delegate to V.
    *   **Expected**: V's `totalDelegated` correct, rewards isolated between A and B.
*   **[D-05] Multi-Validator**
    *   **Steps**: A delegates to V1 and V2 separately.
    *   **Expected**: Two independent delegation relationships.
*   **[D-15] Role Upgrade**
    *   **Steps**: A delegates to B -> A initiates proposal to register as validator -> Register.
    *   **Expected**: Success. A holds dual roles.
*   **[D-16] Circular Delegation**
    *   **Steps**: V1 delegates to V2, V2 delegates to V1.
    *   **Expected**: Success.
*   **[D-17] Role Downgrade**
    *   **Steps**: V1 exits -> V1 delegates to V2.
    *   **Expected**: Success.

### 3.3 Exceptions & Boundaries
*   **[D-06] Early Withdrawal**
    *   **Steps**: `withdrawUnbonded` immediately after `undelegate`.
    *   **Expected**: Withdraw amount 0 or Revert.
*   **[D-07] Self-Delegation**
    *   **Steps**: Validator calls `delegate` to self.
    *   **Expected**: Revert "Cannot delegate to yourself".
*   **[D-08] Zero Delegation**
    *   **Steps**: `msg.value = 0` for `delegate`.
    *   **Expected**: Revert "Insufficient delegation amount".
*   **[D-09] Delegate to Non-Existent**
    *   **Steps**: `delegate` to random address.
    *   **Expected**: Revert "Validator not registered".
*   **[D-10] Over-Undelegate**
    *   **Steps**: Staked 10 ETH, attempt `undelegate` 11 ETH.
    *   **Expected**: Revert "Insufficient delegation".
*   **[D-11] Zero Undelegate**
    *   **Steps**: `undelegate(0)`.
    *   **Expected**: Revert "Amount must be positive".
*   **[D-18] Undelegate Below Minimum**
    *   **Steps**: `undelegate` with `amount < minUndelegation` and `amount > 0`.
    *   **Expected**: Revert "Insufficient undelegation amount".
*   **[D-12] Delegate to Jailed**
    *   **Steps**: Call `delegate` on jailed validator.
    *   **Expected**: Revert "Validator is jailed".
*   **[D-13] Undelegate from Jailed**
    *   **Steps**: Call `undelegate` from jailed validator.
    *   **Expected**: Success (Escape path).
*   **[D-14] Unbonding Queue Full**
    *   **Steps**: Call `undelegate` 21 times (assuming MAX=20).
    *   **Expected**: Revert "Too many unbonding entries".
*   **[D-19] Invalid `maxEntries` on `withdrawUnbonded`**
    *   **Steps**: Call `withdrawUnbonded` with `maxEntries=0` or `> MAX`.
    *   **Expected**: Revert "maxEntries must be positive" / "maxEntries too large".

---

## 4. Punishment & Exit

### 4.1 Normal Flow
*   **[P-01] Resign -> Unjail Path**
    *   **Steps**: `resign` -> `unjail` -> `register`/`active`.
    *   **Expected**: Completion of loop.
*   **[P-02] Thorough Exit**
    *   **Steps**: `resign` -> Wait -> `exit`.
    *   **Expected**: Validator fully removed, Stake cleared.
*   **[P-07] Submit Evidence**
    *   **Steps**: Construct and submit double-sign evidence.
    *   **Expected**:
        *   Slash + Jail.
        *   Evidence is built on signer identity.
        *   If the evidence targets the checkpoint block, validator resolution still follows the old runtime signer.
*   **[P-08] Withdraw Profits**
    *   **Steps**: Call `withdrawProfits`.
    *   **Expected**: Success.

### 4.2 Combined Scenarios
*   **[P-18] Quick Re-entry**
    *   **Steps**: `exit` -> Proposal(Add) -> `register`.
    *   **Expected**: Success.
*   **[P-19] Role Change**
    *   **Steps**: `exit` -> delegate to others.
    *   **Expected**: Success.
*   **[P-20] Redemption Path**
    *   **Steps**: Slash/Jail -> Proposal(Add) -> `unjail`.
    *   **Expected**: Success.
*   **[P-21] Slash during Resign**
    *   **Steps**: Submit double-sign after `resign`.
    *   **Expected**: Successful Slash.
*   **[P-22] Slash after Exit**
    *   **Steps**: Submit double-sign after `exit`.
    *   **Expected**: Revert "Validator not exist".
*   **[P-26] Last-Effective Slash Floor**
    *   **Steps**:
        1. Reduce validator set to only one effective validator.
        2. Run low-level `slashValidator` simulation (`from=PUNISH_ADDR`) to read `(actualSlash, actualReward)`.
        3. Submit real double-sign evidence.
    *   **Expected**:
        *   `actualSlash` is capped by `selfStake - minValidatorStake`.
        *   On-chain `selfStake` never falls below `minValidatorStake`.
        *   Last effective validator remains in effective-top set and chain keeps producing blocks.

### 4.3 Exceptions & Boundaries
*   **[P-03] Forced Exit during Activity**
    *   **Expected**: Revert "Cannot exit: validator is in active set...".
*   **[P-04] Last Man Standing**
    *   **Expected**: Revert "Cannot remove: must keep at least one...".
*   **[P-05] Non-Validator Exit**
    *   **Expected**: Revert "Validator not registered".
*   **[P-06] Double Resign**
    *   **Expected**: Revert "Validator already resigned or jailed".
*   **[P-09] Miner Only Punish**
    *   **Expected**: Revert "Miner only".
*   **[P-10~P-14] Evidence Exceptions**
    *   **Expected**: Revert (Expired, Future, Malformed, Non-Validator, Duplicate).
*   **[P-15~P-17] Withdrawal Exceptions**
    *   **Expected**: Revert (Frequency, Zero profit, Non-Fee address).

### 4.4 Punish Engine & Consensus Hooks
*   **[P-23] Punish Normal Path**
    *   **Steps**: Miner calls `punish(val)` once on non-epoch block.
    *   **Expected**:
        *   `missedBlocksCounter` increments; validator appears in punish list.
        *   Runtime punish resolution uses the current effective signer.
*   **[P-24] Execute Pending (Consensus-Only)**
    *   **Steps**:
        1. Try to call `executePending(1)` via normal external transaction.
        2. Create epoch punish pending entry.
        3. Observe pending queue consumed on following non-epoch blocks without sending `executePending` tx.
    *   **Expected**:
        *   External tx is rejected with `forbidden system transaction`.
        *   Pending entry is auto-executed by consensus path.
*   **[P-25] Decrease Missed Blocks Counter (Epoch)**
    *   **Steps**: Miner calls `decreaseMissedBlocksCounter(epoch)` on epoch block.
    *   **Expected**: Counter decreases (or no-op if list empty), event emitted.
*   **[P-27] Checkpoint Runtime Hooks Still Use Old Signer**
    *   **Steps**:
        1. Schedule a signer rotation for a top validator.
        2. Reach the checkpoint block.
        3. Execute punish/runtime hooks that resolve validator-by-signer on the checkpoint block.
    *   **Expected**:
        *   The checkpoint block still resolves the validator through the old runtime signer.
        *   The transition signer set does not leak into punish/runtime routing on the checkpoint block.
*   **[P-28] Pending Signer Cleanup Before Activation**
    *   **Steps**:
        1. Schedule a pending signer rotation.
        2. Before activation, resign/remove/exit the validator.
        3. Query `getTopSignersForEpochTransition()` around the next checkpoint.
    *   **Expected**:
        *   The transition signer query does not expose a stale signer.
        *   The following epoch does not retain a dirty signer entry.

---

## 5. Validators & Consensus Reward Hooks

### 5.1 Rewards & Fees
*   **[V-03] Distribute Block Reward**
    *   **Steps**: Miner calls `Validators.distributeBlockReward()` with non-zero value.
    *   **Expected**:
        *   Fee income increases for the validator resolved from the current runtime signer (or redistributes if jailed / below minimum stake).
        *   On the checkpoint block, routing still uses the old signer even though `header.Extra` already contains the transition signer set.
*   **[S-22] Distribute Rewards & Claim Cooldown**
    *   **Steps**: Miner calls `Staking.distributeRewards()` -> validator claims -> distribute again -> claim before cooldown.
    *   **Expected**:
        *   First claim success; second claim reverts due to cooldown.
        *   On the checkpoint block, `Staking.distributeRewards()` still resolves the validator through the old runtime signer, not the transition signer set.
*   **[V-04] Withdraw Profits Exceptions**
    *   **Steps**: Non-fee caller withdraws; fee caller withdraws with zero profit.
    *   **Expected**: Revert "You are not the fee receiver..." / "You don't have any profits".

### 5.2 Validator Set Updates (Consensus)
*   **[V-07] Update Active Validator Set (Non-Epoch)**
    *   **Steps**: Miner calls `updateActiveValidatorSet` on non-epoch block.
    *   **Expected**: Revert "Block epoch only".
*   **[V-08] Update Active Validator Set (Epoch)**
    *   **Steps**: Miner calls `updateActiveValidatorSet` on epoch block with expected set.
    *   **Expected**: `currentValidatorSet` updated.
*   **[V-09] Runtime vs Transition Signer Query Split**
    *   **Steps**:
        1. Query `getTopSigners()` and `getTopSignersForEpochTransition()` before the checkpoint.
        2. Query both again on the checkpoint block after a signer rotation has been scheduled.
        3. Query both again on block `N+1`.
    *   **Expected**:
        *   Before the checkpoint, both queries return the old signer set.
        *   On the checkpoint block, `getTopSigners()` still returns the old signer set while `getTopSignersForEpochTransition()` returns the new signer set.
        *   On block `N+1`, both queries return the new signer set.
*   **[V-10] Checkpoint Header Extra Uses Transition Set**
    *   **Steps**:
        1. Reach a checkpoint block with a pending signer rotation.
        2. Read `header.Extra`.
        3. Compare it with the parent-state result of `getTopSignersForEpochTransition()`.
    *   **Expected**:
        *   `header.Extra` matches `getTopSignersForEpochTransition()`.
        *   The test does not rely on any projected-header / simulated-next-block derivation.
*   **[V-11] Block N+1 Accepts Only New Signer**
    *   **Steps**:
        1. Finish the checkpoint block that commits the transition signer set.
        2. Attempt block production / signer validation on block `N+1` with both the old signer and the new signer.
    *   **Expected**:
        *   Only the new signer is accepted by snapshot/consensus on block `N+1`.
        *   The old signer is no longer treated as valid for the next epoch.

### 5.3 Validator Info Validation & Queries
*   **[V-02] Description Boundary (Moniker)**
    *   **Steps**: `createOrEditValidator` with moniker > 70 bytes.
    *   **Expected**: Revert "Invalid moniker length".
*   **[V-02b] Description Boundary (Identity/Website/Email/Details)**
    *   **Steps**: Exceed max lengths (identity 3000, website 140, email 140, details 280).
    *   **Expected**: Revert with corresponding "Invalid ... length".
*   **[V-05] Reward-Eligible / Effective-Top / Signer Queries**
    *   **Steps**:
        1. Call `getActiveValidatorsWithStakes`, `getRewardEligibleValidatorsWithStakes`.
        2. Call `getEffectiveTopValidators`, `getEffectiveTopValidatorCount`, `isLastEffectiveValidator`.
        3. Call `getTopSigners`, `getTopSignersForEpochTransition`.
        4. Jail one validator and re-query reward-eligible list.
    *   **Expected**:
        *   Validators/stakes array lengths match.
        *   Effective-top count equals returned list length.
        *   Jailed validator is excluded from reward-eligible list immediately.
        *   Outside checkpoint boundaries, `getTopSigners()` and `getTopSignersForEpochTransition()` match.
        *   At checkpoint boundaries, runtime and transition signer queries may intentionally diverge.
*   **[V-06] Genesis POSA First-Block Reward Path**
    *   **Steps**:
        1. Start multi-node network with `GENESIS_MODE=posa`.
        2. Ensure fork switches are enabled from genesis (`shanghai/cancun/posa/fixHeader = 0`).
        3. Observe chain height passes block `1` and continues growing.
    *   **Expected**:
        *   Network progresses past the first post-genesis block.
        *   Reward eligibility path does not stall block production during initial POSA startup.
        *   Bootstrap self-stake is funded by deducting from validator cold-address balances, not by pre-funding the `Staking` contract.

---

## 6. Upgrade / Init Security
*   **[U-01] External Initialize Selector Interception**
    *   **Steps**: Send normal external tx to system contract `initialize*` selectors.
    *   **Expected**: Tx rejected at txpool/consensus boundary with `forbidden system transaction`.
*   **[U-02] External ReinitializeV2 Interception**
    *   **Steps**: Send normal external tx to `reinitializeV2` on Proposal/Validators/Staking/Punish.
    *   **Expected**: Tx rejected with `forbidden system transaction`.
*   **[U-03] Fixed Dependency Address Validation (Contract Layer)**
    *   **Steps**: Deploy fresh contract instances to non-system addresses; call `initialize*` with wrong dependency addresses.
    *   **Expected**: Revert with explicit fixed-address errors, e.g. `Invalid ... contract address`.
*   **[U-04] Punish Staking Parameter Completeness**
    *   **Steps**: Call fresh `Punish.initialize` with `staking_ = address(0)`.
    *   **Expected**: Revert `Invalid staking address`.
*   **[U-05] Epoch Transition Query Does Not Regress Migration**
    *   **Steps**:
        1. Run fresh PoSA startup.
        2. Run the PoA->PoSA epoch-boundary upgrade path.
        3. Verify checkpoint signer selection around the upgrade boundary.
    *   **Expected**:
        *   Fresh PoSA uses `getTopSignersForEpochTransition()` semantics correctly.
        *   PoA->PoSA migration behavior is unchanged for its special transition path.
*   **[U-06] CLI Override Bootstrap Mapping**
    *   **Steps**:
        1. Start an upgrade-mode network with `--override.posaValidators` and `--override.posaSigners`.
        2. Ensure the generated runtime artifacts expose the same override values.
        3. Wait for PoSA activation and query `getValidatorBySigner()` / `getValidatorSigner()`.
    *   **Expected**:
        *   CLI override values propagate into native/docker runtime startup.
        *   After migration, signer-to-validator mapping follows the override set, not the genesis fallback mapping.
        *   The overridden validator shows initialized self-stake after migration.

---

## 7. Public Query APIs (Smoke)
*   **[Q-01] Core Query Coverage**
    *   **Steps**:
        1. Call view getters for proposal/pass/nonces, validator status, staking counts, unbonding counts, reward-eligible/effective-top APIs.
        2. Include `getTopSigners()` and `getTopSignersForEpochTransition()`.
        3. Repeat signer queries before checkpoint, on checkpoint, and on block `N+1`.
    *   **Expected**:
        *   No revert; basic invariants hold (lengths match, counts non-negative).
        *   Query semantics match the documented split: checkpoint block runtime signer remains old, transition signer is already new, and block `N+1` uses the new signer for both views.

**Notes**:
*   Ensure `config/test_env.yaml` is configured and `data/test_config.yaml` has been generated before execution.
*   Tests execute in order; prerequisite failure halts subsequent tests.
