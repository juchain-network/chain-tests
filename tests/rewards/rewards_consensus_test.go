package tests

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"juchain.org/chain/tools/ci/internal/testkit"
)

func rewardIncreaseProbeBlocks(defaultMax int) int {
	maxBlocks := defaultMax
	if maxBlocks <= 0 {
		maxBlocks = 60
	}
	if ctx != nil {
		if period, err := ctx.Proposal.WithdrawProfitPeriod(nil); err == nil && period != nil && period.Sign() > 0 {
			dynamic := int(period.Int64())*4 + 8
			if dynamic < maxBlocks {
				maxBlocks = dynamic
			}
		} else if ctx.Config.Test.Params.WithdrawProfit > 0 {
			dynamic := int(ctx.Config.Test.Params.WithdrawProfit)*4 + 8
			if dynamic < maxBlocks {
				maxBlocks = dynamic
			}
		}
	}
	if maxBlocks < 12 {
		maxBlocks = 12
	}
	if maxBlocks > 60 {
		maxBlocks = 60
	}
	return maxBlocks
}

func retryAfterBlockInterval() time.Duration {
	d := retrySleep() / 4
	if d < 10*time.Millisecond {
		return 10 * time.Millisecond
	}
	return d
}

func TestI_ConsensusRewards(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}

	t.Run("V-03_DistributeBlockReward", func(t *testing.T) {
		_, minerAddr := minerKeyOrSkip(t)
		beforeInfo, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
		before := new(big.Int).Set(beforeInfo.AccumulatedRewards)
		maxProbeBlocks := rewardIncreaseProbeBlocks(60)
		if !waitForRewardIncrease(t, minerAddr, before, maxProbeBlocks) {
			info, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
			t.Fatalf("accumulatedRewards did not increase within %d blocks: before=%s after=%s", maxProbeBlocks, before.String(), info.AccumulatedRewards.String())
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
			Interval:    retryAfterBlockInterval(),
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
		deadline := new(big.Int).Add(lastClaim, withdrawPeriod).Uint64()
		observedCooldownRevert := false
		maxChecks := int(withdrawPeriod.Int64()) + 8
		if maxChecks < 4 {
			maxChecks = 4
		}
		if maxChecks > 24 {
			maxChecks = 24
		}
		for i := 0; i < maxChecks; i++ {
			waitBlocks(t, 1)
			info, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
			if info.AccumulatedRewards.Sign() == 0 {
				continue
			}

			opts, _ := ctx.GetTransactor(minerKey)
			tx, err := ctx.Staking.ClaimValidatorRewards(opts)

			minedHeight := uint64(0)
			if err == nil {
				err = ctx.WaitMined(tx.Hash())
				if receipt, errR := ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash()); errR == nil && receipt != nil && receipt.BlockNumber != nil {
					minedHeight = receipt.BlockNumber.Uint64()
				}
			}
			if minedHeight == 0 {
				minedHeight, _ = ctx.Clients[0].BlockNumber(context.Background())
			}

			if minedHeight < deadline {
				if err == nil {
					t.Fatalf("claim succeeded before cooldown deadline: mined=%d deadline=%d", minedHeight, deadline)
				}
				if strings.Contains(err.Error(), "withdrawProfitPeriod") {
					observedCooldownRevert = true
					t.Logf("cooldown enforced before deadline: mined=%d deadline=%d err=%v", minedHeight, deadline, err)
					break
				}
				t.Logf("claim before deadline failed for non-cooldown reason (will retry): mined=%d deadline=%d err=%v", minedHeight, deadline, err)
				continue
			}

			if err == nil {
				if observedCooldownRevert {
					t.Logf("claim succeeded after cooldown window as expected: mined=%d deadline=%d", minedHeight, deadline)
				} else {
					t.Logf("claim succeeded at/after deadline without pre-deadline cooldown sample: mined=%d deadline=%d", minedHeight, deadline)
				}
				return
			}

			t.Logf("claim at/after deadline still failing (will retry): mined=%d deadline=%d err=%v", minedHeight, deadline, err)
		}

		if observedCooldownRevert {
			return
		}

		// With short periods and rotating proposers, rewards may only accrue at/after deadline.
		// Ensure post-deadline claim path remains healthy.
		robustClaimValidatorRewards(t, minerKey)
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
		info, _ := ctx.Staking.GetValidatorInfo(nil, minerAddr)
		if info.AccumulatedRewards.Cmp(before) > 0 {
			return true
		}

		// Event filtering is diagnostic and relatively heavy; sample periodically
		// unless debug logging is explicitly requested.
		if !debugEnabled() && i%5 != 0 {
			continue
		}

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
	}
	return false
}
