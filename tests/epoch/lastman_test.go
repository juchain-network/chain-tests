package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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
			time.Sleep(200 * time.Millisecond)
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
			time.Sleep(100 * time.Millisecond)
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
}
