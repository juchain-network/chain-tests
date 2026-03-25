package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

const (
	rewardsConfigIDMaxValidators    int64 = 9
	rewardsConfigIDDoubleSignWindow int64 = 15
)

type validatorStakeSnapshot struct {
	key        *ecdsa.PrivateKey
	addr       common.Address
	selfStake  *big.Int
	totalStake *big.Int
}

func genesisValidatorAddresses() []common.Address {
	addrs := make([]common.Address, 0, len(ctx.GenesisValidators))
	for _, key := range ctx.GenesisValidators {
		addrs = append(addrs, crypto.PubkeyToAddress(key.PublicKey))
	}
	return addrs
}

func activeGenesisValidatorStakes(t *testing.T) []validatorStakeSnapshot {
	t.Helper()

	snapshots := make([]validatorStakeSnapshot, 0, len(ctx.GenesisValidators))
	for _, key := range ctx.GenesisValidators {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		active, err := ctx.Validators.IsValidatorActive(nil, addr)
		utils.AssertNoError(t, err, "read validator active state failed")
		if !active {
			continue
		}

		info, err := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, err, "read validator staking info failed")
		if !info.IsRegistered || info.IsJailed {
			continue
		}

		totalStake := new(big.Int).Set(info.SelfStake)
		if info.TotalDelegated != nil {
			totalStake = new(big.Int).Add(totalStake, info.TotalDelegated)
		}
		snapshots = append(snapshots, validatorStakeSnapshot{
			key:        key,
			addr:       addr,
			selfStake:  new(big.Int).Set(info.SelfStake),
			totalStake: totalStake,
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		cmp := snapshots[i].totalStake.Cmp(snapshots[j].totalStake)
		if cmp != 0 {
			return cmp < 0
		}
		return strings.ToLower(snapshots[i].addr.Hex()) < strings.ToLower(snapshots[j].addr.Hex())
	})

	return snapshots
}

func formatGenesisValidatorBaselineState(t *testing.T) string {
	t.Helper()

	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		return "<unavailable>"
	}

	parts := make([]string, 0, len(ctx.GenesisValidators))
	for _, key := range ctx.GenesisValidators {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		active, activeErr := ctx.Validators.IsValidatorActive(nil, addr)
		pass, passErr := ctx.Proposal.Pass(nil, addr)
		info, infoErr := ctx.Staking.GetValidatorInfo(nil, addr)

		if activeErr != nil || passErr != nil || infoErr != nil {
			parts = append(
				parts,
				fmt.Sprintf(
					"%s activeErr=%v passErr=%v infoErr=%v",
					addr.Hex(),
					activeErr,
					passErr,
					infoErr,
				),
			)
			continue
		}

		selfStake := "<nil>"
		totalStake := "<nil>"
		if info.SelfStake != nil {
			selfStake = info.SelfStake.String()
			total := new(big.Int).Set(info.SelfStake)
			if info.TotalDelegated != nil {
				total = new(big.Int).Add(total, info.TotalDelegated)
			}
			totalStake = total.String()
		}

		parts = append(
			parts,
			fmt.Sprintf(
				"%s active=%t pass=%t registered=%t jailed=%t selfStake=%s totalStake=%s",
				addr.Hex(),
				active,
				pass,
				info.IsRegistered,
				info.IsJailed,
				selfStake,
				totalStake,
			),
		)
	}

	return strings.Join(parts, "; ")
}

func ensureMinActiveGenesisValidators(t *testing.T, min int, maxEpochs int) {
	t.Helper()

	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if min < 1 {
		min = 1
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}

	for attempt := 0; attempt < maxEpochs; attempt++ {
		snapshots := activeGenesisValidatorStakes(t)
		if len(snapshots) >= min {
			return
		}
		waitForNextEpochBlock(t)
	}

	snapshots := activeGenesisValidatorStakes(t)
	activeGenesis := make([]common.Address, 0, len(snapshots))
	for _, snapshot := range snapshots {
		activeGenesis = append(activeGenesis, snapshot.addr)
	}
	activeAll, _ := ctx.Validators.GetActiveValidators(nil)
	t.Fatalf(
		"active genesis validators < %d (got %d): active=%s activeGenesis=%s genesisState={%s}",
		min,
		len(snapshots),
		formatAddressList(activeAll),
		formatAddressList(activeGenesis),
		formatGenesisValidatorBaselineState(t),
	)
}

func waitValidatorProposalPassState(t *testing.T, valAddr common.Address, want bool) {
	t.Helper()

	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 8,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		pass, err := ctx.Proposal.Pass(nil, valAddr)
		if err != nil {
			return false, err
		}
		return pass == want, nil
	})
	utils.AssertNoError(t, err, fmt.Sprintf("wait for proposal pass=%v failed", want))
}

func finalizeValidatorProposal(t *testing.T, valAddr common.Address, wantPass bool, desc string) {
	t.Helper()

	pass, err := ctx.Proposal.Pass(nil, valAddr)
	utils.AssertNoError(t, err, "read proposal pass state failed")
	if pass == wantPass {
		return
	}

	var (
		tx      *types.Transaction
		lastErr error
	)
	for attempt := 0; attempt < 8; attempt++ {
		proposerKey := getActiveProposerOrSkip(t, 2)
		ctx.WaitIfEpochBlock()
		opts, err := ctx.GetTransactor(proposerKey)
		if err != nil {
			lastErr = err
			waitBlocks(t, 1)
			continue
		}
		tx, err = ctx.Proposal.CreateProposal(opts, valAddr, wantPass, desc)
		if err == nil {
			errW := ctx.WaitMined(tx.Hash())
			if errW == nil {
				lastErr = nil
				break
			}
			err = errW
		}

		lastErr = err
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "epoch block forbidden") ||
			strings.Contains(msg, "too frequent") ||
			strings.Contains(msg, "timeout waiting for tx") ||
			strings.Contains(msg, "revert") {
			waitBlocks(t, 1)
			continue
		}
		t.Fatalf("create proposal %q failed: %v", desc, err)
	}
	if lastErr != nil {
		t.Fatalf("create proposal %q failed: %v", desc, lastErr)
	}
	if tx == nil {
		t.Fatalf("create proposal %q returned no tx", desc)
	}

	propID := getPropID(tx)
	if propID == ([32]byte{}) {
		t.Fatalf("missing proposal id for %q", desc)
	}

	for round := 0; round < 4; round++ {
		for _, vk := range ctx.GenesisValidators {
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

			pass, err := ctx.Proposal.Pass(nil, valAddr)
			if err == nil && pass == wantPass {
				return
			}
		}
		waitBlocks(t, 1)
	}

	waitValidatorProposalPassState(t, valAddr, wantPass)
}

func waitForValidatorFeeAddr(t *testing.T, valAddr, want common.Address) {
	t.Helper()

	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 6,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		feeAddr, _, _, _, _, err := ctx.Validators.GetValidatorInfo(nil, valAddr)
		if err != nil {
			return false, err
		}
		return feeAddr == want, nil
	})
	utils.AssertNoError(t, err, fmt.Sprintf("wait for validator fee address %s failed", want.Hex()))
}

func waitForIncomingProfits(t *testing.T, valAddr common.Address, maxBlocks int) *big.Int {
	t.Helper()
	if maxBlocks < 1 {
		maxBlocks = 1
	}

	var incoming *big.Int
	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: maxBlocks + 1,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		_, _, latestIncoming, _, _, err := ctx.Validators.GetValidatorInfo(nil, valAddr)
		if err != nil {
			return false, err
		}
		incoming = latestIncoming
		return latestIncoming != nil && latestIncoming.Sign() > 0, nil
	})
	utils.AssertNoError(t, err, "validator did not accrue incoming profits in time")
	return new(big.Int).Set(incoming)
}

func withdrawProfitsWithRetry(t *testing.T, feeKey *ecdsa.PrivateKey, valAddr common.Address) {
	t.Helper()

	maxAttempts := 8
	if period, err := ctx.Proposal.WithdrawProfitPeriod(nil); err == nil && period != nil && period.Sign() > 0 {
		if attempts := int(period.Int64()) + 2; attempts > maxAttempts {
			maxAttempts = attempts
		}
		if maxAttempts > 12 {
			maxAttempts = 12
		}
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		opts, err := ctx.GetTransactor(feeKey)
		if err != nil {
			lastErr = err
			waitBlocks(t, 1)
			continue
		}
		tx, err := ctx.Validators.WithdrawProfits(opts, valAddr)
		if err == nil {
			errW := ctx.WaitMined(tx.Hash())
			if errW == nil {
				return
			}
			err = errW
		}

		lastErr = err
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "wait enough blocks") ||
			strings.Contains(msg, "epoch block forbidden") ||
			strings.Contains(msg, "timeout waiting for tx") {
			waitBlocks(t, 1)
			continue
		}
		t.Fatalf("withdraw profits failed: %v", err)
	}

	t.Fatalf("withdraw profits did not succeed within retry budget: %v", lastErr)
}

func isRetryableValidatorStakeErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "epoch block forbidden") ||
		strings.Contains(msg, "nonce") ||
		strings.Contains(msg, "timeout waiting for tx") ||
		strings.Contains(msg, "already known") ||
		strings.Contains(msg, "underpriced") ||
		strings.Contains(msg, "revert")
}

func addValidatorStakeWithRetry(t *testing.T, key *ecdsa.PrivateKey, amount *big.Int) {
	t.Helper()
	if amount == nil || amount.Sign() <= 0 {
		return
	}

	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		opts, err := ctx.GetTransactor(key)
		if err == nil {
			opts.Value = amount
			tx, errTx := ctx.Staking.AddValidatorStake(opts)
			if errTx == nil {
				if errW := ctx.WaitMined(tx.Hash()); errW == nil {
					return
				} else {
					errTx = errW
				}
			}
			err = errTx
		}
		lastErr = err
		if !isRetryableValidatorStakeErr(err) {
			utils.AssertNoError(t, err, "restore validator stake failed")
			return
		}
		waitNextBlock()
	}
	utils.AssertNoError(t, lastErr, "restore validator stake failed")
}

func decreaseValidatorStakeWithRetry(t *testing.T, key *ecdsa.PrivateKey, amount *big.Int) {
	t.Helper()
	if amount == nil || amount.Sign() <= 0 {
		return
	}

	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		opts, err := ctx.GetTransactor(key)
		if err == nil {
			opts.Value = nil
			tx, errTx := ctx.Staking.DecreaseValidatorStake(opts, amount)
			if errTx == nil {
				if errW := ctx.WaitMined(tx.Hash()); errW == nil {
					return
				} else {
					errTx = errW
				}
			}
			err = errTx
		}
		lastErr = err
		if !isRetryableValidatorStakeErr(err) {
			utils.AssertNoError(t, err, "decrease validator stake failed")
			return
		}
		waitNextBlock()
	}
	utils.AssertNoError(t, lastErr, "decrease validator stake failed")
}

func pickLowestStakeActiveGenesisValidator(t *testing.T) validatorStakeSnapshot {
	t.Helper()

	ensureGenesisValidatorBaseline(t)
	ensureMinActiveGenesisValidators(t, 3, 2)
	snapshots := activeGenesisValidatorStakes(t)

	if len(snapshots) < 3 {
		activeAll, _ := ctx.Validators.GetActiveValidators(nil)
		t.Fatalf(
			"need at least 3 active genesis validators, got %d: active=%s allGenesis=%s genesisState={%s}",
			len(snapshots),
			formatAddressList(activeAll),
			formatAddressList(genesisValidatorAddresses()),
			formatGenesisValidatorBaselineState(t),
		)
	}

	return snapshots[0]
}

func ensureGenesisValidatorBaseline(t *testing.T) {
	t.Helper()

	targetMaxValidators := big.NewInt(int64(len(ctx.GenesisValidators)))
	currentMaxValidators, err := ctx.Proposal.MaxValidators(nil)
	utils.AssertNoError(t, err, "read maxValidators for baseline failed")
	if currentMaxValidators == nil || currentMaxValidators.Cmp(targetMaxValidators) < 0 {
		utils.AssertNoError(
			t,
			ctx.EnsureConfig(rewardsConfigIDMaxValidators, targetMaxValidators, currentMaxValidators),
			"raise baseline maxValidators failed",
		)
	}

	for i, key := range ctx.GenesisValidators {
		addr := crypto.PubkeyToAddress(key.PublicKey)

		pass, err := ctx.Proposal.Pass(nil, addr)
		utils.AssertNoError(t, err, "read genesis validator pass state failed")
		if !pass {
			if err := passProposalFor(t, addr, fmt.Sprintf("V-05 Baseline Reproposal %d", i)); err != nil {
				t.Fatalf("restore genesis validator proposal for %s failed: %v", addr.Hex(), err)
			}
		}

		info, err := ctx.Staking.GetValidatorInfo(nil, addr)
		utils.AssertNoError(t, err, "read genesis validator info for baseline failed")
		if info.IsJailed {
			robustUnjailValidator(t, key, addr)
		}
	}

	ensureMinActiveGenesisValidators(t, 3, 3)
}

func waitForValidatorInactive(t *testing.T, addr common.Address, maxEpochs int) bool {
	t.Helper()
	if maxEpochs < 1 {
		maxEpochs = 1
	}

	for i := 0; i < maxEpochs; i++ {
		active, err := ctx.Validators.IsValidatorActive(nil, addr)
		if err == nil && !active {
			return true
		}
		waitForNextEpochBlock(t)
	}
	return false
}

func waitForResignWindowOpen(t *testing.T, valAddr common.Address) {
	t.Helper()

	lastActive, err := ctx.Staking.LastActiveBlock(nil, valAddr)
	utils.AssertNoError(t, err, "read lastActiveBlock before resign failed")
	if lastActive == nil || lastActive.Sign() == 0 {
		return
	}

	doubleSignWindow, err := ctx.Proposal.DoubleSignWindow(nil)
	utils.AssertNoError(t, err, "read doubleSignWindow before resign failed")

	window := uint64(0)
	if doubleSignWindow != nil && doubleSignWindow.Sign() > 0 {
		window = doubleSignWindow.Uint64()
	}

	current, err := ctx.Clients[0].BlockNumber(context.Background())
	utils.AssertNoError(t, err, "read latest block before resign failed")

	target := lastActive.Uint64() + window + 1
	if current < target {
		waitBlocks(t, int(target-current))
	}
}

func resignValidatorWithPreservedIncoming(t *testing.T, key *ecdsa.PrivateKey, valAddr common.Address) {
	t.Helper()

	active, err := ctx.Validators.IsValidatorActive(nil, valAddr)
	utils.AssertNoError(t, err, "read validator active state before resign failed")
	if active {
		t.Fatalf("validator %s must be inactive before preserved-income resign", valAddr.Hex())
	}

	waitForResignWindowOpen(t, valAddr)
	robustResignValidator(t, key, valAddr)
}

func TestI_ValidatorExtras(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	valKey := ctx.GenesisValidators[0]
	valAddr := common.HexToAddress(ctx.Config.Validators[0].Address)
	pass, _ := ctx.Proposal.Pass(nil, valAddr)
	if !pass {
		if err := passProposalFor(t, valAddr, "V-02b Auth"); err != nil {
			t.Fatalf("validator not authorized for edit tests: %v", err)
		}
	}

	// Description boundary checks (identity, website, email, details)
	t.Run("V-02b_DescriptionBoundaryFields", func(t *testing.T) {
		opts, _ := ctx.GetTransactor(valKey)

		tooLong := func(n int) string {
			b := make([]byte, n)
			for i := range b {
				b[i] = 'a'
			}
			return string(b)
		}

		// identity > 3000
		_, err := ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", tooLong(3001), "", "", "")
		if err == nil {
			t.Fatal("identity > 3000 should fail")
		}
		// website > 140
		_, err = ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", "", tooLong(141), "", "")
		if err == nil {
			t.Fatal("website > 140 should fail")
		}
		// email > 140
		_, err = ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", "", "", tooLong(141), "")
		if err == nil {
			t.Fatal("email > 140 should fail")
		}
		// details > 280
		_, err = ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", "", "", "", tooLong(281))
		if err == nil {
			t.Fatal("details > 280 should fail")
		}
	})

	t.Run("V-03_EditRegisteredValidatorAfterPassCleared", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "V-03 Pass Cleared")
		utils.AssertNoError(t, err, "failed to create validator")

		robustResignValidator(t, key, addr)
		waitValidatorProposalPassState(t, addr, false)
		exists, err := ctx.Validators.IsValidatorExist(nil, addr)
		utils.AssertNoError(t, err, "read validator existence failed")
		if !exists {
			t.Fatalf("validator %s should remain registered after pass=false", addr.Hex())
		}

		feeKey, feeAddr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "failed to create replacement fee address")
		_ = feeKey

		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "failed to get validator transactor")
		tx, err := ctx.Validators.CreateOrEditValidator(opts, feeAddr, "V-03 Updated", "identity", "site", "email", "details")
		utils.AssertNoError(t, err, "edit after pass=false should succeed")
		utils.AssertNoError(t, ctx.WaitMined(tx.Hash()), "edit after pass=false tx failed")
		waitForValidatorFeeAddr(t, addr, feeAddr)
	})

	// Withdraw profits exceptions
	t.Run("V-04_WithdrawProfitsExceptions", func(t *testing.T) {
		feeAddr, _, incoming, _, _, _ := ctx.Validators.GetValidatorInfo(nil, valAddr)

		// Non-fee address should fail
		userKey, _, _ := ctx.CreateAndFundAccount(utils.ToWei(1))
		userOpts, _ := ctx.GetTransactor(userKey)
		_, err := ctx.Validators.WithdrawProfits(userOpts, valAddr)
		if err == nil || !strings.Contains(err.Error(), "fee receiver") {
			t.Fatalf("expected non-fee withdrawal to fail, got: %v", err)
		}

		// Ensure fee address is a known key (validator address).
		opts, _ := ctx.GetTransactor(valKey)
		if tx, err := ctx.Validators.CreateOrEditValidator(opts, valAddr, "Genesis", "", "", "", ""); err == nil {
			ctx.WaitMined(tx.Hash())
		} else {
			t.Fatalf("failed to set fee address for zero-profit check: %v", err)
		}
		feeAddr, _, incoming, _, _, _ = ctx.Validators.GetValidatorInfo(nil, valAddr)
		feeKey := keyForAddress(feeAddr)
		if feeKey == nil {
			t.Fatalf("fee address key not available for zero-profit check")
		}
		if incoming.Cmp(big.NewInt(0)) > 0 {
			// Try a single withdraw to clear profits if cooldown allows.
			feeOpts, _ := ctx.GetTransactor(feeKey)
			tx, err := ctx.Validators.WithdrawProfits(feeOpts, valAddr)
			if err == nil {
				utils.AssertNoError(t, ctx.WaitMined(tx.Hash()), "first withdraw tx failed")
				firstReceipt, errR1 := ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
				utils.AssertNoError(t, errR1, "read first withdraw receipt failed")
				cooldown, errCd := ctx.Proposal.WithdrawProfitPeriod(nil)
				utils.AssertNoError(t, errCd, "read withdrawProfitPeriod failed")

				// Immediate second withdraw should fail due to cooldown or no profits.
				feeOpts, _ = ctx.GetTransactorNoEpochWait(feeKey, true)
				tx2, err := ctx.Validators.WithdrawProfits(feeOpts, valAddr)
				if err == nil {
					utils.AssertNoError(t, ctx.WaitMined(tx2.Hash()), "second withdraw tx failed")
					secondReceipt, errR2 := ctx.Clients[0].TransactionReceipt(context.Background(), tx2.Hash())
					utils.AssertNoError(t, errR2, "read second withdraw receipt failed")
					if firstReceipt != nil && secondReceipt != nil && cooldown != nil && cooldown.Sign() > 0 {
						delta := int64(secondReceipt.BlockNumber.Uint64() - firstReceipt.BlockNumber.Uint64())
						if delta < cooldown.Int64() {
							t.Fatalf("expected withdraw exception after immediate retry, got success before cooldown: delta=%d cooldown=%d", delta, cooldown.Int64())
						}
						t.Logf("Immediate retry succeeded only after cooldown elapsed: delta=%d cooldown=%d", delta, cooldown.Int64())
						return
					}
					t.Fatal("expected withdraw exception after immediate retry, got success")
				}
				if !strings.Contains(err.Error(), "You don't have any profits") && !strings.Contains(err.Error(), "wait enough blocks") {
					t.Fatalf("unexpected withdraw error: %v", err)
				}
				return
			}
			if strings.Contains(err.Error(), "wait enough blocks") {
				// Cooldown not satisfied; acceptable exception path.
				return
			}
			t.Fatalf("cannot withdraw to clear profits: %v", err)
		}

		// No incoming profits; expect an exception (cooldown or zero profits).
		feeOpts, _ := ctx.GetTransactor(feeKey)
		_, err = ctx.Validators.WithdrawProfits(feeOpts, valAddr)
		if err == nil {
			t.Fatal("expected withdraw exception, got success")
		}
		if !strings.Contains(err.Error(), "You don't have any profits") && !strings.Contains(err.Error(), "wait enough blocks") {
			t.Fatalf("unexpected withdraw error: %v", err)
		}
	})

	t.Run("V-05_ResignedValidatorCanRotateFeeAddrAndWithdraw", func(t *testing.T) {
		validator := pickLowestStakeActiveGenesisValidator(t)
		valKey, valAddr := validator.key, validator.addr

		originalFeeAddr, _, _, _, _, err := ctx.Validators.GetValidatorInfo(nil, valAddr)
		utils.AssertNoError(t, err, "read validator info before resign failed")
		moniker, identity, website, email, details, err := ctx.Validators.GetValidatorDescription(nil, valAddr)
		utils.AssertNoError(t, err, "read validator description before resign failed")
		originalMaxValidators, err := ctx.Proposal.MaxValidators(nil)
		utils.AssertNoError(t, err, "read maxValidators failed")
		originalDoubleSignWindow, err := ctx.Proposal.DoubleSignWindow(nil)
		utils.AssertNoError(t, err, "read doubleSignWindow failed")
		targetDoubleSignWindow := big.NewInt(8)

		stakeReduction := big.NewInt(0)
		if validator.selfStake != nil && validator.totalStake != nil {
			snapshots := activeGenesisValidatorStakes(t)
			if len(snapshots) < 2 {
				t.Fatalf("need at least 2 active validators to rank for V-05")
			}
			secondLowest := snapshots[1]

			if validator.totalStake.Cmp(secondLowest.totalStake) >= 0 {
				stakeReduction = new(big.Int).Sub(validator.totalStake, secondLowest.totalStake)
				stakeReduction.Add(stakeReduction, big.NewInt(1))

				minStake := testkit.RequireMinValidatorStake(t, func() (*big.Int, error) {
					return ctx.Proposal.MinValidatorStake(nil)
				})
				remaining := new(big.Int).Sub(validator.selfStake, stakeReduction)
				if remaining.Cmp(minStake) < 0 {
					t.Fatalf(
						"cannot reduce validator %s enough to fall out of top-2: selfStake=%s totalStake=%s secondLowest=%s minStake=%s needReduction=%s",
						valAddr.Hex(),
						validator.selfStake.String(),
						validator.totalStake.String(),
						secondLowest.totalStake.String(),
						minStake.String(),
						stakeReduction.String(),
					)
				}
			}
		}

		feeKey, feeAddr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "failed to create rotated fee address")

		t.Cleanup(func() {
			if stakeReduction.Sign() > 0 {
				addValidatorStakeWithRetry(t, valKey, stakeReduction)
			}

			if originalMaxValidators != nil && originalMaxValidators.Sign() > 0 {
				currentMaxValidators, err := ctx.Proposal.MaxValidators(nil)
				if err == nil && currentMaxValidators.Cmp(originalMaxValidators) != 0 {
					if err := ctx.EnsureConfig(rewardsConfigIDMaxValidators, originalMaxValidators, currentMaxValidators); err != nil {
						t.Logf("cleanup: restore maxValidators failed: %v", err)
					}
				}
			}
			if originalDoubleSignWindow != nil && originalDoubleSignWindow.Sign() > 0 {
				currentDoubleSignWindow, err := ctx.Proposal.DoubleSignWindow(nil)
				if err == nil && currentDoubleSignWindow.Cmp(originalDoubleSignWindow) != 0 {
					if err := ctx.EnsureConfig(rewardsConfigIDDoubleSignWindow, originalDoubleSignWindow, currentDoubleSignWindow); err != nil {
						t.Logf("cleanup: restore doubleSignWindow failed: %v", err)
					}
				}
			}

			currentFeeAddr, _, _, _, _, infoErr := ctx.Validators.GetValidatorInfo(nil, valAddr)
			if infoErr == nil && currentFeeAddr != originalFeeAddr {
				if opts, err := ctx.GetTransactor(valKey); err == nil {
					if tx, err := ctx.Validators.CreateOrEditValidator(opts, originalFeeAddr, moniker, identity, website, email, details); err == nil {
						_ = ctx.WaitMined(tx.Hash())
					} else {
						t.Logf("cleanup: restore fee address failed: %v", err)
					}
				} else {
					t.Logf("cleanup: get transactor for fee restore failed: %v", err)
				}
			}

			pass, passErr := ctx.Proposal.Pass(nil, valAddr)
			if passErr == nil && !pass {
				if err := passProposalFor(t, valAddr, "V-05 Cleanup Reproposal"); err != nil {
					t.Logf("cleanup: reproposal failed: %v", err)
				}
			}

			info, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
			if err == nil && info.IsJailed {
				if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
					if current, err := ctx.Clients[0].BlockNumber(context.Background()); err == nil {
						targetHeight := info.JailUntilBlock.Uint64() + 1
						if targetHeight > current {
							waitBlocks(t, int(targetHeight-current))
						}
					}
				}
				for attempt := 0; attempt < 5; attempt++ {
					opts, err := ctx.GetTransactor(valKey)
					if err != nil {
						waitBlocks(t, 1)
						continue
					}
					tx, err := ctx.Staking.UnjailValidator(opts, valAddr)
					if err == nil {
						if err := ctx.WaitMined(tx.Hash()); err == nil {
							break
						}
					}
					waitBlocks(t, 1)
				}
			}
		})

		incomingBefore := waitForIncomingProfits(t, valAddr, 24)
		if incomingBefore.Sign() <= 0 {
			t.Fatalf("expected incoming profits before withdraw, got %s", incomingBefore.String())
		}
		if originalDoubleSignWindow == nil || originalDoubleSignWindow.Sign() <= 0 {
			t.Fatalf("invalid doubleSignWindow before V-05: %v", originalDoubleSignWindow)
		}
		if originalDoubleSignWindow.Cmp(targetDoubleSignWindow) > 0 {
			utils.AssertNoError(
				t,
				ctx.EnsureConfig(rewardsConfigIDDoubleSignWindow, targetDoubleSignWindow, originalDoubleSignWindow),
				"shrink doubleSignWindow failed",
			)
		}

		if stakeReduction.Sign() > 0 {
			decreaseValidatorStakeWithRetry(t, valKey, stakeReduction)
		}
		if originalMaxValidators == nil || originalMaxValidators.Sign() <= 0 {
			t.Fatalf("invalid maxValidators before V-05: %v", originalMaxValidators)
		}
		if originalMaxValidators.Cmp(big.NewInt(2)) > 0 {
			utils.AssertNoError(t, ctx.EnsureConfig(rewardsConfigIDMaxValidators, big.NewInt(2), originalMaxValidators), "shrink maxValidators failed")
		}
		if !waitForValidatorInactive(t, valAddr, 2) {
			t.Fatalf("validator %s did not leave active set after maxValidators reduction", valAddr.Hex())
		}

		resignValidatorWithPreservedIncoming(t, valKey, valAddr)
		waitValidatorProposalPassState(t, valAddr, false)

		opts, err := ctx.GetTransactor(valKey)
		utils.AssertNoError(t, err, "failed to get validator transactor for fee rotation")
		tx, err := ctx.Validators.CreateOrEditValidator(opts, feeAddr, moniker, identity, website, email, details)
		utils.AssertNoError(t, err, "fee rotation after resign/pass=false should succeed")
		utils.AssertNoError(t, ctx.WaitMined(tx.Hash()), "fee rotation tx failed")
		waitForValidatorFeeAddr(t, valAddr, feeAddr)

		withdrawProfitsWithRetry(t, feeKey, valAddr)

		feeAddrAfter, _, incomingAfter, _, lastWithdrawBlock, err := ctx.Validators.GetValidatorInfo(nil, valAddr)
		utils.AssertNoError(t, err, "read validator info after rotated withdraw failed")
		if feeAddrAfter != feeAddr {
			t.Fatalf("rotated fee address mismatch after withdraw: got=%s want=%s", feeAddrAfter.Hex(), feeAddr.Hex())
		}
		if lastWithdrawBlock == nil || lastWithdrawBlock.Sign() <= 0 {
			t.Fatalf("expected lastWithdrawBlock to be updated after rotated withdraw, got %v", lastWithdrawBlock)
		}
		if incomingAfter != nil && incomingAfter.Cmp(incomingBefore) > 0 {
			t.Logf("incoming profits increased during withdraw flow: before=%s after=%s", incomingBefore.String(), incomingAfter.String())
		}

		opts, err = ctx.GetTransactor(valKey)
		utils.AssertNoError(t, err, "failed to get validator transactor for fee restore")
		restoreTx, err := ctx.Validators.CreateOrEditValidator(opts, originalFeeAddr, moniker, identity, website, email, details)
		utils.AssertNoError(t, err, "restore original fee address failed")
		utils.AssertNoError(t, ctx.WaitMined(restoreTx.Hash()), "restore original fee address tx failed")
		waitForValidatorFeeAddr(t, valAddr, originalFeeAddr)

		if originalMaxValidators.Cmp(big.NewInt(2)) != 0 {
			utils.AssertNoError(t, ctx.EnsureConfig(rewardsConfigIDMaxValidators, originalMaxValidators, big.NewInt(2)), "restore maxValidators failed")
		}
		if stakeReduction.Sign() > 0 {
			addValidatorStakeWithRetry(t, valKey, stakeReduction)
			stakeReduction = big.NewInt(0)
		}

		if err := passProposalFor(t, valAddr, "V-05 Reproposal"); err != nil {
			t.Fatalf("reproposal after resign/withdraw failed: %v", err)
		}
		robustUnjailValidator(t, valKey, valAddr)
		if !waitForValidatorActive(t, valAddr, 2) {
			t.Fatalf("validator %s did not return to active set after withdraw flow", valAddr.Hex())
		}
	})
}
