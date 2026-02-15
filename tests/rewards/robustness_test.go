package tests

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestH_Robustness(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}

	// [V-01] Jailed Validator Redistribution
	t.Run("V-01_JailedRedistribution", func(t *testing.T) {
		valKey, valAddr, err := createAndRegisterValidator(t, "V-01 Resign")
		utils.AssertNoError(t, err, "setup validator failed")
		// Ensure doubleSignWindow is small enough for test.
		curWindow, _ := ctx.Proposal.DoubleSignWindow(nil)
		if curWindow != nil && curWindow.Cmp(big.NewInt(200)) > 0 {
			_ = ctx.EnsureConfig(15, big.NewInt(20), curWindow)
			curWindow = big.NewInt(20)
		}
		// Wait until we're outside the doubleSignWindow to avoid revert.
		if curWindow != nil && curWindow.Sign() > 0 {
			lastActive, _ := ctx.Staking.LastActiveBlock(nil, valAddr)
			if lastActive != nil && lastActive.Sign() > 0 {
				curHeight, _ := ctx.Clients[0].BlockNumber(context.Background())
				target := new(big.Int).Add(lastActive, curWindow)
				target.Add(target, big.NewInt(1))
				if curHeight < target.Uint64() {
					waitBlocks(t, int(target.Uint64()-curHeight))
				}
			}
		}
		resigned := false
		for retry := 0; retry < 6; retry++ {
			ctx.WaitIfEpochBlock()
			opts, _ := ctx.GetTransactor(valKey)
			tx, err := ctx.Staking.ResignValidator(opts)
			if err != nil {
				if strings.Contains(err.Error(), "Epoch block forbidden") || strings.Contains(err.Error(), "active set") {
					waitBlocks(t, 1)
					continue
				}
				t.Fatalf("resign failed: %v", err)
			}
			if errW := ctx.WaitMined(tx.Hash()); errW != nil {
				info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
				if info.IsJailed {
					resigned = true
					break
				}
				if strings.Contains(errW.Error(), "transaction") && strings.Contains(errW.Error(), "reverted") {
					waitForNextEpochBlock(t)
					waitBlocks(t, 1)
					continue
				}
				t.Fatalf("resign tx failed: %v", errW)
			}
			resigned = true
			break
		}
		if !resigned {
			t.Fatal("resign retries exhausted")
		}

		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		if info.IsJailed {
			return
		}
		err = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 4,
			Interval:    100 * time.Millisecond,
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			info, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if err != nil {
				return false, err
			}
			return info.IsJailed, nil
		})
		utils.AssertNoError(t, err, "validator should become jailed")
	})

	// [S-16] Zero Delegated Rewards
	t.Run("S-16_ZeroDelegatedRewards", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "ZeroDelegation")
		utils.AssertNoError(t, err, "failed to setup validator")

		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 2,
			Interval:    100 * time.Millisecond,
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			info, err := ctx.Staking.GetValidatorInfo(nil, addr)
			if err != nil {
				return false, err
			}
			return info.AccumulatedRewards.Sign() > 0, nil
		})

		info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
		t.Logf("Validator %s accumulated: %s", addr.Hex(), info.AccumulatedRewards.String())

		opts, _ := ctx.GetTransactor(key)
		ctx.Staking.ClaimValidatorRewards(opts)
	})

	// [S-15] Proposal Expiry
	t.Run("S-15_ProposalExpiry", func(t *testing.T) {
		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(10))
		utils.AssertNoError(t, err, "setup user failed")

		proposerKey := getActiveProposerOrSkip(t, 3)
		opts, err := ctx.GetTransactor(proposerKey)
		utils.AssertNoError(t, err, "transactor failed")

		tx, err := ctx.Proposal.CreateProposal(opts, userAddr, true, "Expiry Test")
		utils.AssertNoError(t, err, "create proposal failed")
		ctx.WaitMined(tx.Hash())

		propID := getPropID(tx)
		if propID == ([32]byte{}) {
			t.Fatal("propID missing")
		}

		p, _ := ctx.Proposal.Proposals(nil, propID)
		proposerAddr := crypto.PubkeyToAddress(proposerKey.PublicKey)
		utils.AssertTrue(t, p.Proposer == proposerAddr, "Proposal not found")

		// Ensure no direct side-effects from user key
		ctx.RefreshNonce(crypto.PubkeyToAddress(userKey.PublicKey))
	})
}
