package tests

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestD_StakingScenarios(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	valKey := ctx.GenesisValidators[0]
	valAddr := common.HexToAddress(ctx.Config.Validators[0].Address)

	// [S-17] Stake Jitter (Frequent changes & Rewards)
	t.Run("S-17_StakeJitter", func(t *testing.T) {
		// 1. Initial State
		info1, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)

		// 2. Add Stake
		addAmt := utils.ToWei(100)
		addOpts, _ := ctx.GetTransactor(valKey)
		addOpts.Value = addAmt
		tx1, err := ctx.Staking.AddValidatorStake(addOpts)
		utils.AssertNoError(t, err, "S-17 add stake failed")
		ctx.WaitMined(tx1.Hash())

		// 3. Wait blocks to accumulate rewards
		waitBlocks(t, 1)

		// 4. Decrease Stake
		decAmt := utils.ToWei(50)
		decOpts, _ := ctx.GetTransactor(valKey)
		decOpts.Value = nil
		tx2, err := ctx.Staking.DecreaseValidatorStake(decOpts, decAmt)
		utils.AssertNoError(t, err, "S-17 decrease stake failed")
		ctx.WaitMined(tx2.Hash())

		// 5. Verify Rewards Accumulation
		// Rewards should be settled on each stake change
		info2, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		utils.AssertTrue(t, info2.AccumulatedRewards.Cmp(info1.AccumulatedRewards) >= 0, "Rewards should accumulate")
	})

	// [S-18] Mixed Stakes (Self-Stake + Delegation)
	t.Run("S-18_MixedStakes", func(t *testing.T) {
		// 1. Setup Delegator
		userKey, _, _ := ctx.CreateAndFundAccount(utils.ToWei(200))

		// 2. Delegate to Validator
		delAmt := utils.ToWei(50)
		uOpts1, _ := ctx.GetTransactor(userKey)
		uOpts1.Value = delAmt
		tx1, err := ctx.Staking.Delegate(uOpts1, valAddr)
		utils.AssertNoError(t, err, "S-18 delegate failed")
		ctx.WaitMined(tx1.Hash())

		// 3. Validator Increases Self Stake
		vAddAmt := utils.ToWei(50)
		vAddOpts, _ := ctx.GetTransactor(valKey)
		vAddOpts.Value = vAddAmt
		tx2, err := ctx.Staking.AddValidatorStake(vAddOpts)
		utils.AssertNoError(t, err, "S-18 val add stake failed")
		ctx.WaitMined(tx2.Hash())

		// 4. Check Total Stake
		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		// Total = Self + Delegated
		// We need to fetch exact values because other tests might have changed state
		// But we know TotalDelegated should at least be delAmt
		utils.AssertTrue(t, info.TotalDelegated.Cmp(delAmt) >= 0, "TotalDelegated mismatch")

		// 5. Validator Decreases Self Stake
		vDecAmt := utils.ToWei(20)
		vDecOpts, _ := ctx.GetTransactor(valKey)
		vDecOpts.Value = nil
		tx3, err := ctx.Staking.DecreaseValidatorStake(vDecOpts, vDecAmt)
		utils.AssertNoError(t, err, "S-18 val decrease stake failed")
		ctx.WaitMined(tx3.Hash())

		// 6. Delegator Increases Delegation
		uOpts2, _ := ctx.GetTransactor(userKey)
		uOpts2.Value = utils.ToWei(10)
		tx4, err := ctx.Staking.Delegate(uOpts2, valAddr)
		utils.AssertNoError(t, err, "S-18 delegate more failed")
		ctx.WaitMined(tx4.Hash())

		// 7. Final Check
		finalInfo, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		utils.AssertTrue(t, finalInfo.TotalDelegated.Cmp(utils.ToWei(60)) >= 0, "Final delegated amount mismatch")
	})
}
