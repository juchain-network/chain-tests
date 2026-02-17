package tests

import (
	"context"
	"crypto/ecdsa"
	"math/big"
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
		// Use a FRESH validator instead of Genesis 0 to keep the network healthy
		valKey, valAddr, err := createAndRegisterValidator(t, "Slashing Target")
		utils.AssertNoError(t, err, "failed to setup target validator")

		// Use a clean account to submit evidence
		reporterKey, reporterAddr, err := ctx.CreateAndFundAccount(utils.ToWei(10))
		utils.AssertNoError(t, err, "failed to setup reporter")

		// 1. Prepare Block Height
		header, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		targetHeight := new(big.Int).Sub(header.Number, big.NewInt(1))
		if targetHeight.Cmp(big.NewInt(0)) <= 0 {
			targetHeight = big.NewInt(1)
		}
		baseTime := header.Time
		if targetHeader, err := ctx.Clients[0].HeaderByNumber(context.Background(), targetHeight); err == nil && targetHeader != nil {
			baseTime = targetHeader.Time
		}

		t.Logf("Constructing double sign evidence for validator %s at height %s", valAddr.Hex(), targetHeight)

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
		utils.AssertNoError(t, err, "failed to sign h1")

		rlp2, err := signHeaderClique(h2, valKey)
		utils.AssertNoError(t, err, "failed to sign h2")

		infoBefore, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		beforeBlock := new(big.Int).Set(header.Number)
		reporterBalBefore, _ := ctx.Clients[0].BalanceAt(context.Background(), reporterAddr, beforeBlock)

		opts, err := ctx.GetTransactor(reporterKey)
		utils.AssertNoError(t, err, "transactor failed")
		tx, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
		utils.AssertNoError(t, err, "failed to submit evidence")
		ctx.WaitMined(tx.Hash())

		infoAfter, _ := ctx.Staking.GetValidatorInfo(nil, valAddr)
		utils.AssertTrue(t, infoAfter.SelfStake.Cmp(infoBefore.SelfStake) < 0, "Validator should be slashed")
		utils.AssertTrue(t, infoAfter.IsJailed, "Validator should be jailed")

		receipt, err := ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
		utils.AssertNoError(t, err, "failed to read receipt")
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
			t.Fatal("failed to parse double sign reward amount from logs")
		}

		reporterBalAfter, _ := ctx.Clients[0].BalanceAt(context.Background(), reporterAddr, receipt.BlockNumber)
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
			t.Fatalf("reporter reward not received: before=%s after=%s gas=%s reward=%s expectedMin=%s",
				reporterBalBefore.String(),
				reporterBalAfter.String(),
				gasCost.String(),
				rewardAmount.String(),
				expectedMin.String(),
			)
		}
	})

	ctx.WaitIfEpochBlock()

	// [P-21] Resign + Double Sign
	t.Run("P-21_ResignThenDoubleSign", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "P-21 ResignDS")
		utils.AssertNoError(t, err, "create val failed")
		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed")

		txR, err := ctx.Staking.ResignValidator(opts)
		utils.AssertNoError(t, err, "resign failed")
		ctx.WaitMined(txR.Hash())

		header, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		targetHeight := new(big.Int).Sub(header.Number, big.NewInt(1))

		h1 := &types.Header{Coinbase: addr, Number: targetHeight, Extra: make([]byte, 32+65), Root: common.Hash{0x21}}
		h2 := &types.Header{Coinbase: addr, Number: targetHeight, Extra: make([]byte, 32+65), Root: common.Hash{0x22}}
		rlp1, _ := signHeaderClique(h1, key)
		rlp2, _ := signHeaderClique(h2, key)

		opts, err = ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed for double sign")
		txDS, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
		utils.AssertNoError(t, err, "Should allow double sign punishment after resign")
		ctx.WaitMined(txDS.Hash())
	})

	ctx.WaitIfEpochBlock()

	// [P-22] Exit + Double Sign
	t.Run("P-22_ExitThenDoubleSign", func(t *testing.T) {
		key, addr, err := createAndRegisterValidator(t, "P-22 ExitDS")
		utils.AssertNoError(t, err, "create val failed")
		opts, err := ctx.GetTransactor(key)
		utils.AssertNoError(t, err, "transactor failed")

		txR, _ := ctx.Staking.ResignValidator(opts)
		ctx.WaitMined(txR.Hash())
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
		key, addr, err := createAndRegisterValidator(t, "DS Exceptions")
		utils.AssertNoError(t, err, "create val failed")
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
				waitBlocks(t, 1)
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
		for _, k := range ctx.GenesisValidators {
			addr := crypto.PubkeyToAddress(k.PublicKey)
			if addr == miner {
				continue
			}
			if !seen[addr] {
				seen[addr] = true
				targets = append(targets, target{key: k, addr: addr})
			}
			if len(targets) >= 2 {
				break
			}
		}
		if len(targets) < 2 {
			// Fallback to any two distinct genesis validators
			for _, k := range ctx.GenesisValidators {
				addr := crypto.PubkeyToAddress(k.PublicKey)
				if !seen[addr] {
					seen[addr] = true
					targets = append(targets, target{key: k, addr: addr})
				}
				if len(targets) >= 2 {
					break
				}
			}
		}
		if len(targets) < 2 {
			t.Fatalf("insufficient distinct validators for multi-double-sign")
		}

		reporterKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(10))
		utils.AssertNoError(t, err, "failed to setup reporter")

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

			opts, err := ctx.GetTransactor(reporterKey)
			utils.AssertNoError(t, err, "transactor failed for multi double sign")
			tx, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
			utils.AssertNoError(t, err, "failed to submit multi double sign evidence")
			ctx.WaitMined(tx.Hash())
		}

		// Submit evidence for two validators in the same epoch
		submitEvidence(targets[0], 0x41)
		submitEvidence(targets[1], 0x51)

		// Both should be jailed
		for _, tgt := range targets[:2] {
			info, _ := ctx.Staking.GetValidatorInfo(nil, tgt.addr)
			utils.AssertTrue(t, info.IsJailed, "validator should be jailed after double sign")
		}

		// Ensure chain still progresses with remaining validator(s)
		if err := ctx.WaitForBlockProgress(2, 30*time.Second); err != nil {
			t.Fatalf("chain did not progress after multi-validator double sign: %v", err)
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
