package tests

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

var (
	cachedExitValidatorKey     *ecdsa.PrivateKey
	cachedExitValidatorAddr    common.Address
	cachedReentryValidatorKey  *ecdsa.PrivateKey
	cachedReentryValidatorAddr common.Address
)

// TestF1_ExitFlow handles P-01 and P-02
func TestF1_ExitFlow(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	valKey, valAddr, err := createAndRegisterValidator(t, "Exit Validator")
	utils.AssertNoError(t, err, "failed to setup validator")

	// Ensure the validator is in the active set before testing exit restrictions.
	if !waitForValidatorActive(t, valAddr, 5) {
		t.Fatalf("validator not active after epoch transition; cannot validate exit restriction deterministically")
	}

	cachedExitValidatorKey = valKey
	cachedExitValidatorAddr = valAddr

	opts, err := ctx.GetTransactor(valKey)
	utils.AssertNoError(t, err, "failed to get transactor")

	// 1. Resign
	t.Log("Resigning...")
	robustResignValidator(t, valKey)

	// Verify jailed status (state update may lag by a few blocks).
	if !waitForValidatorJailed(t, valAddr, 5) {
		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		t.Fatalf("should be jailed after resign (isJailed=%v)", info.IsJailed)
	}

	// 2. Try immediate exit (should fail if in active set)
	t.Log("Attempting immediate exit (expecting failure if in active set)...")
	opts, err = ctx.GetTransactor(valKey)
	utils.AssertNoError(t, err, "failed to get transactor for exit")
	txExit, err := ctx.Staking.ExitValidator(opts)
	if err == nil {
		errW := ctx.WaitMined(txExit.Hash())
		if errW == nil {
			t.Fatal("Exit should have failed in active set")
		}
		t.Log("Exit failed at receipt level as expected:", errW)
	} else {
		t.Log("Exit failed at simulation level as expected:", err)
	}
}

// TestF2_QuickReEntry handles P-18
func TestF2_QuickReEntry(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	var (
		valKey  *ecdsa.PrivateKey
		valAddr common.Address
		err     error
	)
	if cachedExitValidatorKey != nil && cachedExitValidatorAddr != (common.Address{}) {
		if info, errInfo := ctx.Staking.GetValidatorInfo(nil, cachedExitValidatorAddr); errInfo == nil && info.IsRegistered {
			valKey = cachedExitValidatorKey
			valAddr = cachedExitValidatorAddr
			t.Logf("Reusing validator from F1: %s", valAddr.Hex())
		}
	}
	if valKey == nil {
		valKey, valAddr, err = createAndRegisterValidator(t, "ReEntry Validator")
		utils.AssertNoError(t, err, "failed setup")
	}
	opts, err := ctx.GetTransactor(valKey)
	utils.AssertNoError(t, err, "failed transactor")

	t.Logf("Exiting validator %s to allow re-proposal...", valAddr.Hex())

	// 1. Resign & Exit
	infoBeforeExit, errInfo := ctx.Staking.GetValidatorInfo(nil, valAddr)
	utils.AssertNoError(t, errInfo, "failed to read validator info before exit")
	if !infoBeforeExit.IsJailed {
		robustResignValidator(t, valKey)
	}
	robustExitValidator(t, valKey)

	// Verify pass is now false
	p, _ := ctx.Proposal.Pass(nil, valAddr)
	utils.AssertTrue(t, !p, "pass should be false after exit")

	// 2. Re-propose
	err = passProposalFor(t, valAddr, "ReEntry Proposal")
	utils.AssertNoError(t, err, "re-proposal failed")

	pass, _ := ctx.Proposal.Pass(nil, valAddr)
	utils.AssertTrue(t, pass, "should be passed again")

	// 3. Add stake and unjail to re-enter (re-register is not allowed once registered)
	minStake, err := ctx.Proposal.MinValidatorStake(nil)
	utils.AssertNoError(t, err, "failed to read min validator stake")
	opts, err = ctx.GetTransactor(valKey)
	utils.AssertNoError(t, err, "failed transactor for add stake")
	opts.Value = minStake
	txAdd, err := ctx.Staking.AddValidatorStake(opts)
	utils.AssertNoError(t, err, "add stake failed")
	utils.AssertNoError(t, ctx.WaitMined(txAdd.Hash()), "add stake tx failed")
	err = testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 4,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitNextBlock()
		},
	}, func() (bool, error) {
		info, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
		if err != nil {
			return false, err
		}
		return info.SelfStake != nil && info.SelfStake.Cmp(minStake) >= 0, nil
	})
	utils.AssertNoError(t, err, "validator stake did not reach minimum after add stake")

	// Wait until jail period completes before unjailing
	info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
	current, _ := ctx.Clients[0].BlockNumber(context.Background())
	if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
		targetHeight := info.JailUntilBlock.Uint64() + 1
		if targetHeight > current {
			waitBlocks(t, int(targetHeight-current))
		}
		h, err := ctx.Clients[0].BlockNumber(context.Background())
		utils.AssertNoError(t, err, "read current block failed after jail wait")
		utils.AssertTrue(t, h >= targetHeight, "jail period wait failed")
	}
	// Avoid epoch-block-only restrictions without forcing a full epoch transition.
	ctx.WaitIfEpochBlock()
	robustUnjailValidator(t, valKey, valAddr)
	cachedReentryValidatorKey = valKey
	cachedReentryValidatorAddr = valAddr
}

// TestF3_WithdrawProfits handles P-08 and P-15
func TestF3_WithdrawProfits(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	proposerKey := ctx.GenesisValidators[0]
	proposerAddr := crypto.PubkeyToAddress(proposerKey.PublicKey)

	_, _, incoming, _, _, _ := ctx.Validators.GetValidatorInfo(nil, proposerAddr)
	t.Logf("Validator %s has %s fees", proposerAddr.Hex(), utils.WeiToEther(incoming))

	if incoming.Cmp(big.NewInt(0)) > 0 {
		period, err := ctx.Proposal.WithdrawProfitPeriod(nil)
		utils.AssertNoError(t, err, "failed to read withdraw profit period")
		// Keep this boundary check deterministic: second immediate withdraw should be rate-limited.
		if err := ctx.EnsureConfig(4, big.NewInt(5), period); err != nil {
			t.Logf("cannot set WithdrawProfitPeriod to short window, continue with current period: %v", err)
		}
		period, err = ctx.Proposal.WithdrawProfitPeriod(nil)
		utils.AssertNoError(t, err, "failed to read withdraw profit period after update")
		if period == nil || period.Sign() <= 0 {
			t.Skip("skip withdraw cooldown boundary check: invalid withdraw period")
		}
		shortWithdrawPeriod := period.Uint64() <= 30
		if !shortWithdrawPeriod {
			t.Logf("withdraw period remains large (%s), using best-effort first-withdraw path", period.String())
		}
		_, _, incomingNow, _, lastWithdrawBlock, err := ctx.Validators.GetValidatorInfo(nil, proposerAddr)
		utils.AssertNoError(t, err, "failed to read validator info before withdraw")
		if incomingNow.Cmp(big.NewInt(0)) <= 0 {
			t.Skip("no incoming fees available for withdraw")
		}
		if shortWithdrawPeriod && period.Sign() > 0 && lastWithdrawBlock != nil && lastWithdrawBlock.Sign() > 0 {
			curHeight, err := ctx.Clients[0].BlockNumber(context.Background())
			utils.AssertNoError(t, err, "failed to read current block before withdraw")
			targetHeight := new(big.Int).Add(lastWithdrawBlock, period).Uint64()
			if curHeight < targetHeight {
				waitBlocks(t, int(targetHeight-curHeight))
			}
		}

		withdrawn := false
		maxAttempts := 4
		if period.Sign() > 0 {
			if p := int(period.Int64()) + 2; p > maxAttempts {
				maxAttempts = p
			}
			if maxAttempts > 12 {
				maxAttempts = 12
			}
		}
		for attempt := 0; attempt < maxAttempts; attempt++ {
			opts, err := ctx.GetTransactor(proposerKey)
			if err != nil {
				waitNextBlock()
				continue
			}
			tx, err := ctx.Validators.WithdrawProfits(opts, proposerAddr)
			if err == nil {
				if ctx.WaitMined(tx.Hash()) == nil {
					withdrawn = true
					break
				}
				if attempt < maxAttempts-1 {
					waitWithdrawProfitCooldownFor(t, proposerAddr)
				}
				continue
			}
			if strings.Contains(err.Error(), "wait enough blocks") {
				if !shortWithdrawPeriod {
					t.Skipf("skip withdraw cooldown boundary check: first withdraw still locked under large period (%s)", period.String())
				}
				waitWithdrawProfitCooldownFor(t, proposerAddr)
				continue
			}
			if strings.Contains(err.Error(), "Epoch block forbidden") {
				waitNextBlock()
				continue
			}
			if attempt < maxAttempts-1 {
				waitNextBlock()
			} else {
				t.Logf("withdraw attempt failed with non-retryable error: %v", err)
			}
		}
		utils.AssertTrue(t, withdrawn, "withdraw profits did not become available in time")

		opts, err := ctx.GetTransactorNoEpochWait(proposerKey, true)
		utils.AssertNoError(t, err, "failed to get transactor for second withdraw")
		tx2, err := ctx.Validators.WithdrawProfits(opts, proposerAddr)
		if err == nil {
			if errW := ctx.WaitMined(tx2.Hash()); errW == nil {
				_, _, _, _, lastWithdrawBlockAfter, infoErr := ctx.Validators.GetValidatorInfo(nil, proposerAddr)
				curHeight, curErr := ctx.Clients[0].BlockNumber(context.Background())
				if infoErr == nil && curErr == nil && period != nil && lastWithdrawBlockAfter != nil {
					t.Fatalf("expected second withdraw to be rate-limited, got success at height=%d lastWithdrawBlock=%s period=%s",
						curHeight, lastWithdrawBlockAfter.String(), period.String())
				}
				t.Fatal("Expected frequency limit error, got success")
			}
			return
		}
		if strings.Contains(err.Error(), "wait enough blocks") || strings.Contains(err.Error(), "You don't have any profits") {
			return
		}
		t.Fatalf("unexpected second withdraw error: %v", err)
	}
}

// TestF4_MiscExit handles P-09, P-05, P-06
func TestF4_MiscExit(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	t.Run("P-09_MinerOnlyPunish", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "failed user setup")
		opts, err := ctx.GetTransactor(userKey)
		utils.AssertNoError(t, err, "failed transactor")

		target := common.HexToAddress(ctx.Config.Validators[0].Address)
		_, err = ctx.Punish.Punish(opts, target)
		utils.AssertTrue(t, err != nil, "Expected error 'Miner only' for Punish call from user")
	})

	t.Run("P-05_NonValidatorExit", func(t *testing.T) {
		key, _, err := ctx.CreateAndFundAccount(utils.ToWei(10))
		utils.AssertNoError(t, err, "create account failed")
		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed")

		txExit, err := ctx.Staking.ExitValidator(opts)
		if err == nil {
			errW := ctx.WaitMined(txExit.Hash())
			if errW == nil {
				t.Fatal("Non-validator should not be able to exit")
			}
			t.Log("Exit failed at receipt level as expected:", errW)
		} else {
			t.Log("Exit failed at simulation level as expected:", err)
		}
	})

	t.Run("P-06_DoubleResign", func(t *testing.T) {
		var (
			key     *ecdsa.PrivateKey
			valAddr common.Address
			err     error
		)
		if cachedReentryValidatorKey != nil && cachedReentryValidatorAddr != (common.Address{}) {
			if info, errInfo := ctx.Staking.GetValidatorInfo(nil, cachedReentryValidatorAddr); errInfo == nil && info.IsRegistered && !info.IsJailed {
				key = cachedReentryValidatorKey
				valAddr = cachedReentryValidatorAddr
				t.Logf("Reusing validator from F2 for P-06: %s", cachedReentryValidatorAddr.Hex())
			}
		}
		if key == nil {
			key, valAddr, err = createAndRegisterValidator(t, "P-06 Double")
			utils.AssertNoError(t, err, "create val failed")
			if !waitForValidatorActive(t, valAddr, 5) {
				t.Fatalf("validator not active after registration; cannot validate double resign deterministically: %s", valAddr.Hex())
			}
		}

		// 1. Resign
		robustResignValidator(t, key)
		if !waitForValidatorJailed(t, valAddr, 5) {
			info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
			t.Fatalf("validator not jailed after first resign (validator=%s isJailed=%v)", valAddr.Hex(), info.IsJailed)
		}

		// 2. Resign Again
		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed")
		tx, err := ctx.Staking.ResignValidator(opts)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				t.Fatal("Double resign should fail")
			}
		}
	})
}

// TestF5_RoleChange handles P-19
func TestF5_RoleChange(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	// 1. Setup Validator
	key, addr, err := createAndRegisterValidator(t, "P-19 RoleChange")
	utils.AssertNoError(t, err, "create val failed")

	// 2. Resign & Exit
	robustResignValidator(t, key)
	// If still in active set, cross one epoch and re-check.
	active, err := ctx.Validators.IsValidatorActive(nil, addr)
	utils.AssertNoError(t, err, "failed to query active status after resign")
	if active {
		waitForNextEpochBlock(t)
		active, err = ctx.Validators.IsValidatorActive(nil, addr)
		utils.AssertNoError(t, err, "failed to query active status after epoch wait")
	}
	utils.AssertTrue(t, !active, "validator should be inactive before exit")
	robustExitValidator(t, key)

	// 3. Delegate to another validator
	// Pick a currently valid target to avoid transient config/index assumptions.
	targetVal := common.Address{}
	minStake, _ := ctx.Proposal.MinValidatorStake(nil)
	if highest, errHighest := ctx.Validators.GetHighestValidators(nil); errHighest == nil {
		if top, errTop := ctx.Staking.GetTopValidators(nil, highest); errTop == nil {
			for _, candidate := range top {
				if candidate == addr {
					continue
				}
				info, errInfo := ctx.Staking.GetValidatorInfo(nil, candidate)
				if errInfo != nil || info.SelfStake == nil {
					continue
				}
				if minStake != nil && minStake.Sign() > 0 && info.SelfStake.Cmp(minStake) < 0 {
					continue
				}
				targetVal = candidate
				break
			}
		}
	}
	if targetVal == (common.Address{}) {
		// Fallback to genesis validators when top list is temporarily unavailable.
		for _, vk := range ctx.GenesisValidators {
			candidate := crypto.PubkeyToAddress(vk.PublicKey)
			if candidate == addr {
				continue
			}
			info, errInfo := ctx.Staking.GetValidatorInfo(nil, candidate)
			if errInfo != nil || info.SelfStake == nil {
				continue
			}
			if minStake != nil && minStake.Sign() > 0 && info.SelfStake.Cmp(minStake) < 0 {
				continue
			}
			targetVal = candidate
			break
		}
	}
	if targetVal == (common.Address{}) {
		targetVal = common.HexToAddress(ctx.Config.Validators[0].Address)
	}
	infoTarget, errTarget := ctx.Staking.GetValidatorInfo(nil, targetVal)
	if errTarget != nil || infoTarget.SelfStake == nil ||
		(minStake != nil && minStake.Sign() > 0 && infoTarget.SelfStake.Cmp(minStake) < 0) {
		t.Fatalf("selected target validator is not valid: target=%s selfStake=%v minStake=%v err=%v", targetVal.Hex(), infoTarget.SelfStake, minStake, errTarget)
	}
	t.Logf("Delegating to target validator %s", targetVal.Hex())
	robustDelegate(t, key, targetVal, utils.ToWei(10))

	// Verify
	expected := utils.ToWei(10)
	err = testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 4,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitNextBlock()
		},
	}, func() (bool, error) {
		info, err := ctx.Staking.GetDelegationInfo(nil, addr, targetVal)
		if err != nil {
			return false, err
		}
		return info.Amount != nil && info.Amount.Cmp(expected) == 0, nil
	})
	if err != nil {
		info, _ := ctx.Staking.GetDelegationInfo(nil, addr, targetVal)
		utils.AssertBigIntEq(t, info.Amount, expected, "Delegation amount check failed")
	}
}

// TestF6_DoubleSignWindow handles S-20
func TestF6_DoubleSignWindow(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	// Validator who just mined a block cannot resign immediately
	valKey := ctx.GenesisValidators[0]
	opts, err := ctx.GetTransactor(valKey)
	utils.AssertNoError(t, err, "transactor failed")

	_, err = ctx.Staking.ResignValidator(opts)
	if err != nil {
		t.Logf("Correctly rejected (if recently active): %v", err)
	} else {
		t.Log("Resign succeeded (not active in current window)")
	}
}

// TestF7_PunishedRedemption handles P-20
func TestF7_PunishedRedemption(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}
	var err error

	// 1. Setup Validator
	key := cachedReentryValidatorKey
	addr := cachedReentryValidatorAddr
	if key == nil || addr == (common.Address{}) {
		key, addr, err = createAndRegisterValidator(t, "P-20 Punished")
		utils.AssertNoError(t, err, "create val failed")
	} else {
		info, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		if errInfo != nil || !info.IsRegistered {
			key, addr, err = createAndRegisterValidator(t, "P-20 Punished")
			utils.AssertNoError(t, err, "create val failed")
		} else {
			t.Logf("Reusing validator from F2: %s", addr.Hex())
		}
	}

	// 2. Simulate Jail/Resign
	robustResignValidator(t, key)

	// 3. Must pass proposal again to unjail (Redemption)
	passNow, errPass := ctx.Proposal.Pass(nil, addr)
	utils.AssertNoError(t, errPass, "failed to read proposal pass status before redemption")
	if !passNow {
		err = passProposalFor(t, addr, "P-20 Redemption")
		utils.AssertNoError(t, err, "redemption proposal failed")
	} else {
		t.Logf("Validator %s already has pass=true after resign, skip redemption proposal", addr.Hex())
	}

	// 4. Wait jail period
	info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
	current, _ := ctx.Clients[0].BlockNumber(context.Background())
	if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
		targetHeight := info.JailUntilBlock.Uint64() + 1
		if targetHeight > current {
			waitBlocks(t, int(targetHeight-current))
		}
		h, err := ctx.Clients[0].BlockNumber(context.Background())
		utils.AssertNoError(t, err, "read current block failed after jail wait")
		utils.AssertTrue(t, h >= targetHeight, "jail period wait failed")
	}

	// 5. Unjail
	ctx.WaitIfEpochBlock()
	robustUnjailValidator(t, key, addr)

	// 6. Become active in currentValidatorSet (allow one epoch transition).
	status, err := ctx.Validators.IsValidatorActive(nil, addr)
	utils.AssertNoError(t, err, "failed to query active status after unjail")
	if !status {
		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 6,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitNextBlock()
			},
		}, func() (bool, error) {
			ok, err := ctx.Validators.IsValidatorActive(nil, addr)
			if err != nil {
				return false, err
			}
			status = ok
			return ok, nil
		})
	}
	if !status {
		waitForNextEpochBlock(t)
		status, err = ctx.Validators.IsValidatorActive(nil, addr)
		utils.AssertNoError(t, err, "failed to query active status after epoch wait")
	}

	// 7. Verify Active
	utils.AssertTrue(t, status, "Should be active after redemption")
}
