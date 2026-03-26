package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
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

	t.Run("V-01_JailedExcludedFromRewardEligibleImmediately", func(t *testing.T) {
		active, err := ctx.Validators.GetActiveValidators(nil)
		if err != nil {
			t.Fatalf("failed to query active validators: %v", err)
		}
		if len(active) < 2 {
			t.Skip("skip immediate jailed exclusion test: need at least 2 active validators")
		}

		var (
			targetAddr       common.Address
			targetSignerAddr common.Address
			targetSignerKey  *ecdsa.PrivateKey
		)
		for _, addr := range active {
			coldKey := keyForAddress(addr)
			signerAddr, signerKey := signerIdentityForValidator(addr, coldKey)
			if signerKey == nil {
				continue
			}
			info, err := ctx.Staking.GetValidatorInfo(nil, addr)
			if err != nil || info.IsJailed {
				continue
			}
			targetAddr = addr
			targetSignerAddr = signerAddr
			targetSignerKey = signerKey
			break
		}
		if targetSignerKey == nil || targetAddr == (common.Address{}) || targetSignerAddr == (common.Address{}) {
			t.Skip("skip immediate jailed exclusion test: no eligible validator signer target")
		}

		eligibleBefore, err := ctx.Validators.GetRewardEligibleValidatorsWithStakes(nil)
		if err != nil {
			t.Fatalf("failed to query reward-eligible validators before jail: %v", err)
		}
		if !containsAddress(eligibleBefore.Validators, targetAddr) {
			t.Skipf("target %s already absent from reward-eligible set before jail", targetAddr.Hex())
		}

		doubleSignWindow, _ := ctx.Proposal.DoubleSignWindow(nil)
		targetWindow := big.NewInt(8)
		if doubleSignWindow == nil || doubleSignWindow.Cmp(targetWindow) > 0 {
			_ = ctx.EnsureConfig(15, targetWindow, doubleSignWindow)
		}

		reporterKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(5))
		if err != nil {
			t.Fatalf("failed to setup reporter: %v", err)
		}
		if err := submitDoubleSignEvidenceForRewardEligibility(targetAddr, targetSignerAddr, targetSignerKey, reporterKey); err != nil {
			t.Fatalf("failed to submit double-sign evidence: %v", err)
		}

		err = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 8,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			info, err := ctx.Staking.GetValidatorInfo(nil, targetAddr)
			if err != nil {
				return false, err
			}
			return info.IsJailed, nil
		})
		if err != nil {
			t.Fatalf("target validator was not jailed in time: %v", err)
		}

		eligibleAfterAddrs := append([]common.Address(nil), eligibleBefore.Validators...)
		err = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 8,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			eligibleNow, err := ctx.Validators.GetRewardEligibleValidatorsWithStakes(nil)
			if err != nil {
				return false, err
			}
			eligibleAfterAddrs = append(eligibleAfterAddrs[:0], eligibleNow.Validators...)
			return !containsAddress(eligibleNow.Validators, targetAddr), nil
		})
		if err != nil {
			t.Fatalf("jailed validator %s still present in reward-eligible set: %v", targetAddr.Hex(), err)
		}

		activeNow, _ := ctx.Validators.GetActiveValidators(nil)
		if containsAddress(activeNow, targetAddr) {
			t.Logf("validator %s still in active set but excluded from reward-eligible set as expected", targetAddr.Hex())
		} else {
			t.Logf("validator %s removed from both active set and reward-eligible set", targetAddr.Hex())
		}

		if len(eligibleAfterAddrs) == 0 {
			t.Fatalf(
				"reward-eligible set became empty after jailed exclusion: target=%s active=%s eligible=%s",
				targetAddr.Hex(),
				formatAddressList(activeNow),
				formatAddressList(eligibleAfterAddrs),
			)
		}

		beforeSnapshots, err := snapshotAccumulatedRewards(eligibleAfterAddrs)
		if err != nil {
			t.Fatalf("failed to snapshot reward-eligible validator rewards: %v", err)
		}

		probeBlocks := rewardObservationProbeBlocks(len(eligibleAfterAddrs))
		rewardedAddr, afterSnapshots, ok := waitForAnyRewardIncrease(t, eligibleAfterAddrs, beforeSnapshots, probeBlocks)
		if !ok {
			t.Fatalf(
				"rewards did not progress after jailed exclusion within %d blocks: target=%s active=%s eligible=%s latestCoinbase=%s before={%s} after={%s}",
				probeBlocks,
				targetAddr.Hex(),
				formatAddressList(activeNow),
				formatAddressList(eligibleAfterAddrs),
				currentCoinbaseHex(),
				formatRewardSnapshots(eligibleAfterAddrs, beforeSnapshots),
				formatRewardSnapshots(eligibleAfterAddrs, afterSnapshots),
			)
		}
		t.Logf(
			"rewards continued after jailed exclusion via %s: %s -> %s",
			rewardedAddr.Hex(),
			rewardValueFor(beforeSnapshots, rewardedAddr).String(),
			rewardValueFor(afterSnapshots, rewardedAddr).String(),
		)
	})

	t.Run("QueryRewards", func(t *testing.T) {
		valAddr := ctx.Config.Validators[0].Address
		_, _, _, _, rew, _ := ctx.Validators.GetValidatorInfo(nil, common.HexToAddress(valAddr))
		t.Logf("Validator %s rewards: %s", valAddr, rew.String())
	})
}

func submitDoubleSignEvidenceForRewardEligibility(
	validatorAddr common.Address,
	signerAddr common.Address,
	signerKey *ecdsa.PrivateKey,
	reporterKey *ecdsa.PrivateKey,
) error {
	if ctx == nil {
		return fmt.Errorf("context not initialized")
	}
	if signerKey == nil || reporterKey == nil {
		return fmt.Errorf("missing signer or reporter key")
	}
	if signerAddr == (common.Address{}) || validatorAddr == (common.Address{}) {
		return fmt.Errorf("missing validator or signer address")
	}

	reporterAddr := crypto.PubkeyToAddress(reporterKey.PublicKey)
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		ctx.WaitIfEpochBlock()

		head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || head == nil {
			lastErr = fmt.Errorf("read latest header failed: %w", err)
			waitBlocks(nil, 1)
			continue
		}

		targetHeight := new(big.Int).Sub(head.Number, big.NewInt(1))
		if targetHeight.Sign() <= 0 {
			targetHeight = big.NewInt(1)
		}
		baseTime := head.Time
		if h, err := ctx.Clients[0].HeaderByNumber(context.Background(), targetHeight); err == nil && h != nil {
			baseTime = h.Time
		}

		h1 := &types.Header{
			ParentHash:  common.Hash{},
			UncleHash:   types.EmptyUncleHash,
			Coinbase:    signerAddr,
			Root:        common.Hash{},
			TxHash:      types.EmptyRootHash,
			ReceiptHash: types.EmptyRootHash,
			Bloom:       types.Bloom{},
			Difficulty:  big.NewInt(1),
			Number:      targetHeight,
			GasLimit:    30_000_000,
			GasUsed:     0,
			Time:        baseTime,
			Extra:       make([]byte, 32+65),
			MixDigest:   common.Hash{},
			Nonce:       types.BlockNonce{},
		}
		h2 := &types.Header{
			ParentHash:  common.Hash{},
			UncleHash:   types.EmptyUncleHash,
			Coinbase:    signerAddr,
			Root:        common.Hash{0x01},
			TxHash:      types.EmptyRootHash,
			ReceiptHash: types.EmptyRootHash,
			Bloom:       types.Bloom{},
			Difficulty:  big.NewInt(1),
			Number:      targetHeight,
			GasLimit:    30_000_000,
			GasUsed:     0,
			Time:        baseTime,
			Extra:       make([]byte, 32+65),
			MixDigest:   common.Hash{},
			Nonce:       types.BlockNonce{},
		}

		rlp1, err := signHeaderCliqueForRewards(h1, signerKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to sign header 1: %w", err)
			waitBlocks(nil, 1)
			continue
		}
		rlp2, err := signHeaderCliqueForRewards(h2, signerKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to sign header 2: %w", err)
			waitBlocks(nil, 1)
			continue
		}

		opts, err := ctx.GetTransactor(reporterKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to build reporter transactor: %w", err)
			waitBlocks(nil, 1)
			continue
		}

		tx, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
		if err != nil {
			lastErr = err
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "epoch block forbidden") ||
				strings.Contains(msg, "nonce too low") ||
				strings.Contains(msg, "replacement transaction underpriced") ||
				strings.Contains(msg, "already known") ||
				strings.Contains(msg, "revert") {
				ctx.RefreshNonce(reporterAddr)
				waitBlocks(nil, 1)
				continue
			}
			return err
		}

		if err := ctx.WaitMined(tx.Hash()); err != nil {
			lastErr = err
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "reverted") ||
				strings.Contains(msg, "epoch block forbidden") ||
				strings.Contains(msg, "timeout waiting for tx") {
				ctx.RefreshNonce(reporterAddr)
				waitBlocks(nil, 1)
				continue
			}
			return err
		}

		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("double-sign evidence retries exhausted")
	}
	return lastErr
}

func signHeaderCliqueForRewards(h *types.Header, key *ecdsa.PrivateKey) ([]byte, error) {
	origExtra := h.Extra
	if len(origExtra) < 65 {
		h.Extra = make([]byte, 65)
	}
	headerForHash := *h
	extraCopy := make([]byte, len(h.Extra)-65)
	copy(extraCopy, h.Extra[:len(h.Extra)-65])
	headerForHash.Extra = extraCopy
	encodedForHash, err := rlp.EncodeToBytes(&headerForHash)
	if err != nil {
		return nil, err
	}
	hash := crypto.Keccak256(encodedForHash)
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		return nil, err
	}
	if len(h.Extra) < 65 {
		h.Extra = make([]byte, 32+65)
	}
	copy(h.Extra[len(h.Extra)-65:], sig)
	return rlp.EncodeToBytes(h)
}

func rewardObservationProbeBlocks(validatorCount int) int {
	base := 24
	if validatorCount > 0 {
		dynamic := validatorCount * 8
		if dynamic > base {
			base = dynamic
		}
	}
	return rewardIncreaseProbeBlocks(base)
}

func snapshotAccumulatedRewards(addrs []common.Address) (map[common.Address]*big.Int, error) {
	snapshots := make(map[common.Address]*big.Int, len(addrs))
	for _, addr := range addrs {
		info, err := ctx.Staking.GetValidatorInfo(nil, addr)
		if err != nil {
			return nil, fmt.Errorf("read validator %s rewards failed: %w", addr.Hex(), err)
		}
		rewards := big.NewInt(0)
		if info.AccumulatedRewards != nil {
			rewards = new(big.Int).Set(info.AccumulatedRewards)
		}
		snapshots[addr] = rewards
	}
	return snapshots, nil
}

func rewardValueFor(snapshots map[common.Address]*big.Int, addr common.Address) *big.Int {
	if snapshots == nil {
		return big.NewInt(0)
	}
	if value, ok := snapshots[addr]; ok && value != nil {
		return value
	}
	return big.NewInt(0)
}

func findRewardIncrease(addrs []common.Address, before map[common.Address]*big.Int, after map[common.Address]*big.Int) (common.Address, bool) {
	for _, addr := range addrs {
		if rewardValueFor(after, addr).Cmp(rewardValueFor(before, addr)) > 0 {
			return addr, true
		}
	}
	return common.Address{}, false
}

func waitForAnyRewardIncrease(t *testing.T, validatorAddrs []common.Address, before map[common.Address]*big.Int, maxBlocks int) (common.Address, map[common.Address]*big.Int, bool) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if len(validatorAddrs) == 0 {
		t.Fatalf("validatorAddrs is empty")
	}

	current, err := snapshotAccumulatedRewards(validatorAddrs)
	if err != nil {
		t.Fatalf("failed to snapshot current rewards: %v", err)
	}
	if addr, ok := findRewardIncrease(validatorAddrs, before, current); ok {
		return addr, current, true
	}

	nextStart, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("failed to read block number: %v", err)
	}
	for i := 0; i < maxBlocks; i++ {
		waitBlocks(t, 1)

		current, err = snapshotAccumulatedRewards(validatorAddrs)
		if err != nil {
			t.Fatalf("failed to snapshot rewards after waiting: %v", err)
		}
		if addr, ok := findRewardIncrease(validatorAddrs, before, current); ok {
			return addr, current, true
		}

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
		}, validatorAddrs)
		if err == nil {
			for iter.Next() {
				if ev := iter.Event; ev != nil {
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

	current, err = snapshotAccumulatedRewards(validatorAddrs)
	if err != nil {
		t.Fatalf("failed to snapshot final rewards: %v", err)
	}
	return common.Address{}, current, false
}

func formatAddressList(addrs []common.Address) string {
	if len(addrs) == 0 {
		return "<empty>"
	}
	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		parts = append(parts, addr.Hex())
	}
	return strings.Join(parts, ",")
}

func formatRewardSnapshots(addrs []common.Address, snapshots map[common.Address]*big.Int) string {
	if len(addrs) == 0 {
		return "<empty>"
	}
	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		parts = append(parts, fmt.Sprintf("%s=%s", addr.Hex(), rewardValueFor(snapshots, addr).String()))
	}
	return strings.Join(parts, ", ")
}

func currentCoinbaseHex() string {
	if ctx == nil || len(ctx.Clients) == 0 {
		return "<unavailable>"
	}
	header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil {
		return fmt.Sprintf("<unavailable:%v>", err)
	}
	if header == nil {
		return "<unavailable:nil-header>"
	}
	return header.Coinbase.Hex()
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
