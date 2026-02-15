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
	_, _ = ctx.Validators.IsValidatorActive(nil, valAddr)
	_, _ = ctx.Validators.IsValidatorJailed(nil, valAddr)
	_, _ = ctx.Validators.IsValidatorExist(nil, valAddr)

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
