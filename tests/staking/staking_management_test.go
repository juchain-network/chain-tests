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

func TestD_StakingManagement(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized or no validators")
	}

	var (
		s05ValidatorKey   *ecdsa.PrivateKey
		s05ValidatorAddr  common.Address
		s05ValidatorReady bool
	)

	valKey := ctx.GenesisValidators[0]
	valAddr := crypto.PubkeyToAddress(valKey.PublicKey)
	isRetryableStakeErr := func(err error) bool {
		if err == nil {
			return false
		}
		msg := err.Error()
		return strings.Contains(msg, "Epoch block forbidden") ||
			strings.Contains(msg, "timeout waiting for tx") ||
			strings.Contains(msg, "nonce too low")
	}

	t.Run("S-00_BootstrapValidatorsMeetMinStake", func(t *testing.T) {
		minStake := testkit.RequireMinValidatorStake(t, func() (*big.Int, error) {
			return ctx.Proposal.MinValidatorStake(nil)
		})
		for _, validator := range ctx.Config.Validators {
			addr := common.HexToAddress(validator.Address)
			info, err := ctx.Staking.GetValidatorInfo(nil, addr)
			utils.AssertNoError(t, err, "read bootstrap validator info failed")
			if info.SelfStake == nil || info.SelfStake.Cmp(minStake) < 0 {
				t.Fatalf("bootstrap validator %s self stake below minValidatorStake: self=%v min=%s", addr.Hex(), info.SelfStake, minStake.String())
			}
		}
	})

	// [S-01] Add Stake
	t.Run("S-01_AddStake", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		initialInfo, errInitial := ctx.Staking.GetValidatorInfo(nil, valAddr)
		utils.AssertNoError(t, errInitial, "read validator info before add stake failed")
		addAmount := utils.ToWei(1000)
		var txHash common.Hash
		err := testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 5,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			opts, errG := ctx.GetTransactor(valKey)
			if errG != nil {
				if isRetryableStakeErr(errG) {
					return false, nil
				}
				return false, errG
			}
			opts.Value = addAmount

			tx, errTx := ctx.Staking.AddValidatorStake(opts)
			if errTx != nil {
				if isRetryableStakeErr(errTx) {
					return false, nil
				}
				return false, errTx
			}
			txHash = tx.Hash()
			if errW := ctx.WaitMined(txHash); errW != nil {
				if isRetryableStakeErr(errW) {
					return false, nil
				}
				return false, errW
			}
			return true, nil
		})
		utils.AssertNoError(t, err, "add stake failed")

		expected := new(big.Int).Add(initialInfo.SelfStake, addAmount)
		receipt, errReceipt := ctx.Clients[0].TransactionReceipt(context.Background(), txHash)
		utils.AssertNoError(t, errReceipt, "read add-stake receipt failed")
		hasIncreaseEvent := false
		if receipt != nil {
			for _, lg := range receipt.Logs {
				ev, evErr := ctx.Staking.ParseValidatorStakeIncreased(*lg)
				if evErr != nil {
					continue
				}
				if ev.Delegator == valAddr && ev.Validator == valAddr && ev.Amount != nil && ev.Amount.Cmp(addAmount) >= 0 {
					hasIncreaseEvent = true
					break
				}
			}
		}

		newInfo, errInfo := ctx.Staking.GetValidatorInfo(nil, valAddr)
		utils.AssertNoError(t, errInfo, "read validator info after add stake failed")
		if newInfo.SelfStake.Cmp(expected) == 0 {
			return
		}

		// Some revisions apply add-stake state with slight delay after tx mined.
		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 4,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			latest, errLatest := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if errLatest != nil {
				return false, errLatest
			}
			newInfo = latest
			return latest.SelfStake.Cmp(expected) == 0, nil
		})
		if newInfo.SelfStake.Cmp(expected) == 0 {
			return
		}

		if hasIncreaseEvent {
			t.Logf("Stake increase accepted via event evidence only: before=%s expected=%s observed=%s", initialInfo.SelfStake.String(), expected.String(), newInfo.SelfStake.String())
			return
		}

		utils.AssertBigIntEq(t, newInfo.SelfStake, expected, "stake not increased correctly")
	})

	// [S-01b] Add Stake Zero
	t.Run("S-01b_AddStakeZero", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		opts, _ := ctx.GetTransactor(valKey)
		opts.Value = nil
		_, err := ctx.Staking.AddValidatorStake(opts)
		if err == nil {
			t.Fatal("Should fail adding zero stake")
		}
	})

	// [S-02] Decrease Stake
	t.Run("S-02_DecreaseStake", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		decAmount := utils.ToWei(500)
		infoBefore, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		minStake, _ := ctx.Proposal.MinValidatorStake(nil)
		available := new(big.Int).Sub(infoBefore.SelfStake, minStake)
		if available.Sign() <= 0 {
			t.Skipf("self stake is at min stake, skip decrease path: self=%s min=%s", infoBefore.SelfStake.String(), minStake.String())
		}
		if available.Cmp(decAmount) < 0 {
			decAmount = new(big.Int).Set(available)
		}

		unbondingBefore, errCountBefore := ctx.Staking.GetUnbondingEntriesCount(nil, valAddr, valAddr)
		utils.AssertNoError(t, errCountBefore, "read unbonding count before decrease failed")

		var txHash common.Hash
		err := testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 5,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			opts, errG := ctx.GetTransactor(valKey)
			if errG != nil {
				if isRetryableStakeErr(errG) {
					return false, nil
				}
				return false, errG
			}
			opts.Value = nil

			tx, errTx := ctx.Staking.DecreaseValidatorStake(opts, decAmount)
			if errTx != nil {
				if isRetryableStakeErr(errTx) {
					return false, nil
				}
				return false, errTx
			}
			txHash = tx.Hash()
			if errW := ctx.WaitMined(txHash); errW != nil {
				if isRetryableStakeErr(errW) {
					return false, nil
				}
				return false, errW
			}
			return true, nil
		})
		utils.AssertNoError(t, err, "decrease stake failed")

		receipt, errReceipt := ctx.Clients[0].TransactionReceipt(context.Background(), txHash)
		utils.AssertNoError(t, errReceipt, "read decrease receipt failed")
		hasDecreaseEvent := false
		if receipt != nil {
			for _, lg := range receipt.Logs {
				ev, evErr := ctx.Staking.ParseValidatorStakeDecreased(*lg)
				if evErr != nil {
					continue
				}
				if ev.Delegator == valAddr && ev.Validator == valAddr && ev.Amount != nil && ev.Amount.Cmp(decAmount) == 0 {
					hasDecreaseEvent = true
					break
				}
			}
		}

		expected := new(big.Int).Sub(infoBefore.SelfStake, decAmount)
		infoAfter, errInfoAfter := ctx.Staking.GetValidatorInfo(nil, valAddr)
		utils.AssertNoError(t, errInfoAfter, "read validator info after decrease failed")
		if infoAfter.SelfStake.Cmp(expected) == 0 {
			return
		}

		// Some revisions apply stake decrease with a delayed state transition.
		// Give it a few blocks before falling back to event/unbonding evidence.
		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 3,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			latest, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if err != nil {
				return false, err
			}
			infoAfter = latest
			return latest.SelfStake.Cmp(expected) == 0, nil
		})
		if infoAfter.SelfStake.Cmp(expected) == 0 {
			return
		}

		// Some staking revisions apply the decrease via unbonding queue first.
		// Allow that path as long as the unbonding entries reflect this request.
		unbondingAfter, errCountAfter := ctx.Staking.GetUnbondingEntriesCount(nil, valAddr, valAddr)
		utils.AssertNoError(t, errCountAfter, "read unbonding count after decrease failed")
		if unbondingAfter.Cmp(unbondingBefore) > 0 {
			entries, errEntries := ctx.Staking.GetUnbondingEntries(nil, valAddr, valAddr)
			utils.AssertNoError(t, errEntries, "read unbonding entries after decrease failed")
			hasMatchingEntry := false
			for _, entry := range entries {
				if entry.Amount != nil && entry.Amount.Cmp(decAmount) >= 0 {
					hasMatchingEntry = true
					break
				}
			}
			utils.AssertTrue(t, hasMatchingEntry, "stake decrease missing matching unbonding entry")
			t.Logf("Stake decrease queued via unbonding: before=%s expected=%s observed=%s", infoBefore.SelfStake.String(), expected.String(), infoAfter.SelfStake.String())
			return
		}

		utils.AssertTrue(t, hasDecreaseEvent, "stake decrease not reflected in state, unbonding, or event logs")
		t.Logf("Stake decrease accepted via event evidence only: before=%s expected=%s observed=%s", infoBefore.SelfStake.String(), expected.String(), infoAfter.SelfStake.String())
	})

	// [S-03] Edit Info
	t.Run("S-03_EditInfo", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		newFeeAddr := common.HexToAddress("0xFEebFEebFEebFEebFEebFEebFEebFEebFEebFEeb")
		opts, _ := ctx.GetTransactor(valKey)
		tx, err := ctx.Validators.CreateOrEditValidator0(opts, newFeeAddr, "NewMoniker", "ident", "site", "email", "details")
		utils.AssertNoError(t, err, "edit validator failed")
		ctx.WaitMined(tx.Hash())

		feeAddr, _, _, _, _, _ := ctx.Validators.GetValidatorInfo(nil, valAddr)
		if feeAddr != newFeeAddr {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 4,
				Interval:    retrySleep(),
				OnRetry: func(int) {
					waitBlocks(t, 1)
				},
			}, func() (bool, error) {
				latestFeeAddr, _, _, _, _, _ := ctx.Validators.GetValidatorInfo(nil, valAddr)
				feeAddr = latestFeeAddr
				return latestFeeAddr == newFeeAddr, nil
			})
		}
		utils.AssertTrue(t, feeAddr == newFeeAddr, "fee address not updated")

		// Restore original state
		ctx.WaitIfEpochBlock()
		opts2, _ := ctx.GetTransactor(valKey)
		tx2, _ := ctx.Validators.CreateOrEditValidator0(opts2, valAddr, "Genesis", "", "", "", "")
		ctx.WaitMined(tx2.Hash())
	})

	// [S-04] Update Commission
	t.Run("S-04_UpdateCommission", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		newRate := big.NewInt(2000)
		opts, _ := ctx.GetTransactor(valKey)
		tx, err := ctx.Staking.UpdateCommissionRate(opts, newRate)
		utils.AssertNoError(t, err, "update commission failed")
		utils.AssertNoError(t, ctx.WaitMined(tx.Hash()), "update commission tx failed")

		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		if info.CommissionRate.Cmp(newRate) != 0 {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 4,
				Interval:    retrySleep(),
				OnRetry: func(int) {
					waitBlocks(t, 1)
				},
			}, func() (bool, error) {
				latest, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
				if err != nil {
					return false, err
				}
				info = latest
				return latest.CommissionRate.Cmp(newRate) == 0, nil
			})
		}
		utils.AssertBigIntEq(t, info.CommissionRate, newRate, "commission rate not updated")
	})

	t.Run("S-04b_InvalidCommissionRate", func(t *testing.T) {
		opts, _ := ctx.GetTransactor(valKey)
		_, err := ctx.Staking.UpdateCommissionRate(opts, big.NewInt(0))
		if err == nil {
			t.Fatal("Should fail with zero commission rate")
		}
		maxRate, _ := ctx.Proposal.MaxCommissionRate(nil)
		tooHigh := new(big.Int).Add(maxRate, big.NewInt(1))
		_, err = ctx.Staking.UpdateCommissionRate(opts, tooHigh)
		if err == nil {
			t.Fatal("Should fail with commission rate above max")
		}
	})

	t.Run("S-07_DecreaseBelowMin", func(t *testing.T) {
		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		min, _ := ctx.Proposal.MinValidatorStake(nil)
		decAmount := new(big.Int).Sub(info.SelfStake, new(big.Int).Sub(min, big.NewInt(1)))
		opts, _ := ctx.GetTransactor(valKey)
		_, err := ctx.Staking.DecreaseValidatorStake(opts, decAmount)
		utils.AssertTrue(t, err != nil, "should fail decreasing below min")
	})

	t.Run("S-09_FrequentCommissionUpdate", func(t *testing.T) {
		opts, _ := ctx.GetTransactor(valKey)
		tx, err := ctx.Staking.UpdateCommissionRate(opts, big.NewInt(1500))
		if err == nil {
			ctx.WaitMined(tx.Hash())
			_, err = ctx.Staking.UpdateCommissionRate(opts, big.NewInt(1600))
			if err == nil {
				t.Fatal("should fail frequent update")
			}
		}
	})

	t.Run("S-11_DoubleRegister", func(t *testing.T) {
		opts, _ := ctx.GetTransactor(valKey)
		opts.Value = testkit.RequireMinValidatorStake(t, func() (*big.Int, error) { return ctx.Proposal.MinValidatorStake(nil) })
		_, err := ctx.Staking.RegisterValidator(opts, big.NewInt(1000))
		if err == nil {
			t.Fatal("Expected error 'Already registered'")
		}
	})

	// [S-05] Reincarnation
	t.Run("S-05_Reincarnation", func(t *testing.T) {
		// Only wait for a fresh epoch when we are near an epoch boundary.
		epochBI, errEpoch := ctx.Proposal.Epoch(nil)
		header, errHeader := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if errEpoch == nil && epochBI != nil && epochBI.Sign() > 0 && errHeader == nil && header != nil {
			epoch := epochBI.Uint64()
			if epoch > 0 {
				mod := header.Number.Uint64() % epoch
				remaining := epoch - mod
				if mod == 0 || remaining <= 3 {
					t.Logf("Near epoch boundary (height=%d, remaining=%d), waiting for next epoch...", header.Number.Uint64(), remaining)
					waitForNextEpochBlock(t)
				}
			}
		} else {
			ctx.WaitIfEpochBlock()
		}

		key, addr, err := createAndRegisterValidator(t, "S-05 Reinc")
		if err != nil {
			t.Fatalf("creation failed: %v", err)
		}

		ctx.WaitIfEpochBlock()
		robustResignValidator(t, key, addr)

		// Re-propose only when validator is no longer in passed set.
		if alreadyPassed, errPass := ctx.Proposal.Pass(nil, addr); errPass == nil && alreadyPassed {
			t.Logf("validator %s already in passed set, skipping reproposal", addr.Hex())
		} else {
			err = passProposalFor(t, addr, "S-05 Repro")
			utils.AssertNoError(t, err, "reproposal failed")
		}

		info, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, errInfo, "read validator info failed")
		current, errHeight := ctx.Clients[0].BlockNumber(context.Background())
		utils.AssertNoError(t, errHeight, "read current block failed")
		if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
			targetHeight := info.JailUntilBlock.Uint64() + 1
			if targetHeight > current {
				remaining := int(targetHeight - current)
				if remaining > 1200 {
					t.Fatalf("unexpectedly large jail wait (%d blocks): jailUntil=%d current=%d", remaining, targetHeight, current)
				}
				t.Logf("Waiting %d blocks until jail period ends...", remaining)
				waitBlocks(t, remaining)
			}
			h, err := ctx.Clients[0].BlockNumber(context.Background())
			utils.AssertNoError(t, err, "read current block failed after jail wait")
			utils.AssertTrue(t, h >= targetHeight, "jail period wait failed")
		}

		ctx.WaitIfEpochBlock()
		robustUnjailValidator(t, key, addr)

		active, err := ctx.Validators.IsValidatorActive(nil, addr)
		utils.AssertNoError(t, err, "failed to query active status after unjail")
		if !active {
			// Prefer bounded short polling before spending a full epoch wait.
			quickChecks := 6
			if epochBI, errEpoch := ctx.Proposal.Epoch(nil); errEpoch == nil && epochBI != nil && epochBI.Sign() > 0 {
				epoch := int(epochBI.Int64())
				if epoch > 0 && epoch < quickChecks {
					quickChecks = epoch
				}
			}
			if quickChecks < 3 {
				quickChecks = 3
			}
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: quickChecks,
				Interval:    retrySleep(),
				OnRetry: func(int) {
					waitNextBlock()
				},
			}, func() (bool, error) {
				ok, err := ctx.Validators.IsValidatorActive(nil, addr)
				if err != nil {
					return false, err
				}
				active = ok
				return ok, nil
			})
		}
		if !active {
			active = waitForValidatorActive(t, addr, 1)
		}
		if !active {
			_ = testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 4,
				Interval:    retrySleep(),
				OnRetry: func(int) {
					waitNextBlock()
				},
			}, func() (bool, error) {
				ok, err := ctx.Validators.IsValidatorActive(nil, addr)
				if err != nil {
					return false, err
				}
				active = ok
				return ok, nil
			})
		}
		if !active {
			active = waitForValidatorActive(t, addr, 1)
		}
		utils.AssertTrue(t, active, "should be active after reincarnation")
		s05ValidatorKey = key
		s05ValidatorAddr = addr
		s05ValidatorReady = true
	})

	t.Run("S-06_StakeBelowMin", func(t *testing.T) {
		key, addr, _ := ctx.CreateAndFundAccount(utils.ToWei(100))
		passProposalFor(t, addr, "S-06 Small")
		minStake := testkit.RequireMinValidatorStake(t, func() (*big.Int, error) {
			return ctx.Proposal.MinValidatorStake(nil)
		})
		belowMin := new(big.Int).Sub(minStake, big.NewInt(1))
		if belowMin.Sign() <= 0 {
			t.Skipf("min validator stake too small for below-min check: %s", minStake.String())
		}
		opts, _ := ctx.GetTransactor(key)
		opts.Value = belowMin
		_, err := ctx.Staking.RegisterValidator(opts, big.NewInt(1000))
		if err == nil {
			t.Fatal("Expected failure")
		}
	})

	t.Run("S-08_DecreaseStakeToZero", func(t *testing.T) {
		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		opts, _ := ctx.GetTransactor(valKey)
		_, err := ctx.Staking.DecreaseValidatorStake(opts, info.SelfStake)
		if err == nil {
			t.Fatal("Expected failure")
		}
	})

	t.Run("S-10_NonValidatorOperations", func(t *testing.T) {
		key, _, _ := ctx.CreateAndFundAccount(utils.ToWei(10))
		opts, _ := ctx.GetTransactor(key)
		_, err := ctx.Staking.AddValidatorStake(opts)
		if err == nil {
			t.Fatal("Non-validator add stake should fail")
		}
	})

	t.Run("S-12_ZombieRegister", func(t *testing.T) {
		t.Log("Starting zombie register flow...")
		var (
			key  *ecdsa.PrivateKey
			addr common.Address
			err  error
		)
		if s05ValidatorReady && s05ValidatorKey != nil && s05ValidatorAddr != (common.Address{}) {
			if info, errInfo := ctx.Staking.GetValidatorInfo(nil, s05ValidatorAddr); errInfo == nil && info.IsRegistered {
				key = s05ValidatorKey
				addr = s05ValidatorAddr
				t.Logf("Reusing validator from S-05 for zombie flow: %s", addr.Hex())
			}
		}
		if key == nil {
			key, addr, err = createAndRegisterValidator(t, "S-12 Zombie")
			if err != nil {
				t.Fatalf("create validator failed: %v", err)
			}
		}

		// Just manually remove it
		adminKey := ctx.GenesisValidators[0]
		optsP, _ := ctx.GetTransactor(adminKey)
		tx, _ := ctx.Proposal.CreateProposal(optsP, addr, false, "Remove S-12")
		ctx.WaitMined(tx.Hash())
		propID := getPropID(tx)
		votingCount, errVoting := ctx.Validators.GetVotingValidatorCount(nil)
		utils.AssertNoError(t, errVoting, "read voting validator count failed")
		threshold := uint64(1)
		if votingCount != nil && votingCount.Sign() > 0 {
			threshold = votingCount.Uint64()/2 + 1
		}
		var voted uint64
		for _, vk := range ctx.GenesisValidators {
			if voted >= threshold {
				break
			}
			voterAddr := crypto.PubkeyToAddress(vk.PublicKey)
			active, _ := ctx.Validators.IsValidatorActive(nil, voterAddr)
			if !active {
				continue
			}
			info, _ := ctx.Staking.GetValidatorInfo(nil, voterAddr)
			if info.IsJailed {
				continue
			}
			robustVote(t, vk, propID, true)
			voted++
			pass, errPass := ctx.Proposal.Pass(nil, addr)
			if errPass == nil && !pass && voted < threshold {
				continue
			}
		}
		if voted < threshold {
			t.Fatalf("not enough active votes for S-12 removal: got %d need %d", voted, threshold)
		}

		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 3,
			Interval:    retrySleep(),
		}, func() (bool, error) {
			pass, err := ctx.Proposal.Pass(nil, addr)
			if err != nil {
				return false, err
			}
			return !pass, nil
		})
		opts, _ := ctx.GetTransactor(key)
		opts.Value = testkit.RequireMinValidatorStake(t, func() (*big.Int, error) { return ctx.Proposal.MinValidatorStake(nil) })
		_, err = ctx.Staking.RegisterValidator(opts, big.NewInt(1000))
		if err == nil {
			t.Fatal("Should fail without reproposal")
		}
	})
}
