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

func TestD_StakingManagement(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized or no validators")
	}

	valKey := ctx.GenesisValidators[0]
	valAddr := crypto.PubkeyToAddress(valKey.PublicKey)

	// [S-01] Add Stake
	t.Run("S-01_AddStake", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		initialInfo, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		addAmount := utils.ToWei(1000)
		opts, _ := ctx.GetTransactor(valKey)
		opts.Value = addAmount

		tx, err := ctx.Staking.AddValidatorStake(opts)
		utils.AssertNoError(t, err, "add stake failed")
		ctx.WaitMined(tx.Hash())

		newInfo, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		expected := new(big.Int).Add(initialInfo.SelfStake, addAmount)
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
		opts, _ := ctx.GetTransactor(valKey)
		opts.Value = nil

		infoBefore, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		tx, err := ctx.Staking.DecreaseValidatorStake(opts, decAmount)
		utils.AssertNoError(t, err, "decrease stake failed")
		ctx.WaitMined(tx.Hash())

		infoAfter, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		expected := new(big.Int).Sub(infoBefore.SelfStake, decAmount)
		utils.AssertBigIntEq(t, infoAfter.SelfStake, expected, "stake not decreased correctly")
	})

	// [S-03] Edit Info
	t.Run("S-03_EditInfo", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		newFeeAddr := common.HexToAddress("0xFEebFEebFEebFEebFEebFEebFEebFEebFEebFEeb")
		opts, _ := ctx.GetTransactor(valKey)
		tx, err := ctx.Validators.CreateOrEditValidator(opts, newFeeAddr, "NewMoniker", "ident", "site", "email", "details")
		utils.AssertNoError(t, err, "edit validator failed")
		ctx.WaitMined(tx.Hash())

		feeAddr, _, _, _, _, _ := ctx.Validators.GetValidatorInfo(nil, valAddr)
		utils.AssertTrue(t, feeAddr == newFeeAddr, "fee address not updated")

		// Restore original state
		ctx.WaitIfEpochBlock()
		opts2, _ := ctx.GetTransactor(valKey)
		tx2, _ := ctx.Validators.CreateOrEditValidator(opts2, valAddr, "Genesis", "", "", "", "")
		ctx.WaitMined(tx2.Hash())
	})

	// [S-04] Update Commission
	t.Run("S-04_UpdateCommission", func(t *testing.T) {
		ctx.WaitIfEpochBlock()
		newRate := big.NewInt(2000)
		opts, _ := ctx.GetTransactor(valKey)
		tx, err := ctx.Staking.UpdateCommissionRate(opts, newRate)
		utils.AssertNoError(t, err, "update commission failed")
		ctx.WaitMined(tx.Hash())

		info, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
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
		opts.Value = utils.ToWei(100000)
		_, err := ctx.Staking.RegisterValidator(opts, big.NewInt(1000))
		if err == nil {
			t.Fatal("Expected error 'Already registered'")
		}
	})

	// [S-05] Reincarnation
	t.Run("S-05_Reincarnation", func(t *testing.T) {
		t.Log("Waiting for fresh epoch...")
		waitForNextEpochBlock(t)

		key, addr, err := createAndRegisterValidator(t, "S-05 Reinc")
		if err != nil {
			t.Fatalf("creation failed: %v", err)
		}

		ctx.WaitIfEpochBlock()
		opts, _ := ctx.GetTransactor(key)
		tx, err := ctx.Staking.ResignValidator(opts)
		utils.AssertNoError(t, err, "resign failed")
		ctx.WaitMined(tx.Hash())

		// Pass reproposal
		err = passProposalFor(t, addr, "S-05 Repro")
		utils.AssertNoError(t, err, "reproposal failed")

		info, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, errInfo, "read validator info failed")
		current, errHeight := ctx.Clients[0].BlockNumber(context.Background())
		utils.AssertNoError(t, errHeight, "read current block failed")
		if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
			targetHeight := info.JailUntilBlock.Uint64() + 1
			maxAttempts := 2
			if targetHeight > current {
				remaining := int(targetHeight - current)
				if remaining > 1200 {
					t.Fatalf("unexpectedly large jail wait (%d blocks): jailUntil=%d current=%d", remaining, targetHeight, current)
				}
				t.Logf("Waiting up to %d blocks until jail period ends...", remaining)
				maxAttempts = remaining + 2
			}
			err = testkit.WaitUntil(testkit.WaitUntilOptions{
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
			utils.AssertNoError(t, err, "jail period wait failed")
		}

		ctx.WaitIfEpochBlock()
		robustUnjailValidator(t, key, addr)

		active := waitForValidatorActive(t, addr, 2)
		utils.AssertTrue(t, active, "should be active after reincarnation")
	})

	t.Run("S-06_StakeBelowMin", func(t *testing.T) {
		key, addr, _ := ctx.CreateAndFundAccount(utils.ToWei(100))
		passProposalFor(t, addr, "S-06 Small")
		opts, _ := ctx.GetTransactor(key)
		opts.Value = utils.ToWei(0.5) // Below 1 ETH min
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
		key, addr, err := createAndRegisterValidator(t, "S-12 Zombie")
		if err != nil {
			t.Fatalf("create validator failed: %v", err)
		}

		// Just manually remove it
		adminKey := ctx.GenesisValidators[0]
		optsP, _ := ctx.GetTransactor(adminKey)
		tx, _ := ctx.Proposal.CreateProposal(optsP, addr, false, "Remove S-12")
		ctx.WaitMined(tx.Hash())
		propID := getPropID(tx)
		for _, vk := range ctx.GenesisValidators {
			robustVote(t, vk, propID, true)
		}

		_ = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 3,
			Interval:    100 * time.Millisecond,
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			pass, err := ctx.Proposal.Pass(nil, addr)
			if err != nil {
				return false, err
			}
			return !pass, nil
		})
		opts, _ := ctx.GetTransactor(key)
		opts.Value = utils.ToWei(100000)
		_, err = ctx.Staking.RegisterValidator(opts, big.NewInt(1000))
		if err == nil {
			t.Fatal("Should fail without reproposal")
		}
	})
}
