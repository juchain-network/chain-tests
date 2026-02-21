package tests

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestD_StakingScenarios(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	valKey := ctx.GenesisValidators[0]
	valAddr := common.HexToAddress(ctx.Config.Validators[0].Address)
	isRetryableStakeErr := func(err error) bool {
		if err == nil {
			return false
		}
		msg := strings.ToLower(err.Error())
		return strings.Contains(msg, "epoch block forbidden") ||
			strings.Contains(msg, "nonce") ||
			strings.Contains(msg, "timeout waiting for tx") ||
			strings.Contains(msg, "already known") ||
			strings.Contains(msg, "underpriced") ||
			strings.Contains(msg, "revert")
	}
	robustAddValidatorStake := func(t *testing.T, key *ecdsa.PrivateKey, amount *big.Int, failMsg string) {
		var lastErr error
		for attempt := 0; attempt < 8; attempt++ {
			opts, err := ctx.GetTransactor(key)
			if err == nil {
				opts.Value = amount
				tx, errTx := ctx.Staking.AddValidatorStake(opts)
				if errTx == nil {
					if errW := ctx.WaitMined(tx.Hash()); errW == nil {
						return
					} else {
						errTx = errW
					}
				}
				err = errTx
			}
			lastErr = err
			if !isRetryableStakeErr(err) {
				utils.AssertNoError(t, err, failMsg)
				return
			}
			waitNextBlock()
		}
		utils.AssertNoError(t, lastErr, failMsg)
	}
	robustDecreaseValidatorStake := func(t *testing.T, key *ecdsa.PrivateKey, amount *big.Int, failMsg string) {
		var lastErr error
		for attempt := 0; attempt < 8; attempt++ {
			opts, err := ctx.GetTransactor(key)
			if err == nil {
				opts.Value = nil
				tx, errTx := ctx.Staking.DecreaseValidatorStake(opts, amount)
				if errTx == nil {
					if errW := ctx.WaitMined(tx.Hash()); errW == nil {
						return
					} else {
						errTx = errW
					}
				}
				err = errTx
			}
			lastErr = err
			if !isRetryableStakeErr(err) {
				utils.AssertNoError(t, err, failMsg)
				return
			}
			waitNextBlock()
		}
		utils.AssertNoError(t, lastErr, failMsg)
	}

	// [S-17] Stake Jitter (Frequent changes & Rewards)
	t.Run("S-17_StakeJitter", func(t *testing.T) {
		// 1. Initial State
		info1, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)

		// 2. Add Stake
		addAmt := utils.ToWei(100)
		robustAddValidatorStake(t, valKey, addAmt, "S-17 add stake failed")

		// 4. Decrease Stake
		decAmt := utils.ToWei(50)
		robustDecreaseValidatorStake(t, valKey, decAmt, "S-17 decrease stake failed")

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
		robustDelegate(t, userKey, valAddr, delAmt)

		// 3. Validator Increases Self Stake
		vAddAmt := utils.ToWei(50)
		robustAddValidatorStake(t, valKey, vAddAmt, "S-18 val add stake failed")

		// 4. Check Total Stake
		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		// Total = Self + Delegated. Around epoch boundaries, delegated totals can lag
		// by a block; poll briefly before asserting.
		if info.TotalDelegated.Cmp(delAmt) < 0 {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 8,
				Interval:    retrySleep(),
				OnRetry: func(int) {
					waitNextBlock()
				},
			}, func() (bool, error) {
				latest, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
				if err != nil {
					return false, err
				}
				info = latest
				return latest.TotalDelegated.Cmp(delAmt) >= 0, nil
			})
		}
		utils.AssertTrue(t, info.TotalDelegated.Cmp(delAmt) >= 0, "TotalDelegated mismatch")

		// 5. Validator Decreases Self Stake
		vDecAmt := utils.ToWei(20)
		robustDecreaseValidatorStake(t, valKey, vDecAmt, "S-18 val decrease stake failed")

		// 6. Delegator Increases Delegation
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))

		// 7. Final Check
		finalInfo, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		if finalInfo.TotalDelegated.Cmp(utils.ToWei(60)) < 0 {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 8,
				Interval:    retrySleep(),
				OnRetry: func(int) {
					waitNextBlock()
				},
			}, func() (bool, error) {
				latest, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
				if err != nil {
					return false, err
				}
				finalInfo = latest
				return latest.TotalDelegated.Cmp(utils.ToWei(60)) >= 0, nil
			})
		}
		utils.AssertTrue(t, finalInfo.TotalDelegated.Cmp(utils.ToWei(60)) >= 0, "Final delegated amount mismatch")
	})
}
