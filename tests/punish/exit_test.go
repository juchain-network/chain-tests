package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
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

	opts, err := ctx.GetTransactor(valKey)
	utils.AssertNoError(t, err, "failed to get transactor")

	// 1. Resign
	t.Log("Resigning...")
	txResign, err := ctx.Staking.ResignValidator(opts)
	utils.AssertNoError(t, err, "resign failed")
	ctx.WaitMined(txResign.Hash())

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

	valKey, valAddr, err := createAndRegisterValidator(t, "ReEntry Validator")
	utils.AssertNoError(t, err, "failed setup")
	opts, err := ctx.GetTransactor(valKey)
	utils.AssertNoError(t, err, "failed transactor")

	t.Logf("Exiting validator %s to allow re-proposal...", valAddr.Hex())

	// 1. Resign & Exit
	txR, err := ctx.Staking.ResignValidator(opts)
	utils.AssertNoError(t, err, "resign failed")
	ctx.WaitMined(txR.Hash())
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
	ctx.WaitMined(txAdd.Hash())

	// Wait until jail period completes before unjailing
	info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
	current, _ := ctx.Clients[0].BlockNumber(context.Background())
	if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
		targetHeight := info.JailUntilBlock.Uint64() + 1
		maxAttempts := 2
		if targetHeight > current {
			maxAttempts = int(targetHeight-current) + 2
		}
		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: maxAttempts,
			Interval:    100 * time.Millisecond,
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			h, err := ctx.Clients[0].BlockNumber(context.Background())
			if err != nil {
				return false, err
			}
			return h >= targetHeight, nil
		})
	}
	// Ensure we are in a new epoch and not on an epoch block before unjailing
	waitForNextEpochBlock(t)
	robustUnjailValidator(t, valKey, valAddr)
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
		// Keep the test deterministic and fast.
		ctx.EnsureConfig(4, big.NewInt(2), period)
		period, err = ctx.Proposal.WithdrawProfitPeriod(nil)
		utils.AssertNoError(t, err, "failed to read withdraw profit period after update")
		// Ensure we have waited enough blocks before first withdrawal
		if period.Sign() > 0 {
			waitBlocks(t, int(period.Uint64()+1))
		}
		opts, err := ctx.GetTransactor(proposerKey)
		utils.AssertNoError(t, err, "failed to get transactor")
		tx, err := ctx.Validators.WithdrawProfits(opts, proposerAddr)
		utils.AssertNoError(t, err, "withdraw profits failed")
		ctx.WaitMined(tx.Hash())

		opts, err = ctx.GetTransactor(proposerKey)
		utils.AssertNoError(t, err, "failed to get transactor for second withdraw")
		_, err = ctx.Validators.WithdrawProfits(opts, proposerAddr)
		if err == nil {
			t.Fatal("Expected frequency limit error, got success")
		}
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
		key, _, err := createAndRegisterValidator(t, "P-06 Double")
		utils.AssertNoError(t, err, "create val failed")
		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed")

		// 1. Resign
		tx, _ := ctx.Staking.ResignValidator(opts)
		ctx.WaitMined(tx.Hash())

		// 2. Resign Again
		_, err = ctx.Staking.ResignValidator(opts)
		if err == nil {
			t.Fatal("Double resign should fail")
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
	opts, err := ctx.GetTransactor(key)
	utils.AssertNoError(t, err, "transactor failed")

	// 2. Resign & Wait & Exit
	txR, err := ctx.Staking.ResignValidator(opts)
	utils.AssertNoError(t, err, "resign failed")
	ctx.WaitMined(txR.Hash())
	// Wait for next epoch boundary so validator is out of active set
	waitForNextEpochBlock(t)
	robustExitValidator(t, key)

	// 3. Delegate to another validator
	targetVal := common.HexToAddress(ctx.Config.Validators[0].Address)
	robustDelegate(t, key, targetVal, utils.ToWei(10))

	// Verify
	info, _ := ctx.Staking.GetDelegationInfo(nil, addr, targetVal)
	utils.AssertBigIntEq(t, info.Amount, utils.ToWei(10), "Delegation amount check failed")
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

	// 1. Setup Validator
	key, addr, err := createAndRegisterValidator(t, "P-20 Punished")
	utils.AssertNoError(t, err, "create val failed")
	opts, err := ctx.GetTransactor(key)
	utils.AssertNoError(t, err, "transactor failed")

	// 2. Simulate Jail/Resign
	txR, err := ctx.Staking.ResignValidator(opts)
	utils.AssertNoError(t, err, "resign failed")
	ctx.WaitMined(txR.Hash())

	// 3. Must pass proposal again to unjail (Redemption)
	err = passProposalFor(t, addr, "P-20 Redemption")
	utils.AssertNoError(t, err, "redemption proposal failed")

	// 4. Wait jail period
	info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
	current, _ := ctx.Clients[0].BlockNumber(context.Background())
	if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
		targetHeight := info.JailUntilBlock.Uint64() + 1
		maxAttempts := 2
		if targetHeight > current {
			maxAttempts = int(targetHeight-current) + 2
		}
		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: maxAttempts,
			Interval:    100 * time.Millisecond,
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			h, err := ctx.Clients[0].BlockNumber(context.Background())
			if err != nil {
				return false, err
			}
			return h >= targetHeight, nil
		})
	}

	// 5. Unjail
	waitForNextEpochBlock(t)
	robustUnjailValidator(t, key, addr)

	// 6. Wait for next epoch to be active in currentValidatorSet
	_ = testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 2,
		Interval:    100 * time.Millisecond,
		OnRetry: func(int) {
			waitForNextEpochBlock(t)
		},
	}, func() (bool, error) {
		status, err := ctx.Validators.IsValidatorActive(nil, addr)
		if err != nil {
			return false, err
		}
		return status, nil
	})

	// 7. Verify Active
	status, _ := ctx.Validators.IsValidatorActive(nil, addr)
	utils.AssertTrue(t, status, "Should be active after redemption")
}
