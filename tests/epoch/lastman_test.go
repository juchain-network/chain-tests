package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/utils"
)

// TestZ_LastManStanding ensures the last validator cannot be removed (protection path).
// This is destructive; keep it at the end of the suite.
func TestZ_LastManStanding(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) < 3 {
		t.Fatalf("Need at least 3 genesis validators")
	}
	ensureMinActiveValidators(t, 3, 2)
	highest, _ := ctx.Validators.GetHighestValidators(nil)
	if len(highest) <= 1 {
		t.Fatalf("validator set already reduced to 1")
	}

	pIndex := 0
	waitForProgress := func(label string, timeout time.Duration) bool {
		start, _ := ctx.Clients[0].BlockNumber(context.Background())
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			cur, _ := ctx.Clients[0].BlockNumber(context.Background())
			if cur > start {
				return true
			}
			time.Sleep(blockPollInterval())
		}
		t.Logf("block did not advance after %s (start=%d)", label, start)
		return false
	}
	waitReceipt := func(txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
		if txHash == (common.Hash{}) {
			return nil, fmt.Errorf("empty tx hash")
		}
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			receipt, err := ctx.Clients[0].TransactionReceipt(context.Background(), txHash)
			if err == nil && receipt != nil {
				if receipt.Status == 0 {
					return receipt, fmt.Errorf("transaction %s reverted", txHash.Hex())
				}
				return receipt, nil
			}
			time.Sleep(retrySleep())
		}
		return nil, fmt.Errorf("timeout waiting for tx %s", txHash.Hex())
	}
	removeByProposal := func(target common.Address, name string) error {
		proposerKey := getNextProposerOrSkip(t, &pIndex)
		opts, _ := ctx.GetTransactor(proposerKey)
		opts.Value = nil

		tx, err := ctx.Proposal.CreateProposal(opts, target, false, name)
		if err != nil && strings.Contains(err.Error(), "Proposal creation too frequent") {
			waitNextBlock()
			tx, err = ctx.Proposal.CreateProposal(opts, target, false, name+" Retry")
		}
		if err != nil {
			return err
		}
		if errW := ctx.WaitMined(tx.Hash()); errW != nil {
			return errW
		}

		propID := getPropID(tx)
		if propID == ([32]byte{}) {
			return fmt.Errorf("missing proposal id for %s", target.Hex())
		}
		voteProposalToPass(t, propID, name)

		pass, _ := ctx.Proposal.Pass(nil, target)
		if pass {
			return fmt.Errorf("expected pass=false for removed validator %s", target.Hex())
		}
		return nil
	}

	// Remove two validators to leave only one.
	v1 := common.HexToAddress(ctx.Config.Validators[1].Address)
	v2 := common.HexToAddress(ctx.Config.Validators[2].Address)
	utils.AssertNoError(t, removeByProposal(v1, "G-12 Remove V1"), "remove v1 failed")
	if !waitForProgress("after removing v1", 20*time.Second) {
		t.Fatalf("chain stalled after removing validator %s", v1.Hex())
	}
	utils.AssertNoError(t, removeByProposal(v2, "G-12 Remove V2"), "remove v2 failed")
	if !waitForProgress("after removing v2", 20*time.Second) {
		t.Fatalf("chain stalled after removing validator %s", v2.Hex())
	}

	highest, _ = ctx.Validators.GetHighestValidators(nil)
	if len(highest) != 1 {
		t.Fatalf("expected highestValidatorsSet length = 1 after removals, got %d", len(highest))
	}
	// If the chain stalls after reaching a single validator, skip the final removal attempt
	// to avoid hanging the suite and document the issue separately.
	if !waitForProgress("reducing to 1 validator", 20*time.Second) {
		t.Fatalf("chain stalled after reducing to 1 validator; last-man removal attempt cannot proceed")
	}

	// Now attempt to remove the last remaining validator.
	last := common.HexToAddress(ctx.Config.Validators[0].Address)
	proposerKey := getNextProposerOrSkip(t, &pIndex)
	opts, _ := ctx.GetTransactor(proposerKey)
	tx, err := ctx.Proposal.CreateProposal(opts, last, false, "G-12 Last Man")
	if err != nil && strings.Contains(err.Error(), "Proposal creation too frequent") {
		waitNextBlock()
		tx, err = ctx.Proposal.CreateProposal(opts, last, false, "G-12 Last Man Retry")
	}
	utils.AssertNoError(t, err, "last man proposal failed")
	receipt, err := waitReceipt(tx.Hash(), 30*time.Second)
	if err != nil {
		t.Fatalf("last man proposal not mined in time: %v", err)
	}
	var propID [32]byte
	for _, l := range receipt.Logs {
		if ev, err := ctx.Proposal.ParseLogCreateProposal(*l); err == nil {
			propID = ev.Id
			break
		}
	}
	if propID == ([32]byte{}) {
		t.Fatalf("missing proposal id for last man")
	}

	// Cast votes with short waits to avoid hanging if the chain stalls.
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
		vo, _ := ctx.GetTransactor(vk)
		txVote, err := ctx.Proposal.VoteProposal(vo, propID, true)
		if err != nil {
			if strings.Contains(err.Error(), "You can't vote for a proposal twice") || strings.Contains(err.Error(), "Proposal already passed") {
				continue
			}
			if strings.Contains(err.Error(), "Epoch block forbidden") {
				waitNextBlock()
				continue
			}
			continue
		}
		if _, err := waitReceipt(txVote.Hash(), 30*time.Second); err != nil {
			// Receipt-level reverts are possible once the proposal is already finalized
			// while later validators are still attempting to vote.
			if strings.Contains(err.Error(), "reverted") {
				if passed, _ := ctx.Proposal.Pass(nil, last); passed {
					break
				}
				waitNextBlock()
				continue
			}
			t.Fatalf("last man vote not mined in time: %v", err)
		}
	}

	// Protection: last validator should remain in highest set.
	highest, _ = ctx.Validators.GetHighestValidators(nil)
	if len(highest) != 1 {
		t.Fatalf("expected highestValidatorsSet length = 1, got %d", len(highest))
	}
	if highest[0] != last {
		t.Fatalf("expected last validator %s to remain, got %s", last.Hex(), highest[0].Hex())
	}

	t.Run("P-04_LastEffectiveSlashFloor", func(t *testing.T) {
		minStake, err := ctx.Proposal.MinValidatorStake(nil)
		if err != nil || minStake == nil {
			t.Fatalf("failed to read minValidatorStake: %v", err)
		}

		effectiveTop, err := ctx.Validators.GetEffectiveTopValidators(nil)
		if err != nil {
			t.Fatalf("failed to query effective top validators: %v", err)
		}
		if len(effectiveTop) != 1 || effectiveTop[0] != last {
			t.Fatalf("expected last validator %s to be sole effective top validator, got %v", last.Hex(), effectiveTop)
		}
		isLastEffective, err := ctx.Validators.IsLastEffectiveValidator(nil, last)
		if err != nil {
			t.Fatalf("failed to query isLastEffectiveValidator: %v", err)
		}
		if !isLastEffective {
			t.Fatalf("expected isLastEffectiveValidator(%s)=true", last.Hex())
		}

		infoBefore, err := ctx.Staking.GetValidatorInfo(nil, last)
		if err != nil {
			t.Fatalf("failed to query validator info before slash check: %v", err)
		}
		selfBefore := new(big.Int).Set(infoBefore.SelfStake)

		burnAddr, err := ctx.Proposal.BurnAddress(nil)
		if err != nil {
			t.Fatalf("failed to query burnAddress: %v", err)
		}
		if burnAddr == (common.Address{}) {
			t.Fatalf("burnAddress must be non-zero")
		}

		reporterAddr := common.Address{}
		if ctx.FunderKey != nil {
			reporterAddr = crypto.PubkeyToAddress(ctx.FunderKey.PublicKey)
		}
		if reporterAddr == (common.Address{}) {
			reporterAddr = crypto.PubkeyToAddress(ctx.GenesisValidators[0].PublicKey)
		}

		stakingABI, err := contracts.StakingMetaData.GetAbi()
		if err != nil || stakingABI == nil {
			t.Fatalf("failed to load staking ABI: %v", err)
		}

		slashRequest := new(big.Int).Mul(new(big.Int).Set(selfBefore), big.NewInt(2))
		callData, err := stakingABI.Pack("slashValidator", last, slashRequest, reporterAddr, big.NewInt(0), burnAddr)
		if err != nil {
			t.Fatalf("failed to pack slashValidator call: %v", err)
		}
		msg := ethereum.CallMsg{
			From: reporterAddr,
			To:   &testctx.StakingAddr,
			Gas:  3_000_000,
			Data: callData,
		}
		// Simulate slash as if called by Punish contract to validate low-level cap logic.
		msg.From = testctx.PunishAddr
		raw, err := ctx.Clients[0].CallContract(context.Background(), msg, nil)
		if err != nil {
			t.Fatalf("slashValidator low-level call failed: %v", err)
		}
		out, err := stakingABI.Unpack("slashValidator", raw)
		if err != nil || len(out) != 2 {
			t.Fatalf("failed to decode slashValidator return: out=%v err=%v", out, err)
		}
		simulatedSlash := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
		simulatedReward := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

		expectedMaxSlash := big.NewInt(0)
		if selfBefore.Cmp(minStake) > 0 {
			expectedMaxSlash = new(big.Int).Sub(selfBefore, minStake)
		}
		if simulatedSlash.Cmp(expectedMaxSlash) != 0 {
			t.Fatalf("slashValidator cap mismatch: got=%s want=%s selfBefore=%s minStake=%s",
				simulatedSlash.String(), expectedMaxSlash.String(), selfBefore.String(), minStake.String())
		}
		if simulatedReward.Sign() != 0 {
			t.Fatalf("expected zero reward in low-level slash call, got=%s", simulatedReward.String())
		}

		doubleSignWindow, _ := ctx.Proposal.DoubleSignWindow(nil)
		targetWindow := big.NewInt(8)
		if doubleSignWindow == nil || doubleSignWindow.Cmp(targetWindow) > 0 {
			_ = ctx.EnsureConfig(15, targetWindow, doubleSignWindow)
		}

		reporterKey, _, err := ctx.CreateAndFundAccount(utils.ToWei(5))
		if err != nil {
			t.Fatalf("failed to create reporter account: %v", err)
		}
		reporterEOA := crypto.PubkeyToAddress(reporterKey.PublicKey)
		lastKey := keyForAddress(last)
		if lastKey == nil {
			t.Fatalf("missing key for last validator %s", last.Hex())
		}

		submitDoubleSign := func() (*types.Receipt, error) {
			var lastErr error
			for attempt := 0; attempt < 8; attempt++ {
				ctx.WaitIfEpochBlock()

				head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
				if err != nil || head == nil {
					lastErr = fmt.Errorf("failed to read latest header: %w", err)
					waitNextBlock()
					continue
				}
				targetHeight := new(big.Int).Sub(head.Number, big.NewInt(1))
				if targetHeight.Sign() <= 0 {
					targetHeight = big.NewInt(1)
				}
				baseTime := head.Time
				if h, err := ctx.Clients[0].HeaderByNumber(context.Background(), targetHeight); err == nil && h != nil {
					baseTime = h.Time
				}

				h1 := &types.Header{
					ParentHash:  common.Hash{},
					UncleHash:   types.EmptyUncleHash,
					Coinbase:    last,
					Root:        common.Hash{},
					TxHash:      types.EmptyRootHash,
					ReceiptHash: types.EmptyRootHash,
					Bloom:       types.Bloom{},
					Difficulty:  big.NewInt(1),
					Number:      targetHeight,
					GasLimit:    30_000_000,
					GasUsed:     0,
					Time:        baseTime,
					Extra:       make([]byte, 32+65),
					MixDigest:   common.Hash{},
					Nonce:       types.BlockNonce{},
				}
				h2 := &types.Header{
					ParentHash:  common.Hash{},
					UncleHash:   types.EmptyUncleHash,
					Coinbase:    last,
					Root:        common.Hash{0x01},
					TxHash:      types.EmptyRootHash,
					ReceiptHash: types.EmptyRootHash,
					Bloom:       types.Bloom{},
					Difficulty:  big.NewInt(1),
					Number:      targetHeight,
					GasLimit:    30_000_000,
					GasUsed:     0,
					Time:        baseTime,
					Extra:       make([]byte, 32+65),
					MixDigest:   common.Hash{},
					Nonce:       types.BlockNonce{},
				}

				rlp1, err := signHeaderCliqueForEpoch(h1, lastKey)
				if err != nil {
					lastErr = fmt.Errorf("failed to sign header 1: %w", err)
					waitNextBlock()
					continue
				}
				rlp2, err := signHeaderCliqueForEpoch(h2, lastKey)
				if err != nil {
					lastErr = fmt.Errorf("failed to sign header 2: %w", err)
					waitNextBlock()
					continue
				}

				opts, err := ctx.GetTransactor(reporterKey)
				if err != nil {
					lastErr = fmt.Errorf("failed to create transactor: %w", err)
					waitNextBlock()
					continue
				}

				txDS, err := ctx.Punish.SubmitDoubleSignEvidence(opts, rlp1, rlp2)
				if err != nil {
					lastErr = err
					msg := strings.ToLower(err.Error())
					if strings.Contains(msg, "epoch block forbidden") ||
						strings.Contains(msg, "nonce too low") ||
						strings.Contains(msg, "replacement transaction underpriced") ||
						strings.Contains(msg, "already known") ||
						strings.Contains(msg, "revert") {
						ctx.RefreshNonce(reporterEOA)
						waitNextBlock()
						continue
					}
					return nil, err
				}

				if err := ctx.WaitMined(txDS.Hash()); err != nil {
					lastErr = err
					msg := strings.ToLower(err.Error())
					if strings.Contains(msg, "reverted") ||
						strings.Contains(msg, "epoch block forbidden") ||
						strings.Contains(msg, "timeout waiting for tx") {
						ctx.RefreshNonce(reporterEOA)
						waitNextBlock()
						continue
					}
					return nil, err
				}

				receipt, err := ctx.Clients[0].TransactionReceipt(context.Background(), txDS.Hash())
				if err != nil || receipt == nil {
					lastErr = fmt.Errorf("failed to read double-sign receipt: %w", err)
					waitNextBlock()
					continue
				}
				return receipt, nil
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("double-sign submission retries exhausted")
			}
			return nil, lastErr
		}

		receiptDS, err := submitDoubleSign()
		if err != nil {
			t.Fatalf("failed to submit double-sign on last effective validator: %v", err)
		}

		infoAfter, err := ctx.Staking.GetValidatorInfo(nil, last)
		if err != nil {
			t.Fatalf("failed to query validator info after slash: %v", err)
		}
		selfAfter := new(big.Int).Set(infoAfter.SelfStake)
		if selfAfter.Cmp(minStake) < 0 {
			t.Fatalf("selfStake dropped below minValidatorStake: after=%s min=%s", selfAfter.String(), minStake.String())
		}

		onChainSlash := new(big.Int).Sub(selfBefore, selfAfter)
		if onChainSlash.Sign() < 0 {
			t.Fatalf("invalid slash delta: before=%s after=%s", selfBefore.String(), selfAfter.String())
		}
		if onChainSlash.Cmp(expectedMaxSlash) > 0 {
			t.Fatalf("slash exceeded last-effective cap: slash=%s cap=%s", onChainSlash.String(), expectedMaxSlash.String())
		}
		if onChainSlash.Cmp(simulatedSlash) > 0 {
			t.Fatalf("on-chain slash exceeds low-level simulated cap: slash=%s simulatedCap=%s", onChainSlash.String(), simulatedSlash.String())
		}

		var loggedSlash *big.Int
		for _, l := range receiptDS.Logs {
			if ev, err := ctx.Staking.ParseValidatorSlashed(*l); err == nil && ev.Validator == last {
				loggedSlash = ev.SlashAmount
				break
			}
		}
		if loggedSlash != nil && loggedSlash.Cmp(onChainSlash) != 0 {
			t.Fatalf("slash event mismatch: event=%s state=%s", loggedSlash.String(), onChainSlash.String())
		}

		effectiveAfter, err := ctx.Validators.GetEffectiveTopValidators(nil)
		if err != nil {
			t.Fatalf("failed to query effective top validators after slash: %v", err)
		}
		if len(effectiveAfter) != 1 || effectiveAfter[0] != last {
			t.Fatalf("last effective validator removed unexpectedly after slash: %v", effectiveAfter)
		}
		isLastAfter, err := ctx.Validators.IsLastEffectiveValidator(nil, last)
		if err != nil {
			t.Fatalf("failed to query isLastEffectiveValidator after slash: %v", err)
		}
		if !isLastAfter {
			t.Fatalf("expected last validator to remain last-effective after slash")
		}

		if !waitForProgress("after last-effective slash", 30*time.Second) {
			t.Fatalf("chain stalled after last-effective slash")
		}
	})
}

func signHeaderCliqueForEpoch(h *types.Header, key *ecdsa.PrivateKey) ([]byte, error) {
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
