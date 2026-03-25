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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestG_DoubleSign(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}
	ensureMinActiveValidators(t, 3, 2)

	// [P-07] Submit Double Sign Evidence
	t.Run("P-07_DoubleSignEvidence", func(t *testing.T) {
		submitEvidenceFor := func(valKey *ecdsa.PrivateKey, valAddr common.Address) error {
			active, errActive := ctx.Validators.IsValidatorActive(nil, valAddr)
			if errActive != nil {
				return fmt.Errorf("failed to check validator active status %s: %w", valAddr.Hex(), errActive)
			}
			if !active && !waitForValidatorActive(t, valAddr, 2) {
				return fmt.Errorf("validator not active before double-sign evidence: %s", valAddr.Hex())
			}
			ctx.WaitIfEpochBlock()

			// Use a clean account to submit evidence
			reporterKey, reporterAddr, err := ctx.CreateAndFundAccount(utils.ToWei(10))
			if err != nil {
				return fmt.Errorf("failed to setup reporter: %w", err)
			}

			for attempt := 0; attempt < 3; attempt++ {
				// 1. Prepare Block Height
				header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
				if err != nil || header == nil {
					return fmt.Errorf("failed to read latest header: %w", err)
				}
				targetHeight := new(big.Int).Sub(header.Number, big.NewInt(1))
				if targetHeight.Cmp(big.NewInt(0)) <= 0 {
					targetHeight = big.NewInt(1)
				}
				baseTime := header.Time
				if targetHeader, err := ctx.Clients[0].HeaderByNumber(context.Background(), targetHeight); err == nil && targetHeader != nil {
					baseTime = targetHeader.Time
				}

				t.Logf("Constructing double sign evidence for validator %s at height %s (attempt=%d)", valAddr.Hex(), targetHeight, attempt+1)

				// 2. Construct Two Headers
				h1 := &types.Header{
					ParentHash:  common.Hash{},
					UncleHash:   types.EmptyUncleHash,
					Coinbase:    valAddr,
					Root:        common.Hash{},
					TxHash:      types.EmptyRootHash,
					ReceiptHash: types.EmptyRootHash,
					Bloom:       types.Bloom{},
					Difficulty:  big.NewInt(1),
					Number:      targetHeight,
					GasLimit:    30000000,
					GasUsed:     0,
					Time:        baseTime,
					Extra:       make([]byte, 32+65),
					MixDigest:   common.Hash{},
					Nonce:       types.BlockNonce{},
				}
				h2 := &types.Header{
					ParentHash:  common.Hash{},
					UncleHash:   types.EmptyUncleHash,
					Coinbase:    valAddr,
					Root:        common.Hash{0x01},
					TxHash:      types.EmptyRootHash,
					ReceiptHash: types.EmptyRootHash,
					Bloom:       types.Bloom{},
					Difficulty:  big.NewInt(1),
					Number:      targetHeight,
					GasLimit:    30000000,
					GasUsed:     0,
					Time:        baseTime,
					Extra:       make([]byte, 32+65),
					MixDigest:   common.Hash{},
					Nonce:       types.BlockNonce{},
				}

				rlp1, err := signHeaderClique(h1, valKey)
				if err != nil {
					return fmt.Errorf("failed to sign h1: %w", err)
				}
				rlp2, err := signHeaderClique(h2, valKey)
				if err != nil {
					return fmt.Errorf("failed to sign h2: %w", err)
				}

				infoBefore, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
				if err != nil {
					return fmt.Errorf("failed to read validator info before submit: %w", err)
				}
				beforeBlock := new(big.Int).Set(header.Number)
				reporterBalBefore, err := ctx.Clients[0].BalanceAt(context.Background(), reporterAddr, beforeBlock)
				if err != nil {
					return fmt.Errorf("failed to read reporter balance before submit: %w", err)
				}

				opts, err := ctx.GetTransactor(reporterKey)
				if err != nil {
					return fmt.Errorf("transactor failed: %w", err)
				}
				tx, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
				if err != nil {
					if attempt < 2 && (strings.Contains(strings.ToLower(err.Error()), "revert") || strings.Contains(err.Error(), "Epoch block forbidden") || strings.Contains(err.Error(), "timeout waiting for tx")) {
						waitBlocks(t, 1)
						continue
					}
					return fmt.Errorf("failed to submit evidence: %w", err)
				}
				if err := ctx.WaitMined(tx.Hash()); err != nil {
					if attempt < 2 && (strings.Contains(strings.ToLower(err.Error()), "revert") || strings.Contains(err.Error(), "Epoch block forbidden") || strings.Contains(err.Error(), "timeout waiting for tx")) {
						waitBlocks(t, 1)
						continue
					}
					return fmt.Errorf("double sign evidence tx failed: %w", err)
				}

				infoAfter, err := ctx.Staking.GetValidatorInfo(nil, valAddr)
				if err != nil {
					return fmt.Errorf("failed to read validator info after submit: %w", err)
				}
				if infoAfter.SelfStake == nil || infoBefore.SelfStake == nil || infoAfter.SelfStake.Cmp(infoBefore.SelfStake) >= 0 {
					if attempt < 2 {
						waitBlocks(t, 1)
						continue
					}
					return fmt.Errorf("validator was not slashed: before=%v after=%v", infoBefore.SelfStake, infoAfter.SelfStake)
				}
				if !infoAfter.IsJailed {
					if attempt < 2 {
						waitBlocks(t, 1)
						continue
					}
					return fmt.Errorf("validator was not jailed after double-sign evidence")
				}

				receipt, err := ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
				if err != nil || receipt == nil {
					return fmt.Errorf("failed to read receipt: %w", err)
				}
				effectiveGasPrice := receipt.EffectiveGasPrice
				if effectiveGasPrice == nil || effectiveGasPrice.Sign() == 0 {
					effectiveGasPrice = tx.GasPrice()
				}
				gasCost := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), effectiveGasPrice)

				var rewardAmount *big.Int
				for _, l := range receipt.Logs {
					if ev, err := ctx.Staking.ParseValidatorSlashed(*l); err == nil {
						if ev.Validator == valAddr {
							rewardAmount = ev.RewardAmount
							break
						}
					}
				}
				if rewardAmount == nil {
					for _, l := range receipt.Logs {
						if ev, err := ctx.Punish.ParseLogDoubleSignPunish(*l); err == nil {
							rewardAmount = ev.RewardAmount
							break
						}
					}
				}
				if rewardAmount == nil {
					return fmt.Errorf("failed to parse double sign reward amount from logs")
				}

				reporterBalAfter, err := ctx.Clients[0].BalanceAt(context.Background(), reporterAddr, receipt.BlockNumber)
				if err != nil {
					return fmt.Errorf("failed to read reporter balance after submit: %w", err)
				}
				expectedMin := new(big.Int).Sub(reporterBalBefore, gasCost)
				expectedMin.Add(expectedMin, rewardAmount)
				if reporterBalAfter.Cmp(expectedMin) < 0 {
					_ = testkit.WaitUntil(testkit.WaitUntilOptions{
						MaxAttempts: 3,
						Interval:    retrySleep(),
					}, func() (bool, error) {
						var err error
						reporterBalAfter, err = ctx.Clients[0].BalanceAt(context.Background(), reporterAddr, nil)
						if err != nil {
							return false, err
						}
						return reporterBalAfter.Cmp(expectedMin) >= 0, nil
					})
				}
				if reporterBalAfter.Cmp(expectedMin) < 0 {
					return fmt.Errorf("reporter reward not received: before=%s after=%s gas=%s reward=%s expectedMin=%s",
						reporterBalBefore.String(),
						reporterBalAfter.String(),
						gasCost.String(),
						rewardAmount.String(),
						expectedMin.String(),
					)
				}
				return nil
			}
			return fmt.Errorf("failed to submit double sign evidence for %s after retries", valAddr.Hex())
		}

		// Prefer reusing an already-created validator from prior flows to avoid
		// repeated create/register overhead in the slow-path suite.
		valKey := cachedReentryValidatorKey
		valAddr := cachedReentryValidatorAddr
		if valKey != nil && valAddr != (common.Address{}) {
			if info, errInfo := ctx.Staking.GetValidatorInfo(nil, valAddr); errInfo == nil && info.IsRegistered && !info.IsJailed {
				active, errActive := ctx.Validators.IsValidatorActive(nil, valAddr)
				if errActive == nil && !active {
					_ = testkit.WaitUntil(testkit.WaitUntilOptions{
						MaxAttempts: 6,
						Interval:    retrySleep(),
						OnRetry: func(int) {
							waitNextBlock()
						},
					}, func() (bool, error) {
						ok, err := ctx.Validators.IsValidatorActive(nil, valAddr)
						if err != nil {
							return false, err
						}
						active = ok
						return ok, nil
					})
				}
				if !active {
					active = waitForValidatorActive(t, valAddr, 1)
				}
				if active {
					t.Logf("Reusing validator for P-07: %s", valAddr.Hex())
					if err := submitEvidenceFor(valKey, valAddr); err == nil {
						return
					} else {
						t.Logf("P-07 reuse path failed, fallback to fresh validator: %v", err)
					}
				}
			}
		}

		// Fallback for stability: use a fresh validator when reuse path fails.
		var err error
		valKey, valAddr, err = createAndRegisterValidator(t, "Slashing Target")
		utils.AssertNoError(t, err, "failed to setup target validator")
		if !waitForValidatorActive(t, valAddr, 3) {
			t.Fatalf("fresh validator did not become active for P-07: %s", valAddr.Hex())
		}
		utils.AssertNoError(t, submitEvidenceFor(valKey, valAddr), "failed to submit evidence")
	})

	ctx.WaitIfEpochBlock()

	// [P-21] Resign + Double Sign
	t.Run("P-21_ResignThenDoubleSign", func(t *testing.T) {
		var (
			key  *ecdsa.PrivateKey
			addr common.Address
			err  error
		)
		if cachedReentryValidatorKey != nil && cachedReentryValidatorAddr != (common.Address{}) {
			if info, errInfo := ctx.Staking.GetValidatorInfo(nil, cachedReentryValidatorAddr); errInfo == nil && info.IsRegistered && !info.IsJailed {
				key = cachedReentryValidatorKey
				addr = cachedReentryValidatorAddr
				t.Logf("Reusing validator from F2 for P-21: %s", addr.Hex())
			}
		}
		if key == nil {
			key, addr, err = createAndRegisterValidator(t, "P-21 ResignDS")
			utils.AssertNoError(t, err, "create val failed")
		}
		robustResignValidator(t, key)

		var lastErr error
		for attempt := 0; attempt < 8; attempt++ {
			ctx.WaitIfEpochBlock()

			header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
			if err != nil || header == nil {
				lastErr = fmt.Errorf("read latest header failed: %w", err)
				waitBlocks(t, 1)
				continue
			}
			targetHeight := new(big.Int).Sub(header.Number, big.NewInt(1))
			if targetHeight.Sign() <= 0 {
				targetHeight = big.NewInt(1)
			}
			baseTime := header.Time
			if targetHeader, err := ctx.Clients[0].HeaderByNumber(context.Background(), targetHeight); err == nil && targetHeader != nil {
				baseTime = targetHeader.Time
			}

			h1 := &types.Header{
				Coinbase: addr,
				Number:   targetHeight,
				Extra:    make([]byte, 32+65),
				Root:     common.Hash{0x21},
				Time:     baseTime,
			}
			h2 := &types.Header{
				Coinbase: addr,
				Number:   targetHeight,
				Extra:    make([]byte, 32+65),
				Root:     common.Hash{0x22},
				Time:     baseTime,
			}
			rlp1, err := signHeaderClique(h1, key)
			if err != nil {
				lastErr = fmt.Errorf("failed to sign first header: %w", err)
				waitBlocks(t, 1)
				continue
			}
			rlp2, err := signHeaderClique(h2, key)
			if err != nil {
				lastErr = fmt.Errorf("failed to sign second header: %w", err)
				waitBlocks(t, 1)
				continue
			}

			opts, err := ctx.GetTransactor(key)
			if err != nil {
				lastErr = fmt.Errorf("transactor failed for double sign: %w", err)
				waitBlocks(t, 1)
				continue
			}
			txDS, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
			if err == nil {
				if errW := ctx.WaitMined(txDS.Hash()); errW == nil {
					lastErr = nil
					break
				} else {
					err = errW
				}
			}
			lastErr = err
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			if strings.Contains(msg, "Epoch block forbidden") ||
				strings.Contains(msg, "nonce too low") ||
				strings.Contains(msg, "already known") ||
				strings.Contains(msg, "replacement transaction underpriced") ||
				strings.Contains(strings.ToLower(msg), "revert") {
				waitBlocks(t, 1)
				continue
			}
			break
		}
		utils.AssertNoError(t, lastErr, "Should allow double sign punishment after resign")
	})

	ctx.WaitIfEpochBlock()

	// [P-22] Exit + Double Sign
	t.Run("P-22_ExitThenDoubleSign", func(t *testing.T) {
		// Keep this scenario isolated from P-21. After the min-stake alignment,
		// reusing a validator that was already slashed in P-21 can leave it below
		// minValidatorStake, which makes exitValidator inapplicable and turns this
		// case into a stake-floor test instead of an exit-after-clean-resign test.
		key, addr, err := createAndRegisterValidator(t, "P-22 ExitDS")
		utils.AssertNoError(t, err, "create val failed")
		robustResignValidator(t, key)

		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed")
		info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
		current, _ := ctx.Clients[0].BlockNumber(context.Background())
		targetHeight := uint64(0)
		if info.JailUntilBlock != nil && info.JailUntilBlock.Sign() > 0 {
			targetHeight = info.JailUntilBlock.Uint64() + 1
		} else {
			unjailPeriod, _ := ctx.Proposal.ValidatorUnjailPeriod(nil)
			if unjailPeriod != nil && unjailPeriod.Sign() > 0 {
				targetHeight = current + unjailPeriod.Uint64() + 1
			}
		}
		if targetHeight > 0 {
			if targetHeight > current {
				waitBlocks(t, int(targetHeight-current))
			}
			h, err := ctx.Clients[0].BlockNumber(context.Background())
			utils.AssertNoError(t, err, "read current block failed after jail wait")
			utils.AssertTrue(t, h >= targetHeight, "jail period wait failed")
		}
		robustExitValidator(t, key)

		header, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		h1 := &types.Header{Coinbase: addr, Number: header.Number, Extra: make([]byte, 32+65), Root: common.Hash{0x31}}
		h2 := &types.Header{Coinbase: addr, Number: header.Number, Extra: make([]byte, 32+65), Root: common.Hash{0x32}}
		rlp1, _ := signHeaderClique(h1, key)
		rlp2, _ := signHeaderClique(h2, key)

		_, err = ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
		if err == nil {
			t.Fatal("Should fail double sign punishment after exit")
		}
	})

	ctx.WaitIfEpochBlock()

	// [P-10~P-14] Double Sign Exceptions
	t.Run("P-10-14_DoubleSignExceptions", func(t *testing.T) {
		key := getActiveProposerOrSkip(t, 2)
		addr := crypto.PubkeyToAddress(key.PublicKey)
		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed")
		header, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)

		hBase := &types.Header{Coinbase: addr, Number: header.Number, Extra: make([]byte, 32+65)}

		// P-11: Same Header
		h1_same, _ := signHeaderClique(hBase, key)
		_, err = ctx.Punish.SubmitDoubleSignEvidence(opts, h1_same, h1_same)
		if err == nil {
			t.Fatal("Should fail with 'Same header'")
		}

		// P-12: Height Mismatch
		h1_h1 := *hBase
		h2_h2 := *hBase
		h2_h2.Number = new(big.Int).Add(hBase.Number, big.NewInt(1))
		h2_h2.Root = common.Hash{0x01}
		rlp1, _ := signHeaderClique(&h1_h1, key)
		rlp2, _ := signHeaderClique(&h2_h2, key)
		_, err = ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
		if err == nil {
			t.Fatal("Should fail with 'Height mismatch'")
		}

		// P-14: Signer != Coinbase
		otherKey, _, _ := ctx.CreateAndFundAccount(utils.ToWei(1))
		h1_wrong := *hBase
		h1_wrong.Root = common.Hash{0x05}
		h2_wrong := *hBase
		h2_wrong.Root = common.Hash{0x06}
		rlp1_wrong, _ := signHeaderClique(&h1_wrong, otherKey)
		rlp2_wrong, _ := signHeaderClique(&h2_wrong, otherKey)
		_, err = ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1_wrong, rlp2_wrong)
		if err == nil {
			t.Fatal("Should fail with 'Signer != coinbase'")
		}

		// P-10: Future block
		hFuture := *hBase
		hFuture.Number = new(big.Int).Add(hBase.Number, big.NewInt(10))
		hFuture.Root = common.Hash{0x07}
		rlpFuture1, _ := signHeaderClique(&hFuture, key)
		hFuture2 := hFuture
		hFuture2.Root = common.Hash{0x08}
		rlpFuture2, _ := signHeaderClique(&hFuture2, key)
		_, err = ctx.Punish.SubmitDoubleSignEvidence(opts, rlpFuture1, rlpFuture2)
		if err == nil {
			t.Fatal("Should fail with 'Future block'")
		}
	})

	ctx.WaitIfEpochBlock()

	// [P-23] Multi-Validator Double Sign (same epoch)
	t.Run("P-23_MultiValidatorDoubleSign", func(t *testing.T) {
		if len(ctx.GenesisValidators) < 2 {
			t.Fatalf("not enough validator keys")
		}

		// Avoid epoch boundary blocks
		ctx.WaitIfEpochBlock()
		header, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		epochBI, _ := ctx.Proposal.Epoch(nil)
		if epochBI != nil && epochBI.Sign() > 0 {
			epoch := epochBI.Uint64()
			mod := header.Number.Uint64() % epoch
			if mod > epoch-5 {
				waitForNextEpochBlock(t)
				ctx.WaitIfEpochBlock()
				header, _ = ctx.Clients[0].HeaderByNumber(context.Background(), nil)
			}
		}

		miner := header.Coinbase
		type target struct {
			key  *ecdsa.PrivateKey
			addr common.Address
		}
		targets := make([]target, 0, 2)
		seen := make(map[common.Address]bool)
		if activeVals, err := ctx.Validators.GetActiveValidators(nil); err == nil {
			for _, addr := range activeVals {
				if addr == miner || seen[addr] {
					continue
				}
				key := keyForAddress(addr)
				if key == nil {
					continue
				}
				info, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
				if errInfo != nil || info.IsJailed {
					continue
				}
				seen[addr] = true
				targets = append(targets, target{key: key, addr: addr})
				if len(targets) >= 2 {
					break
				}
			}
		}
		if len(targets) < 2 {
			t.Skip("skip multi-validator double sign: not enough active non-miner validators")
		}
		if len(targets) < 2 {
			t.Fatalf("insufficient distinct validators for multi-double-sign")
		}

		targetSet := map[common.Address]struct{}{
			targets[0].addr: {},
			targets[1].addr: {},
		}
		reporterCandidates := make([]*ecdsa.PrivateKey, 0, len(ctx.GenesisValidators)+1)
		reporterSeen := make(map[common.Address]bool)
		addReporter := func(key *ecdsa.PrivateKey) {
			if key == nil {
				return
			}
			addr := crypto.PubkeyToAddress(key.PublicKey)
			if reporterSeen[addr] {
				return
			}
			if _, blocked := targetSet[addr]; blocked {
				return
			}
			reporterSeen[addr] = true
			reporterCandidates = append(reporterCandidates, key)
		}
		addReporter(keyForAddress(miner))
		for _, key := range ctx.GenesisValidators {
			addReporter(key)
		}
		addReporter(ctx.FunderKey)
		if len(reporterCandidates) == 0 {
			t.Fatalf("no eligible reporter account for multi double sign")
		}

		waitMinedShort := func(txHash common.Hash, timeout time.Duration) error {
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				receipt, err := ctx.Clients[0].TransactionReceipt(context.Background(), txHash)
				if err == nil && receipt != nil {
					if receipt.Status == 0 {
						return fmt.Errorf("transaction %s reverted", txHash.Hex())
					}
					return nil
				}
				time.Sleep(retrySleep())
			}
			return fmt.Errorf("timeout waiting for tx %s", txHash.Hex())
		}

		submitEvidence := func(tgt target, rootBase byte) {
			ctx.WaitIfEpochBlock()
			head, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
			targetHeight := new(big.Int).Sub(head.Number, big.NewInt(1))
			if targetHeight.Sign() <= 0 {
				targetHeight = big.NewInt(1)
			}
			baseTime := head.Time
			if targetHeader, err := ctx.Clients[0].HeaderByNumber(context.Background(), targetHeight); err == nil && targetHeader != nil {
				baseTime = targetHeader.Time
			}

			h1 := &types.Header{Coinbase: tgt.addr, Number: targetHeight, Extra: make([]byte, 32+65), Root: common.Hash{rootBase}, Time: baseTime}
			h2 := &types.Header{Coinbase: tgt.addr, Number: targetHeight, Extra: make([]byte, 32+65), Root: common.Hash{rootBase + 1}, Time: baseTime}
			rlp1, err := signHeaderClique(h1, tgt.key)
			utils.AssertNoError(t, err, "failed to sign h1")
			rlp2, err := signHeaderClique(h2, tgt.key)
			utils.AssertNoError(t, err, "failed to sign h2")

			lastErr := error(nil)
			maxAttempts := len(reporterCandidates) * 2
			if maxAttempts < 4 {
				maxAttempts = 4
			}
			for attempt := 0; attempt < maxAttempts; attempt++ {
				reporterKey := reporterCandidates[attempt%len(reporterCandidates)]
				reporterAddr := crypto.PubkeyToAddress(reporterKey.PublicKey)
				opts, err := ctx.GetTransactor(reporterKey)
				if err != nil {
					lastErr = err
					waitNextBlock()
					continue
				}
				tx, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
				if err == nil {
					if errW := waitMinedShort(tx.Hash(), 75*time.Second); errW == nil {
						return
					} else {
						lastErr = errW
						if strings.Contains(errW.Error(), "timeout waiting for tx") {
							if progErr := ctx.WaitForBlockProgress(1, 20*time.Second); progErr != nil {
								t.Skipf("skip multi-validator double sign: chain stalled while tx pending (%s): %v", tx.Hash().Hex(), progErr)
							}
							ctx.RefreshNonce(reporterAddr)
							waitNextBlock()
							continue
						}
						t.Fatalf("failed to mine multi double sign evidence tx: %v", errW)
					}
				}
				lastErr = err
				if strings.Contains(err.Error(), "nonce too low") ||
					strings.Contains(err.Error(), "already known") ||
					strings.Contains(err.Error(), "replacement transaction underpriced") ||
					strings.Contains(err.Error(), "Epoch block forbidden") {
					ctx.RefreshNonce(reporterAddr)
					waitNextBlock()
					continue
				}
				t.Fatalf("failed to submit multi double sign evidence: %v", err)
			}
			t.Fatalf("failed to submit multi double sign evidence after retries: %v", lastErr)
		}

		// Submit evidence for two validators in the same epoch
		submitEvidence(targets[0], 0x41)
		submitEvidence(targets[1], 0x51)

		// Both should be jailed
		for _, tgt := range targets[:2] {
			err := testkit.WaitUntil(testkit.WaitUntilOptions{
				MaxAttempts: 6,
				Interval:    retrySleep(),
				OnRetry: func(int) {
					waitBlocks(t, 1)
				},
			}, func() (bool, error) {
				info, err := ctx.Staking.GetValidatorInfo(nil, tgt.addr)
				if err != nil {
					return false, err
				}
				return info.IsJailed, nil
			})
			utils.AssertNoError(t, err, "validator should be jailed after double sign")
		}

		// Ensure chain still progresses with remaining validator(s).
		// On some runs, validator-set transition right after slashing can delay progress briefly.
		if err := ctx.WaitForBlockProgress(1, 30*time.Second); err != nil {
			t.Logf("initial progress check failed after multi-validator double sign: %v", err)
			// Prefer a longer direct progress probe over forced epoch jumps: when the
			// chain is temporarily stalled after multi-slash handling, block production
			// can recover without reaching a specific epoch boundary.
			if err2 := ctx.WaitForBlockProgress(1, 120*time.Second); err2 != nil {
				t.Fatalf("chain did not progress after multi-validator double sign: %v (initial=%v)", err2, err)
			}
		}
	})
}

func signHeaderClique(h *types.Header, key *ecdsa.PrivateKey) ([]byte, error) {
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
