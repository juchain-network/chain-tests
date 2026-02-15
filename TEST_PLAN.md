# JuChain Integration Test Plan

This document outlines the end-to-end integration test paths for JuChain system contracts. To ensure test efficiency, the execution order is crucial.

**Execution Strategy**:
1.  **Parameter Adjustment (Phase 0)**: First, use governance proposals to shorten system time parameters (e.g., unbonding period, jail period) to create suitable conditions for subsequent tests.
2.  **Core Functionalities (Phase 1)**: Execute the full flow of validator onboarding, staking, and delegation under the new parameters.
3.  **Boundaries & Exceptions (Phase 2)**: Interleave various exception scenario tests throughout the process.

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
    *   **Steps**: Miner calls `distributeRewards` -> validator claims -> distribute again -> claim before cooldown.
    *   **Expected**: First claim success; second claim reverts "Must wait withdrawProfitPeriod blocks between claims".

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
    *   **Expected**: Slash + Jail.
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
    *   **Expected**: `missedBlocksCounter` increments; validator appears in punish list.
*   **[P-24] Execute Pending (No-op)**
    *   **Steps**: Call `executePending(0)` and `executePending(1)` when queue empty.
    *   **Expected**: No revert; no state change.
*   **[P-25] Decrease Missed Blocks Counter (Epoch)**
    *   **Steps**: Miner calls `decreaseMissedBlocksCounter(epoch)` on epoch block.
    *   **Expected**: Counter decreases (or no-op if list empty), event emitted.

---

## 5. Validators & Consensus Reward Hooks

### 5.1 Rewards & Fees
*   **[V-03] Distribute Block Reward**
    *   **Steps**: Miner calls `Validators.distributeBlockReward()` with non-zero value.
    *   **Expected**: Fee income increases for active miner (or redistributes if jailed).
*   **[S-22] Distribute Rewards & Claim Cooldown**
    *   **Steps**: Miner calls `Staking.distributeRewards()` -> validator claims -> distribute again -> claim before cooldown.
    *   **Expected**: First claim success; second claim reverts due to cooldown.
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

### 5.3 Validator Info Validation & Queries
*   **[V-02] Description Boundary (Moniker)**
    *   **Steps**: `createOrEditValidator` with moniker > 70 bytes.
    *   **Expected**: Revert "Invalid moniker length".
*   **[V-02b] Description Boundary (Identity/Website/Email/Details)**
    *   **Steps**: Exceed max lengths (identity 3000, website 140, email 140, details 280).
    *   **Expected**: Revert with corresponding "Invalid ... length".
*   **[V-05] Active Validators With Stakes**
    *   **Steps**: Call `getActiveValidatorsWithStakes`.
    *   **Expected**: Array lengths match, values returned.

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

---

## 7. Public Query APIs (Smoke)
*   **[Q-01] Core Query Coverage**
    *   **Steps**: Call view getters for proposal/pass/nonces, validator status, staking counts, unbonding counts.
    *   **Expected**: No revert; basic invariants hold (lengths match, counts non-negative).

**Notes**:
*   Ensure `tools/ci/config.yaml` is correctly configured before execution.
*   Tests execute in order; prerequisite failure halts subsequent tests.
