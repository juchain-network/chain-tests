package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

func rewardAccrualAttemptBudget() int {
	attempts := 8
	if ctx != nil {
		if period, err := ctx.Proposal.WithdrawProfitPeriod(nil); err == nil && period != nil && period.Sign() > 0 {
			attempts = int(period.Int64()) + 3
		} else if ctx.Config.Test.Params.WithdrawProfit > 0 {
			attempts = int(ctx.Config.Test.Params.WithdrawProfit) + 3
		}
	}
	if attempts < 4 {
		attempts = 4
	}
	if attempts > 12 {
		attempts = 12
	}
	return attempts
}

func retryAfterBlockInterval() time.Duration {
	d := retrySleep() / 4
	if d < 10*time.Millisecond {
		return 10 * time.Millisecond
	}
	return d
}

func resignValidatorWithRetry(t *testing.T, key *ecdsa.PrivateKey, addr common.Address, msg string) {
	t.Helper()

	isTransientResignErr := func(err error) bool {
		if err == nil {
			return false
		}
		lower := strings.ToLower(err.Error())
		return strings.Contains(lower, "epoch block forbidden") ||
			strings.Contains(lower, "too many removals") ||
			strings.Contains(lower, "wait until next epoch") ||
			strings.Contains(lower, "revert") ||
			strings.Contains(lower, "reverted")
	}

	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 6,
		Interval:    retryAfterBlockInterval(),
		OnRetry: func(int) {
			waitNextBlock()
		},
	}, func() (bool, error) {
		info, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		if errInfo == nil && info.IsJailed {
			return true, nil
		}

		opts, errOpts := ctx.GetTransactor(key)
		if errOpts != nil {
			return false, nil
		}
		txR, errResign := ctx.Staking.ResignValidator(opts)
		if errResign != nil {
			if isTransientResignErr(errResign) {
				return false, nil
			}
			return false, errResign
		}
		if errMine := ctx.WaitMined(txR.Hash()); errMine != nil {
			if isTransientResignErr(errMine) {
				return false, nil
			}
			return false, errMine
		}

		infoAfter, errInfoAfter := ctx.Staking.GetValidatorInfo(nil, addr)
		if errInfoAfter != nil {
			return false, nil
		}
		return infoAfter.IsJailed, nil
	})
	utils.AssertNoError(t, err, msg)
}

func waitDelegationAmount(
	t *testing.T,
	delegator common.Address,
	validator common.Address,
	expected *big.Int,
	msg string,
) {
	if expected == nil {
		t.Fatalf("expected delegation amount is nil")
	}
	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 4,
		Interval:    retryAfterBlockInterval(),
		OnRetry: func(int) {
			waitNextBlock()
		},
	}, func() (bool, error) {
		info, err := ctx.Staking.GetDelegationInfo(nil, delegator, validator)
		if err != nil {
			return false, err
		}
		return info.Amount != nil && info.Amount.Cmp(expected) == 0, nil
	})
	if err != nil {
		info, _ := ctx.Staking.GetDelegationInfo(nil, delegator, validator)
		utils.AssertBigIntEq(t, info.Amount, expected, msg)
	}
}

func TestE_Delegation(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	valAddr := common.HexToAddress(ctx.Config.Validators[0].Address)
	valKey := ctx.GenesisValidators[0]

	// Ensure config for tests
	coo, _ := ctx.Proposal.ProposalCooldown(nil)
	ctx.EnsureConfig(19, big.NewInt(1), coo)
	mst, _ := ctx.Proposal.MinValidatorStake(nil)
	ctx.EnsureConfig(8, big.NewInt(1000000000000000000), mst)
	mde, _ := ctx.Proposal.MinDelegation(nil)
	ctx.EnsureConfig(10, big.NewInt(1000000000000000000), mde)
	mud, _ := ctx.Proposal.MinUndelegation(nil)
	ctx.EnsureConfig(11, utils.ToWei(1), mud)
	unb, _ := ctx.Proposal.UnbondingPeriod(nil)
	ctx.EnsureConfig(6, big.NewInt(3), unb)
	unj, _ := ctx.Proposal.ValidatorUnjailPeriod(nil)
	ctx.EnsureConfig(7, big.NewInt(3), unj)
	wp, _ := ctx.Proposal.WithdrawProfitPeriod(nil)
	ctx.EnsureConfig(4, big.NewInt(2), wp)

	var (
		reusedJailedValidatorKey  *ecdsa.PrivateKey
		reusedJailedValidatorAddr common.Address
		reusedJailedDelegatorKey  *ecdsa.PrivateKey
		reusedJailedFlowReady     bool
	)
	if key, addr, err := createAndRegisterValidator(t, "D-04a Shared"); err == nil {
		reusedJailedValidatorKey = key
		reusedJailedValidatorAddr = addr
	} else {
		t.Logf("shared validator pre-create failed, fallback to on-demand create: %v", err)
	}

	t.Run("D-01_FullFlow", func(t *testing.T) {
		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(200))
		utils.AssertNoError(t, err, "failed to setup delegator")

		t.Logf("User %s delegating 100 ETH to %s...", userAddr.Hex(), valAddr.Hex())
		delegateAmount := utils.ToWei(100)
		robustDelegate(t, userKey, valAddr, delegateAmount)

		waitDelegationAmount(t, userAddr, valAddr, delegateAmount, "delegation amount mismatch")
		info, _ := ctx.Staking.GetDelegationInfo(nil, userAddr, valAddr)

		t.Log("Waiting briefly for rewards to accrue...")
		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 2,
			Interval:    retryAfterBlockInterval(),
		}, func() (bool, error) {
			info, err = ctx.Staking.GetDelegationInfo(nil, userAddr, valAddr)
			if err != nil {
				return false, err
			}
			return info.PendingRewards.Cmp(big.NewInt(0)) > 0, nil
		})
		info, _ = ctx.Staking.GetDelegationInfo(nil, userAddr, valAddr)
		t.Logf("Accumulated rewards: %s", info.PendingRewards.String())

		if info.PendingRewards.Cmp(big.NewInt(0)) > 0 {
			t.Log("Claiming rewards...")
			robustClaimRewards(t, userKey, valAddr)
		}

		t.Log("Undelegating 50 ETH...")
		undelAmount := utils.ToWei(50)
		robustUndelegate(t, userKey, valAddr, undelAmount)

		entries, _ := ctx.Staking.GetUnbondingEntries(nil, userAddr, valAddr)
		utils.AssertTrue(t, len(entries) > 0, "unbonding entry missing")
	})

	t.Run("D-01b_WithdrawUnbonded", func(t *testing.T) {
		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(200))
		utils.AssertNoError(t, err, "failed to setup delegator")

		robustDelegate(t, userKey, valAddr, utils.ToWei(20))
		robustUndelegate(t, userKey, valAddr, utils.ToWei(10))

		entries, _ := ctx.Staking.GetUnbondingEntries(nil, userAddr, valAddr)
		var completion uint64
		for _, e := range entries {
			if e.CompletionBlock != nil && e.CompletionBlock.Sign() > 0 {
				cb := e.CompletionBlock.Uint64()
				if cb > completion {
					completion = cb
				}
			}
		}
		if completion > 0 {
			curHeight, _ := ctx.Clients[0].BlockNumber(context.Background())
			if curHeight < completion {
				waitBlocks(t, int(completion-curHeight))
			}
		}

		robustWithdrawUnbonded(t, userKey, valAddr, 20)

		cnt, _ := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
		if cnt == nil {
			cnt = big.NewInt(0)
		}
		if cnt.Sign() != 0 {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 4,
				Interval:    retryAfterBlockInterval(),
				OnRetry: func(int) {
					waitNextBlock()
					robustWithdrawUnbonded(t, userKey, valAddr, 20)
				},
			}, func() (bool, error) {
				latest, err := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
				if err != nil {
					return false, err
				}
				if latest == nil {
					latest = big.NewInt(0)
				}
				cnt = latest
				return latest.Sign() == 0, nil
			})
		}
		utils.AssertTrue(t, cnt.Sign() == 0, "unbonding entries should be cleared")
	})

	t.Run("D-02_ClaimCommission", func(t *testing.T) {
		infoBefore, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		t.Logf("Initial accumulated rewards: %s", infoBefore.AccumulatedRewards.String())
		claimedBefore := new(big.Int).Set(infoBefore.TotalClaimedRewards)

		maxAccrualAttempts := rewardAccrualAttemptBudget()
		accrued := false
		var lastAccrualErr error
		for attempt := 0; attempt < maxAccrualAttempts; attempt++ {
			infoNow, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if err != nil {
				lastAccrualErr = err
			} else if infoNow.AccumulatedRewards.Sign() > 0 {
				accrued = true
				break
			}
			if attempt < maxAccrualAttempts-1 {
				waitBlocks(t, 1)
			}
		}
		if !accrued {
			if lastAccrualErr != nil {
				t.Fatalf("validator rewards should accrue before claim: %v", lastAccrualErr)
			}
			t.Fatal("validator rewards should accrue before claim")
		}

		robustClaimValidatorRewards(t, valKey)

		err := testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 3,
			Interval:    retryAfterBlockInterval(),
		}, func() (bool, error) {
			infoAfter, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if err != nil {
				return false, err
			}
			return infoAfter.TotalClaimedRewards.Cmp(claimedBefore) > 0, nil
		})
		utils.AssertNoError(t, err, "total claimed rewards should increase")
	})

	t.Run("D-02b_ClaimNoDelegation", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(10))
		utils.AssertNoError(t, err, "setup user failed")
		opts, _ := ctx.GetTransactor(userKey)
		_, err = ctx.Staking.ClaimRewards(opts, valAddr)
		if err == nil {
			t.Fatal("Should fail claim rewards with no delegation")
		}
	})

	t.Run("D-04a_ValidatorResignImpact", func(t *testing.T) {
		var (
			key  *ecdsa.PrivateKey
			addr common.Address
		)
		if reusedJailedValidatorKey != nil && reusedJailedValidatorAddr != (common.Address{}) {
			if info, errInfo := ctx.Staking.GetValidatorInfo(nil, reusedJailedValidatorAddr); errInfo == nil && info.IsRegistered {
				key = reusedJailedValidatorKey
				addr = reusedJailedValidatorAddr
				t.Logf("Reusing shared validator for D-04a: %s", addr.Hex())
			}
		}
		if key == nil {
			err := testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 3,
				Interval:    retryAfterBlockInterval(),
				OnRetry: func(int) {
					waitBlocks(t, 1)
				},
			}, func() (bool, error) {
				k, a, errCreate := createAndRegisterValidator(t, "D-04a Val")
				if errCreate == nil {
					key = k
					addr = a
					return true, nil
				}
				msg := errCreate.Error()
				if strings.Contains(msg, "Proposal expired") ||
					strings.Contains(msg, "failed to pass proposal") ||
					strings.Contains(msg, "condition not met") {
					return false, nil
				}
				return false, errCreate
			})
			utils.AssertNoError(t, err, "setup validator failed")
		}

		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(500))
		utils.AssertNoError(t, err, "setup user failed")
		robustDelegate(t, userKey, addr, utils.ToWei(10))

		resignValidatorWithRetry(t, key, addr, "resign validator failed")

		opts, _ := ctx.GetTransactor(userKey)
		opts.Value = utils.ToWei(5)
		txDel, err := ctx.Staking.Delegate(opts, addr)
		if err == nil {
			if errW := ctx.WaitMined(txDel.Hash()); errW == nil {
				t.Fatal("Should not be able to delegate to resigned validator")
			}
		}

		reusedJailedValidatorKey = key
		reusedJailedValidatorAddr = addr
		reusedJailedDelegatorKey = userKey
		reusedJailedFlowReady = true
	})

	t.Run("D-04b_MultiDelegatorIsolation", func(t *testing.T) {
		keyA, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user A failed")
		keyB, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user B failed")

		robustDelegate(t, keyA, valAddr, utils.ToWei(10))
		robustDelegate(t, keyB, valAddr, utils.ToWei(20))

		waitDelegationAmount(t, crypto.PubkeyToAddress(keyA.PublicKey), valAddr, utils.ToWei(10), "User A amount mismatch")
		waitDelegationAmount(t, crypto.PubkeyToAddress(keyB.PublicKey), valAddr, utils.ToWei(20), "User B amount mismatch")
	})

	t.Run("D-08_DelegationBelowMin", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(5))
		utils.AssertNoError(t, err, "setup user failed")
		opts, _ := ctx.GetTransactor(userKey)
		opts.Value = utils.ToWei(0.1)
		_, err = ctx.Staking.Delegate(opts, valAddr)
		if err == nil {
			t.Fatal("Should fail delegation below minimum (0.1 ETH)")
		}
	})

	t.Run("D-03_CompoundDelegation", func(t *testing.T) {
		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "failed delegator setup")
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		waitDelegationAmount(t, userAddr, valAddr, utils.ToWei(20), "total amount mismatch")
	})

	t.Run("D-07_UndelegateOverflow", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(50))
		utils.AssertNoError(t, err, "failed setup user")
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		opts, _ := ctx.GetTransactor(userKey)
		_, err = ctx.Staking.Undelegate(opts, valAddr, utils.ToWei(11))
		utils.AssertTrue(t, err != nil, "should fail undelegating more than staked")
	})

	t.Run("D-15_DelegatorToValidator", func(t *testing.T) {
		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(250005))
		utils.AssertNoError(t, err, "failed user setup")
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		err = passProposalFor(t, userAddr, "D-15 Propose")
		utils.AssertNoError(t, err, "proposal failed")

		var lastErr error
		for retry := 0; retry < 10; retry++ {
			ctx.WaitIfEpochBlock()
			opts, errG := ctx.GetTransactor(userKey)
			if errG != nil {
				lastErr = errG
				ctx.RefreshNonce(userAddr)
				continue
			}
			opts.Value = testkit.RequireMinValidatorStake(t, func() (*big.Int, error) { return ctx.Proposal.MinValidatorStake(nil) })
			opts.GasLimit = 5000000

			txReg, err := ctx.Staking.RegisterValidator(opts, big.NewInt(500))
			if err != nil {
				lastErr = err
				if strings.Contains(err.Error(), "Epoch block forbidden") {
					ctx.WaitIfEpochBlock()
					continue
				}
				if strings.Contains(err.Error(), "Too many new validators") {
					if retry < 2 {
						waitNextBlock()
					} else {
						waitForNextEpochBlock(t)
					}
					continue
				}
				break
			}

			if errW := ctx.WaitMined(txReg.Hash()); errW != nil {
				lastErr = errW
				if strings.Contains(errW.Error(), "Epoch block forbidden") {
					ctx.WaitIfEpochBlock()
					continue
				}
				if strings.Contains(errW.Error(), "revert") || strings.Contains(errW.Error(), "reverted") {
					ctx.RefreshNonce(userAddr)
					ctx.WaitIfEpochBlock()
				}
				continue
			}
			receipt, errR := ctx.Clients[0].TransactionReceipt(context.Background(), txReg.Hash())
			if errR != nil {
				lastErr = errR
				continue
			}
			if receipt.Status == 0 {
				lastErr = fmt.Errorf("register validator tx reverted")
				ctx.RefreshNonce(userAddr)
				if retry < 2 {
					waitNextBlock()
				} else {
					waitForNextEpochBlock(t)
				}
				continue
			}

			err = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 3,
				Interval:    retryAfterBlockInterval(),
			}, func() (bool, error) {
				isVal, err := ctx.Validators.IsValidatorExist(nil, userAddr)
				if err != nil {
					return false, err
				}
				return isVal, nil
			})
			utils.AssertNoError(t, err, "should be validator")
			return
		}
		if lastErr != nil {
			t.Fatalf("register validator failed: %v", lastErr)
		}
	})

	t.Run("D-05_MultiValidatorDelegation", func(t *testing.T) {
		val1 := common.HexToAddress(ctx.Config.Validators[0].Address)
		val2 := common.HexToAddress(ctx.Config.Validators[1].Address)

		v1Info, _ := ctx.Staking.GetValidatorInfo(nil, val1)
		v2Info, _ := ctx.Staking.GetValidatorInfo(nil, val2)
		t.Logf("Val1: %s (Stake: %s), Val2: %s (Stake: %s)", val1.Hex(), v1Info.SelfStake, val2.Hex(), v2Info.SelfStake)

		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user failed")
		t.Logf("Delegating to Val1: %s", val1.Hex())
		robustDelegate(t, userKey, val1, utils.ToWei(10))
		t.Logf("Delegating to Val2: %s", val2.Hex())
		robustDelegate(t, userKey, val2, utils.ToWei(10))
		waitDelegationAmount(t, userAddr, val1, utils.ToWei(10), "V1 delegation failed")
		waitDelegationAmount(t, userAddr, val2, utils.ToWei(10), "V2 delegation failed")
	})

	t.Run("D-16_CircularDelegation", func(t *testing.T) {
		v0Key := ctx.GenesisValidators[0]
		v0Addr := common.HexToAddress(ctx.Config.Validators[0].Address)
		v1Key := ctx.GenesisValidators[1]
		v1Addr := common.HexToAddress(ctx.Config.Validators[1].Address)
		robustDelegate(t, v0Key, v1Addr, utils.ToWei(10))
		robustDelegate(t, v1Key, v0Addr, utils.ToWei(10))
		waitDelegationAmount(t, v0Addr, v1Addr, utils.ToWei(10), "V0->V1 check failed")
		waitDelegationAmount(t, v1Addr, v0Addr, utils.ToWei(10), "V1->V0 check failed")
	})

	t.Run("D-17_RoleDowngrade", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "D-17 Downgrade")
		utils.AssertNoError(t, err, "setup validator failed")

		infoBefore, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, errInfo, "failed to read validator info before resign")
		if !infoBefore.IsJailed {
			resignValidatorWithRetry(t, key, addr, "resign tx failed")
		}

		unjailPeriod, _ := ctx.Proposal.ValidatorUnjailPeriod(nil)
		// Wait until jail period is fully passed, then cross epoch for set update.
		info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
		current, _ := ctx.Clients[0].BlockNumber(context.Background())
		targetHeight := uint64(0)
		if info.IsJailed {
			if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
				targetHeight = info.JailUntilBlock.Uint64() + 1
			} else if unjailPeriod != nil && unjailPeriod.Sign() > 0 {
				targetHeight = current + unjailPeriod.Uint64() + 1
			}
		}
		if targetHeight > current {
			waitBlocks(t, int(targetHeight-current))
		}
		curInfo, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, errInfo, "read validator info failed after jail wait")
		h, errHeight := ctx.Clients[0].BlockNumber(context.Background())
		utils.AssertNoError(t, errHeight, "read current block failed after jail wait")
		if curInfo.JailUntilBlock != nil && curInfo.JailUntilBlock.Sign() > 0 {
			utils.AssertTrue(t, h >= curInfo.JailUntilBlock.Uint64(), "jail period wait failed")
		} else if targetHeight > 0 {
			utils.AssertTrue(t, h >= targetHeight, "jail period wait failed")
		}
		robustExitValidator(t, key)
		t.Logf("Ex-validator %s delegating to %s", addr.Hex(), valAddr.Hex())
		robustDelegate(t, key, valAddr, utils.ToWei(10))
		waitDelegationAmount(t, addr, valAddr, utils.ToWei(10), "Delegation check failed")
	})

	t.Run("D-06_EarlyWithdraw", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user failed")
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		robustUndelegate(t, userKey, valAddr, utils.ToWei(5))
		opts, _ := ctx.GetTransactor(userKey)
		_, err = ctx.Staking.WithdrawUnbonded(opts, valAddr, big.NewInt(10))
		if err == nil {
			t.Fatal("Early withdraw should fail")
		}
	})

	t.Run("D-07_SelfDelegation", func(t *testing.T) {
		opts, _ := ctx.GetTransactor(valKey)
		opts.Value = utils.ToWei(10)
		_, err := ctx.Staking.Delegate(opts, valAddr)
		if err == nil {
			t.Fatal("Self-delegation should fail")
		}
	})

	t.Run("D-09_DelegateToNonExistent", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user failed")
		opts, _ := ctx.GetTransactor(userKey)
		opts.Value = utils.ToWei(10)
		_, err = ctx.Staking.Delegate(opts, common.HexToAddress("0x1234"))
		if err == nil {
			t.Fatal("Should fail delegating to non-existent")
		}
	})

	t.Run("D-11_ZeroUndelegate", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user failed")
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		opts, _ := ctx.GetTransactor(userKey)
		_, err = ctx.Staking.Undelegate(opts, valAddr, big.NewInt(0))
		if err == nil {
			t.Fatal("Should fail zero undelegation")
		}
	})

	t.Run("D-18_UndelegateBelowMin", func(t *testing.T) {
		minUndel, _ := ctx.Proposal.MinUndelegation(nil)
		if minUndel == nil || minUndel.Sign() <= 0 {
			t.Fatalf("minUndelegation unavailable")
		}
		if minUndel.Cmp(big.NewInt(1)) <= 0 {
			if err := ctx.EnsureConfig(11, big.NewInt(2), minUndel); err != nil {
				t.Fatalf("failed to raise minUndelegation: %v", err)
			}
			minUndel, _ = ctx.Proposal.MinUndelegation(nil)
			if minUndel.Cmp(big.NewInt(1)) <= 0 {
				t.Fatalf("minUndelegation too small after update")
			}
		}
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(50))
		utils.AssertNoError(t, err, "setup user failed")
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		opts, _ := ctx.GetTransactor(userKey)
		_, err = ctx.Staking.Undelegate(opts, valAddr, new(big.Int).Sub(minUndel, big.NewInt(1)))
		if err == nil {
			t.Fatal("Should fail undelegate below min")
		}
	})

	var jailedValidatorAddr common.Address
	jailedValidatorReady := false
	t.Run("D-13_UndelegateFromJailed", func(t *testing.T) {
		var (
			key     *ecdsa.PrivateKey
			addr    common.Address
			userKey *ecdsa.PrivateKey
		)
		if reusedJailedFlowReady && reusedJailedValidatorKey != nil && reusedJailedDelegatorKey != nil && reusedJailedValidatorAddr != (common.Address{}) {
			key = reusedJailedValidatorKey
			addr = reusedJailedValidatorAddr
			userKey = reusedJailedDelegatorKey
			t.Logf("Reusing resigned validator from D-04a: %s", addr.Hex())
		} else {
			var err error
			key, addr, err = createAndRegisterValidator(t, "D-13 Jailed")
			utils.AssertNoError(t, err, "setup validator failed")
			userKey, _, err = ctx.CreateAndFundAccount(utils.ToWei(50))
			utils.AssertNoError(t, err, "setup user failed")
			robustDelegate(t, userKey, addr, utils.ToWei(10))
			vOpts, _ := ctx.GetTransactor(key)
			txR, err := ctx.Staking.ResignValidator(vOpts)
			utils.AssertNoError(t, err, "resign call failed")
			utils.AssertNoError(t, ctx.WaitMined(txR.Hash()), "resign tx failed")
		}

		info, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, errInfo, "read validator info failed")
		if !info.IsJailed {
			vOpts, _ := ctx.GetTransactor(key)
			txR, err := ctx.Staking.ResignValidator(vOpts)
			utils.AssertNoError(t, err, "resign call failed")
			utils.AssertNoError(t, ctx.WaitMined(txR.Hash()), "resign tx failed")
		}

		delegatorAddr := crypto.PubkeyToAddress(userKey.PublicKey)
		delInfo, errDel := ctx.Staking.GetDelegationInfo(nil, delegatorAddr, addr)
		if errDel != nil || delInfo.Amount == nil || delInfo.Amount.Cmp(utils.ToWei(10)) < 0 {
			robustDelegate(t, userKey, addr, utils.ToWei(10))
		}

		jailedValidatorAddr = addr
		jailedValidatorReady = true
		robustUndelegate(t, userKey, addr, utils.ToWei(10))
	})

	t.Run("D-12_DelegateToJailed", func(t *testing.T) {
		addr := jailedValidatorAddr
		if !jailedValidatorReady {
			key, createdAddr, err := createAndRegisterValidator(t, "D-12 Jailed")
			utils.AssertNoError(t, err, "setup validator failed")
			vOpts, _ := ctx.GetTransactor(key)
			txR, err := ctx.Staking.ResignValidator(vOpts)
			utils.AssertNoError(t, err, "resign call failed")
			utils.AssertNoError(t, ctx.WaitMined(txR.Hash()), "resign tx failed")
			addr = createdAddr
			jailedValidatorAddr = addr
			jailedValidatorReady = true
		}

		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(50))
		utils.AssertNoError(t, err, "setup user failed")
		opts, _ := ctx.GetTransactor(userKey)
		opts.Value = utils.ToWei(10)
		_, err = ctx.Staking.Delegate(opts, addr)
		if err == nil {
			t.Fatal("Should fail delegating to jailed")
		}
	})

	t.Run("D-14_MaxUnbonding", func(t *testing.T) {
		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(300))
		utils.AssertNoError(t, err, "setup user failed")
		const undelegateBurst = 20
		epochLen := ctx.Config.Network.Epoch
		if epochLen == 0 {
			epochBig, errEpoch := ctx.Proposal.Epoch(nil)
			if errEpoch == nil && epochBig != nil && epochBig.Sign() > 0 {
				epochLen = epochBig.Uint64()
			}
		}
		robustDelegate(t, userKey, valAddr, utils.ToWei(100))
		if epochLen > 0 {
			curHeight, errH := ctx.Clients[0].BlockNumber(context.Background())
			utils.AssertNoError(t, errH, "read current block failed")
			remaining := epochLen - (curHeight % epochLen)
			// Keep enough room to submit and mine the whole burst in the same epoch.
			// When we are too close to boundary, proactively cross it once instead of
			// falling back to 20 slow per-entry retries.
			// Full burst can still succeed with modest headroom because multiple txs can
			// be packed per block; keep threshold low to avoid unnecessary pre-waits.
			const minHeadroom = 8
			if remaining <= minHeadroom {
				waitN := int(remaining + 2) // cross epoch block + one extra safe block
				t.Logf("Near epoch boundary (remaining=%d), waiting %d blocks for burst headroom", remaining, waitN)
				waitBlocks(t, waitN)
			}
		}
		ctx.WaitIfEpochBlock()
		prevCnt, _ := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
		if prevCnt == nil {
			prevCnt = big.NewInt(0)
		}

		txHashes := make([]common.Hash, 0, undelegateBurst)
		baseNonce, errNonce := ctx.Clients[0].PendingNonceAt(context.Background(), userAddr)
		utils.AssertNoError(t, errNonce, "read pending nonce for undelegate burst failed")
		for i := 0; i < undelegateBurst; i++ {
			opts, errG := ctx.GetTransactorNoEpochWait(userKey, i == 0)
			if errG != nil {
				t.Logf("undelegate burst halted before submit %d/%d: %v", i, undelegateBurst, errG)
				break
			}
			opts.Nonce = new(big.Int).SetUint64(baseNonce + uint64(i))
			tx, errSubmit := ctx.Staking.Undelegate(opts, valAddr, utils.ToWei(1))
			if errSubmit != nil {
				msg := strings.ToLower(errSubmit.Error())
				if strings.Contains(msg, "epoch block forbidden") ||
					strings.Contains(msg, "nonce") ||
					strings.Contains(msg, "insufficient delegation") {
					t.Logf("undelegate burst submit fallback at %d/%d: %v", i, undelegateBurst, errSubmit)
					break
				}
				t.Fatalf("undelegate burst submit failed: %v", errSubmit)
			}
			txHashes = append(txHashes, tx.Hash())
		}

		scanConfirmed := func() int {
			ok := 0
			for _, h := range txHashes {
				receipt, errR := ctx.Clients[0].TransactionReceipt(context.Background(), h)
				if errR != nil || receipt == nil {
					continue
				}
				if receipt.Status == 1 {
					ok++
				}
			}
			return ok
		}
		confirmed := scanConfirmed()
		if confirmed < len(txHashes) && len(txHashes) > 0 {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 2,
				Interval:    retryAfterBlockInterval(),
				OnRetry: func(int) {
					waitNextBlock()
				},
			}, func() (bool, error) {
				confirmed = scanConfirmed()
				// Only give burst a very short grace window, then rely on fast fill.
				return confirmed > 0, nil
			})
		}

		expectedMin := new(big.Int).Add(prevCnt, big.NewInt(undelegateBurst))
		curCnt, errCnt := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
		utils.AssertNoError(t, errCnt, "read unbonding count after burst failed")
		if curCnt == nil {
			curCnt = big.NewInt(0)
		}
		if curCnt.Cmp(expectedMin) < 0 {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 4,
				Interval:    retryAfterBlockInterval(),
				OnRetry: func(int) {
					waitNextBlock()
				},
			}, func() (bool, error) {
				latest, err := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
				if err != nil {
					return false, err
				}
				if latest != nil {
					curCnt = latest
				}
				return latest != nil && latest.Cmp(expectedMin) >= 0, nil
			})
		}
		if curCnt.Cmp(expectedMin) < 0 {
			missingBI := new(big.Int).Sub(expectedMin, curCnt)
			if !missingBI.IsInt64() {
				t.Fatalf("invalid missing count after undelegate burst: %s", missingBI.String())
			}
			missing := int(missingBI.Int64())
			t.Logf("undelegate burst confirmed=%d/%d current=%s target=%s; filling missing=%d",
				confirmed, len(txHashes), curCnt.String(), expectedMin.String(), missing)

			isTransientUndelegateErr := func(msg string) bool {
				lower := strings.ToLower(msg)
				return strings.Contains(lower, "epoch block forbidden") ||
					strings.Contains(lower, "nonce") ||
					strings.Contains(lower, "timeout waiting for tx") ||
					strings.Contains(lower, "revert") ||
					strings.Contains(lower, "reverted")
			}
			batchFillMissing := func(target int) (int, error) {
				if target <= 0 {
					return 0, nil
				}
				baseNonce, errNonce := ctx.Clients[0].PendingNonceAt(context.Background(), userAddr)
				if errNonce != nil {
					return 0, errNonce
				}
				submitted := 0
				for i := 0; i < target; i++ {
					opts, errOpts := ctx.GetTransactorNoEpochWait(userKey, i == 0)
					if errOpts != nil {
						break
					}
					opts.Nonce = new(big.Int).SetUint64(baseNonce + uint64(i))
					txFill, errFill := ctx.Staking.Undelegate(opts, valAddr, utils.ToWei(1))
					if errFill != nil {
						if isTransientUndelegateErr(errFill.Error()) {
							break
						}
						return submitted, errFill
					}
					txHashes = append(txHashes, txFill.Hash())
					submitted++
				}
				return submitted, nil
			}
			fastUndelegateOne := func() error {
				for retry := 0; retry < 6; retry++ {
					ctx.WaitIfEpochBlock()
					opts, errOpts := ctx.GetTransactorNoEpochWait(userKey, retry == 0)
					if errOpts != nil {
						waitNextBlock()
						continue
					}
					txFill, errFill := ctx.Staking.Undelegate(opts, valAddr, utils.ToWei(1))
					if errFill != nil {
						if isTransientUndelegateErr(errFill.Error()) {
							waitNextBlock()
							continue
						}
						return errFill
					}
					if errMine := ctx.WaitMined(txFill.Hash()); errMine != nil {
						if isTransientUndelegateErr(errMine.Error()) {
							waitNextBlock()
							continue
						}
						return errMine
					}
					return nil
				}
				return fmt.Errorf("fast undelegate retries exhausted")
			}
			submitted, errBatch := batchFillMissing(missing)
			if errBatch != nil {
				t.Fatalf("batch fill undelegate failed: %v", errBatch)
			}
			if submitted > 0 {
				t.Logf("batch fill submitted %d/%d undelegate txs", submitted, missing)
				_ = testkit.WaitUntil(testkit.WaitUntilOptions{
					MaxAttempts: 5,
					Interval:    retryAfterBlockInterval(),
					OnRetry: func(int) {
						waitNextBlock()
					},
				}, func() (bool, error) {
					latest, err := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
					if err != nil {
						return false, err
					}
					if latest != nil {
						curCnt = latest
					}
					return latest != nil && latest.Cmp(expectedMin) >= 0, nil
				})
			}
			if curCnt.Cmp(expectedMin) >= 0 {
				goto finalizeD14
			}

			missingBI = new(big.Int).Sub(expectedMin, curCnt)
			if !missingBI.IsInt64() {
				t.Fatalf("invalid remaining missing count after batch fill: %s", missingBI.String())
			}
			missing = int(missingBI.Int64())
			for i := 0; i < missing; i++ {
				if errFill := fastUndelegateOne(); errFill != nil {
					t.Fatalf("fast fill undelegate %d/%d failed: %v", i+1, missing, errFill)
				}
			}
		}

	finalizeD14:
		err = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 4,
			Interval:    retryAfterBlockInterval(),
			OnRetry: func(int) {
				waitNextBlock()
			},
		}, func() (bool, error) {
			curCnt, err := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
			if err != nil {
				return false, err
			}
			return curCnt != nil && curCnt.Cmp(expectedMin) >= 0, nil
		})
		utils.AssertNoError(t, err, "unbonding entries did not reach max baseline in time")
		opts, _ := ctx.GetTransactor(userKey)
		_, err = ctx.Staking.Undelegate(opts, valAddr, utils.ToWei(1))
		if err == nil {
			t.Fatal("Should fail exceeding max entries")
		}
	})

	t.Run("D-19_InvalidMaxEntries", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(10))
		utils.AssertNoError(t, err, "setup user failed")
		opts, _ := ctx.GetTransactor(userKey)
		_, err = ctx.Staking.WithdrawUnbonded(opts, valAddr, big.NewInt(0))
		if err == nil {
			t.Fatal("Should fail with maxEntries=0")
		}
		_, err = ctx.Staking.WithdrawUnbonded(opts, valAddr, big.NewInt(21))
		if err == nil {
			t.Fatal("Should fail with maxEntries too large")
		}
	})
}
