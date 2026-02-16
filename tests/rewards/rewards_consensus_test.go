package tests

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestI_ConsensusRewards(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}

	t.Run("V-03_DistributeBlockReward", func(t *testing.T) {
		_, minerAddr := minerKeyOrSkip(t)
		beforeInfo, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
		before := new(big.Int).Set(beforeInfo.AccumulatedRewards)
		if !waitForRewardIncrease(t, minerAddr, before, 60) {
			info, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
			t.Fatalf("accumulatedRewards did not increase within 60 blocks: before=%s after=%s", before.String(), info.AccumulatedRewards.String())
		}
		info, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
		t.Logf("Rewards increased for %s: %s -> %s", minerAddr.Hex(), before.String(), info.AccumulatedRewards.String())
	})

	t.Run("S-22_DistributeRewardsAndCooldown", func(t *testing.T) {
		minerKey, minerAddr := minerKeyOrSkip(t)
		withdrawPeriod, _ := ctx.Proposal.WithdrawProfitPeriod(nil)
		if withdrawPeriod == nil || withdrawPeriod.Sign() == 0 {
			t.Fatalf("withdrawProfitPeriod unavailable")
		}

		maxAccrual := int(withdrawPeriod.Int64())
		if maxAccrual < 5 {
			maxAccrual = 5
		}
		if maxAccrual > 200 {
			maxAccrual = 200
		}
		infoBefore, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
		if !waitForRewardIncrease(t, minerAddr, infoBefore.AccumulatedRewards, maxAccrual) {
			infoNow, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
			t.Fatalf("no rewards accrued for validator in time: before=%s after=%s", infoBefore.AccumulatedRewards.String(), infoNow.AccumulatedRewards.String())
		}
		infoBefore, _ = ctx.Staking.GetValidatorInfo(nil, minerAddr)
		if infoBefore.AccumulatedRewards.Sign() == 0 {
			t.Fatalf("no rewards accrued for validator in time")
		}
		lastClaimBefore := big.NewInt(0)
		if infoBefore.LastClaimBlock != nil {
			lastClaimBefore = new(big.Int).Set(infoBefore.LastClaimBlock)
		}

		robustClaimValidatorRewards(t, minerKey)

		var infoAfterClaim struct {
			LastClaimBlock     *big.Int
			AccumulatedRewards *big.Int
		}
		err := testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 4,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			info, err := ctx.Staking.GetValidatorInfo(nil, minerAddr)
			if err != nil {
				return false, err
			}
			infoAfterClaim.LastClaimBlock = info.LastClaimBlock
			infoAfterClaim.AccumulatedRewards = info.AccumulatedRewards
			if info.LastClaimBlock == nil {
				return false, nil
			}
			return info.LastClaimBlock.Cmp(lastClaimBefore) > 0, nil
		})
		if err != nil {
			t.Fatalf("claim did not advance lastClaimBlock: before=%s after=%v rewards=%v err=%v", lastClaimBefore.String(), infoAfterClaim.LastClaimBlock, infoAfterClaim.AccumulatedRewards, err)
		}
		lastClaim := new(big.Int).Set(infoAfterClaim.LastClaimBlock)

		// Try to claim again within cooldown after rewards accumulate.
		deadline := new(big.Int).Add(lastClaim, withdrawPeriod)
		cooldownChecks := int(withdrawPeriod.Int64()) - 1
		if cooldownChecks < 1 {
			cooldownChecks = 1
		}
		if cooldownChecks > 50 {
			cooldownChecks = 50
		}
		for i := 0; i < cooldownChecks; i++ {
			waitBlocks(t, 1)
			curHeight, _ := ctx.Clients[0].BlockNumber(context.Background())
			if curHeight >= deadline.Uint64() {
				break
			}
			info, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
			if info.AccumulatedRewards.Sign() == 0 {
				continue
			}

			opts, _ := ctx.GetTransactor(minerKey)
			tx, err := ctx.Staking.ClaimValidatorRewards(opts)
			if err == nil {
				err = ctx.WaitMined(tx.Hash())
			}
			if err == nil {
				t.Fatalf("expected cooldown revert, got success")
			}
			if !strings.Contains(err.Error(), "withdrawProfitPeriod") {
				t.Logf("claim failed as expected (cooldown), err=%v", err)
			}
			return
		}

		t.Fatalf("no rewards accrued within cooldown window")
	})

	t.Run("QueryRewards", func(t *testing.T) {
		valAddr := ctx.Config.Validators[0].Address
		_, _, _, _, rew, _ := ctx.Validators.GetValidatorInfo(nil, common.HexToAddress(valAddr))
		t.Logf("Validator %s rewards: %s", valAddr, rew.String())
	})
}

func waitForRewardIncrease(t *testing.T, minerAddr common.Address, before *big.Int, maxBlocks int) bool {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	start, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("failed to read block number: %v", err)
	}
	infoNow, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
	if infoNow.AccumulatedRewards.Cmp(before) > 0 {
		return true
	}
	nextStart := start
	for i := 0; i < maxBlocks; i++ {
		waitBlocks(t, 1)
		end, err := ctx.Clients[0].BlockNumber(context.Background())
		if err != nil {
			continue
		}
		if end < nextStart {
			nextStart = end
		}
		iter, err := ctx.Staking.FilterRewardsDistributed(&bind.FilterOpts{
			Start: nextStart,
			End:   &end,
		}, []common.Address{minerAddr})
		if err == nil {
			if iter.Next() {
				ev := iter.Event
				_ = iter.Close()
				if ev != nil {
					t.Logf("RewardsDistributed: validator=%s amount=%s block=%d", ev.Validator.Hex(), ev.Amount.String(), ev.Raw.BlockNumber)
				}
			}
			if err := iter.Error(); err != nil {
				t.Logf("reward log filter error: %v", err)
			}
			_ = iter.Close()
		} else {
			t.Logf("reward log filter failed: %v", err)
		}
		if end >= nextStart {
			nextStart = end + 1
		}
		info, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
		if info.AccumulatedRewards.Cmp(before) > 0 {
			return true
		}
	}
	return false
}
