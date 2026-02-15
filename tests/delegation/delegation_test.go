package tests

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/utils"
)

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
	ctx.EnsureConfig(6, big.NewInt(10), unb)
	unj, _ := ctx.Proposal.ValidatorUnjailPeriod(nil)
	ctx.EnsureConfig(7, big.NewInt(10), unj)

	t.Run("D-01_FullFlow", func(t *testing.T) {
		userKey, userAddr, err := ctx.CreateAndFundAccount(utils.ToWei(200))
		utils.AssertNoError(t, err, "failed to setup delegator")

		t.Logf("User %s delegating 100 ETH to %s...", userAddr.Hex(), valAddr.Hex())
		delegateAmount := utils.ToWei(100)
		robustDelegate(t, userKey, valAddr, delegateAmount)

		info, _ := ctx.Staking.GetDelegationInfo(nil, userAddr, valAddr)
		utils.AssertBigIntEq(t, info.Amount, delegateAmount, "delegation amount mismatch")

		t.Log("Waiting for some blocks to accumulate rewards...")
		waitBlocks(t, 2)

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

		period, _ := ctx.Proposal.UnbondingPeriod(nil)
		waitBlocks(t, int(new(big.Int).Add(period, big.NewInt(1)).Int64()))

		robustWithdrawUnbonded(t, userKey, valAddr, 20)

		cnt, _ := ctx.Staking.GetUnbondingEntriesCount(nil, userAddr, valAddr)
		utils.AssertTrue(t, cnt.Sign() == 0, "unbonding entries should be cleared")
	})

	t.Run("D-02_ClaimCommission", func(t *testing.T) {
		infoBefore, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		t.Logf("Initial accumulated rewards: %s", infoBefore.AccumulatedRewards.String())
		claimedBefore := new(big.Int).Set(infoBefore.TotalClaimedRewards)

		success := false
		for i := 0; i < 20; i++ {
			waitBlocks(t, 1)
			infoNow, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if infoNow.AccumulatedRewards.Sign() == 0 {
				continue
			}

			robustClaimValidatorRewards(t, valKey)
			infoAfter, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if infoAfter.TotalClaimedRewards.Cmp(claimedBefore) > 0 {
				success = true
				break
			}
		}
		utils.AssertTrue(t, success, "total claimed rewards should increase")
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
		key, addr, err := createAndRegisterValidator(t, "D-04a Val")
		utils.AssertNoError(t, err, "setup validator failed")
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(500))
		utils.AssertNoError(t, err, "setup user failed")
		robustDelegate(t, userKey, addr, utils.ToWei(10))

		vOpts, _ := ctx.GetTransactor(key)
		txR, _ := ctx.Staking.ResignValidator(vOpts)
		ctx.WaitMined(txR.Hash())

		opts, _ := ctx.GetTransactor(userKey)
		opts.Value = utils.ToWei(5)
		_, err = ctx.Staking.Delegate(opts, addr)
		if err == nil {
			t.Fatal("Should not be able to delegate to resigned validator")
		}
	})

	t.Run("D-04b_MultiDelegatorIsolation", func(t *testing.T) {
		keyA, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user A failed")
		keyB, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user B failed")

		robustDelegate(t, keyA, valAddr, utils.ToWei(10))
		robustDelegate(t, keyB, valAddr, utils.ToWei(20))

		waitBlocks(t, 1)
		infoA, _ := ctx.Staking.GetDelegationInfo(nil, crypto.PubkeyToAddress(keyA.PublicKey), valAddr)
		infoB, _ := ctx.Staking.GetDelegationInfo(nil, crypto.PubkeyToAddress(keyB.PublicKey), valAddr)
		utils.AssertBigIntEq(t, infoA.Amount, utils.ToWei(10), "User A amount mismatch")
		utils.AssertBigIntEq(t, infoB.Amount, utils.ToWei(20), "User B amount mismatch")
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
		waitBlocks(t, 1)
		robustDelegate(t, userKey, valAddr, utils.ToWei(10))
		info, _ := ctx.Staking.GetDelegationInfo(nil, userAddr, valAddr)
		utils.AssertBigIntEq(t, info.Amount, utils.ToWei(20), "total amount mismatch")
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
		waitBlocks(t, 1)

		var lastErr error
		for retry := 0; retry < 10; retry++ {
			opts, errG := ctx.GetTransactor(userKey)
			if errG != nil {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			opts.Value = utils.ToWei(100000)
			opts.GasLimit = 5000000

			txReg, err := ctx.Staking.RegisterValidator(opts, big.NewInt(500))
			if err != nil {
				lastErr = err
				if strings.Contains(err.Error(), "Epoch block forbidden") || strings.Contains(err.Error(), "Too many new validators") {
					waitForNextEpochBlock(t)
					waitBlocks(t, 1)
					continue
				}
				break
			}

			if errW := ctx.WaitMined(txReg.Hash()); errW != nil {
				lastErr = errW
				if strings.Contains(errW.Error(), "revert") || strings.Contains(errW.Error(), "reverted") {
					ctx.RefreshNonce(userAddr)
					waitForNextEpochBlock(t)
					waitBlocks(t, 1)
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
				waitForNextEpochBlock(t)
				waitBlocks(t, 1)
				continue
			}

			isVal, _ := ctx.Validators.IsValidatorExist(nil, userAddr)
			utils.AssertTrue(t, isVal, "should be validator")
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
		info1, _ := ctx.Staking.GetDelegationInfo(nil, userAddr, val1)
		info2, _ := ctx.Staking.GetDelegationInfo(nil, userAddr, val2)
		utils.AssertBigIntEq(t, info1.Amount, utils.ToWei(10), "V1 delegation failed")
		utils.AssertBigIntEq(t, info2.Amount, utils.ToWei(10), "V2 delegation failed")
	})

	t.Run("D-16_CircularDelegation", func(t *testing.T) {
		v0Key := ctx.GenesisValidators[0]
		v0Addr := common.HexToAddress(ctx.Config.Validators[0].Address)
		v1Key := ctx.GenesisValidators[1]
		v1Addr := common.HexToAddress(ctx.Config.Validators[1].Address)
		robustDelegate(t, v0Key, v1Addr, utils.ToWei(10))
		robustDelegate(t, v1Key, v0Addr, utils.ToWei(10))
		info0, _ := ctx.Staking.GetDelegationInfo(nil, v0Addr, v1Addr)
		info1, _ := ctx.Staking.GetDelegationInfo(nil, v1Addr, v0Addr)
		utils.AssertBigIntEq(t, info0.Amount, utils.ToWei(10), "V0->V1 check failed")
		utils.AssertBigIntEq(t, info1.Amount, utils.ToWei(10), "V1->V0 check failed")
	})

	t.Run("D-17_RoleDowngrade", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "D-17 Downgrade")
		if err != nil {
			t.Logf("Skipping D-17: %v", err)
			return
		}
		opts, _ := ctx.GetTransactor(key)
		ctx.Staking.ResignValidator(opts)
		unjailPeriod, _ := ctx.Proposal.ValidatorUnjailPeriod(nil)
		// Wait until jail period is fully passed, then cross epoch for set update.
		info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
		current, _ := ctx.Clients[0].BlockNumber(context.Background())
		if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
			jailUntil := info.JailUntilBlock.Uint64()
			if current < jailUntil {
				waitBlocks(t, int(jailUntil-current+1))
			}
		} else if unjailPeriod != nil && unjailPeriod.Sign() > 0 {
			waitBlocks(t, int(new(big.Int).Add(unjailPeriod, big.NewInt(1)).Int64()))
		}
		waitForNextEpochBlock(t)
		robustExitValidator(t, key)
		t.Logf("Ex-validator %s delegating to %s", addr.Hex(), valAddr.Hex())
		robustDelegate(t, key, valAddr, utils.ToWei(10))
		waitBlocks(t, 1)
		delegationInfo, _ := ctx.Staking.GetDelegationInfo(nil, addr, valAddr)
		utils.AssertBigIntEq(t, delegationInfo.Amount, utils.ToWei(10), "Delegation check failed")
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

	t.Run("D-12_DelegateToJailed", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "D-12 Jailed")
		utils.AssertNoError(t, err, "setup validator failed")
		vOpts, _ := ctx.GetTransactor(key)
		ctx.Staking.ResignValidator(vOpts)
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(50))
		utils.AssertNoError(t, err, "setup user failed")
		opts, _ := ctx.GetTransactor(userKey)
		opts.Value = utils.ToWei(10)
		_, err = ctx.Staking.Delegate(opts, addr)
		if err == nil {
			t.Fatal("Should fail delegating to jailed")
		}
	})

	t.Run("D-13_UndelegateFromJailed", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "D-13 Jailed")
		utils.AssertNoError(t, err, "setup validator failed")
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(50))
		utils.AssertNoError(t, err, "setup user failed")
		robustDelegate(t, userKey, addr, utils.ToWei(10))
		vOpts, _ := ctx.GetTransactor(key)
		ctx.Staking.ResignValidator(vOpts)
		robustUndelegate(t, userKey, addr, utils.ToWei(10))
	})

	t.Run("D-14_MaxUnbonding", func(t *testing.T) {
		userKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(100))
		utils.AssertNoError(t, err, "setup user failed")
		robustDelegate(t, userKey, valAddr, utils.ToWei(25))
		for i := 0; i < 20; i++ {
			robustUndelegate(t, userKey, valAddr, utils.ToWei(1))
			waitBlocks(t, 1)
		}
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
