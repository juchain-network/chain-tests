package tests

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestI_PublicQueryCoverage(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	valAddr := common.HexToAddress(ctx.Config.Validators[0].Address)

	// Validators query APIs
	active, _ := ctx.Validators.GetActiveValidators(nil)
	activeSigners, err := ctx.Validators.GetActiveSigners(nil)
	utils.AssertNoError(t, err, "getActiveSigners failed")
	info, err := ctx.Validators.GetActiveValidatorsWithStakes(nil)
	utils.AssertNoError(t, err, "getActiveValidatorsWithStakes failed")
	if len(info.Validators) != len(info.TotalStakes) {
		t.Fatalf("validators/stakes length mismatch: %d vs %d", len(info.Validators), len(info.TotalStakes))
	}
	count, _ := ctx.Validators.GetActiveValidatorCount(nil)
	if count != nil && int(count.Uint64()) != len(active) {
		t.Logf("active count mismatch: count=%d list=%d", count.Uint64(), len(active))
	}
	_, _ = ctx.Validators.GetTopValidators(nil)
	_, _ = ctx.Validators.GetHighestValidators(nil)
	runtimeSigners, err := ctx.Validators.GetTopSigners(nil)
	utils.AssertNoError(t, err, "getTopSigners failed")
	transitionSigners, err := ctx.Validators.GetTopSignersForEpochTransition(nil)
	utils.AssertNoError(t, err, "getTopSignersForEpochTransition failed")
	if len(runtimeSigners) == 0 {
		t.Fatalf("runtime signer set is empty")
	}
	if len(transitionSigners) == 0 {
		t.Fatalf("transition signer set is empty")
	}
	if len(activeSigners) == 0 {
		t.Fatalf("active signer set is empty")
	}
	effectiveTop, err := ctx.Validators.GetEffectiveTopValidators(nil)
	utils.AssertNoError(t, err, "getEffectiveTopValidators failed")
	effectiveCount, err := ctx.Validators.GetEffectiveTopValidatorCount(nil)
	utils.AssertNoError(t, err, "getEffectiveTopValidatorCount failed")
	if effectiveCount != nil && int(effectiveCount.Uint64()) != len(effectiveTop) {
		t.Fatalf("effective count mismatch: count=%d list=%d", effectiveCount.Uint64(), len(effectiveTop))
	}
	rewardEligible, err := ctx.Validators.GetRewardEligibleValidatorsWithStakes(nil)
	utils.AssertNoError(t, err, "getRewardEligibleValidatorsWithStakes failed")
	if len(rewardEligible.Validators) != len(rewardEligible.TotalStakes) {
		t.Fatalf("reward-eligible validators/stakes length mismatch: %d vs %d", len(rewardEligible.Validators), len(rewardEligible.TotalStakes))
	}
	minStake, err := ctx.Proposal.MinValidatorStake(nil)
	utils.AssertNoError(t, err, "read minValidatorStake failed")
	isLastEffective, err := ctx.Validators.IsLastEffectiveValidator(nil, valAddr)
	utils.AssertNoError(t, err, "isLastEffectiveValidator failed")
	if len(effectiveTop) == 1 {
		want := effectiveTop[0] == valAddr
		if isLastEffective != want {
			t.Fatalf("isLastEffectiveValidator mismatch: addr=%s got=%v want=%v", valAddr.Hex(), isLastEffective, want)
		}
	} else if isLastEffective {
		t.Fatalf("isLastEffectiveValidator should be false when effective top count=%d", len(effectiveTop))
	}
	for _, addr := range rewardEligible.Validators {
		if !containsAddress(active, addr) {
			t.Fatalf("reward-eligible validator %s not found in active set", addr.Hex())
		}
		info, err := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, err, "read reward-eligible validator info failed")
		if info.SelfStake == nil || info.SelfStake.Cmp(minStake) < 0 {
			t.Fatalf("reward-eligible validator %s self stake below minValidatorStake: self=%v min=%s", addr.Hex(), info.SelfStake, minStake.String())
		}
	}
	_, _ = ctx.Validators.IsValidatorActive(nil, valAddr)
	_, _ = ctx.Validators.IsValidatorJailed(nil, valAddr)
	_, _ = ctx.Validators.IsValidatorExist(nil, valAddr)
	signerAddr, err := ctx.Validators.GetValidatorSigner(nil, valAddr)
	utils.AssertNoError(t, err, "getValidatorSigner failed")
	if signerAddr == (common.Address{}) {
		t.Fatalf("validator signer is zero for %s", valAddr.Hex())
	}
	mappedValidator, err := ctx.Validators.GetValidatorBySigner(nil, signerAddr)
	utils.AssertNoError(t, err, "getValidatorBySigner failed")
	if mappedValidator != valAddr {
		t.Fatalf("validator/signer mapping mismatch: signer=%s got=%s want=%s", signerAddr.Hex(), mappedValidator.Hex(), valAddr.Hex())
	}

	// Staking query APIs
	_, _ = ctx.Staking.GetValidatorStatus(nil, valAddr)
	totalCount, _ := ctx.Staking.GetValidatorCount(nil)
	if totalCount != nil && count != nil && totalCount.Cmp(count) < 0 {
		t.Logf("validator count < active count: %s < %s", totalCount, count)
	}
	_, _ = ctx.Staking.GetDelegationInfo(nil, valAddr, valAddr)
	_, _ = ctx.Staking.GetUnbondingEntriesCount(nil, valAddr, valAddr)
	_, _ = ctx.Staking.GetUnbondingEntries(nil, valAddr, valAddr)

	// Proposal query APIs
	_, _ = ctx.Proposal.IsProposalValidForStaking(nil, valAddr)
	_, _ = ctx.Proposal.Pass(nil, valAddr)
	_, _ = ctx.Proposal.ProposerNonces(nil, valAddr)
}

func containsAddress(addrs []common.Address, target common.Address) bool {
	for _, addr := range addrs {
		if addr == target {
			return true
		}
	}
	return false
}
