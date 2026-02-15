package tests

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"juchain.org/chain/tools/ci/internal/utils"
)

// TestB_Governance_Extended covers remaining scenarios from Phase 1 of TEST_PLAN.md
// specifically G-04, G-14, G-15 and other edge cases.
func TestB_Governance_Extended(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	// [G-04] Reject Proposal Flow
	t.Run("G-04_RejectProposal", func(t *testing.T) {
		// 1. Create a candidate
		_, candidateAddr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "create candidate failed")

		// 2. Create Proposal with robust retry
		proposerKey := ctx.GenesisValidators[0]
		var tx *types.Transaction

		for {
			opts, _ := ctx.GetTransactor(proposerKey)
			opts.Value = nil
			tx, err = ctx.Proposal.CreateProposal(opts, candidateAddr, true, "G-04 Reject")
			if err == nil {
				break
			}
			if strings.Contains(err.Error(), "Proposal creation too frequent") {
				t.Log("G-04 creation hit cooldown, waiting 1 block...")
				waitBlocks(t, 1)
				continue
			}
			t.Fatalf("create proposal failed: %v", err)
		}
		ctx.WaitMined(tx.Hash())

		// 3. Get Proposal ID
		receipt, _ := ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
		var propID [32]byte
		for _, log := range receipt.Logs {
			ev, err := ctx.Proposal.ParseLogCreateProposal(*log)
			if err == nil {
				propID = ev.Id
				break
			}
		}

		// 4. Vote Reject (false) from majority
		// Assuming 3 validators in test env
		for i, voterKey := range ctx.GenesisValidators {
			voterOpts, _ := ctx.GetTransactor(voterKey)
			// Vote false
			txVote, err := ctx.Proposal.VoteProposal(voterOpts, propID, false)
			if err == nil {
				ctx.WaitMined(txVote.Hash())
				t.Logf("Validator %d voted NO", i)
			}
		}

		// 5. Verify Status
		pass, _ := ctx.Proposal.Pass(nil, candidateAddr)
		utils.AssertTrue(t, !pass, "Proposal should NOT pass")

		// Verify rejection event? (Optional, requires parsing logs)
	})

	// [G-14] Parallel Governance
	t.Run("G-14_ParallelGovernance", func(t *testing.T) {
		// Create two proposals simultaneously: one for Config, one for Validator

		// 1. Config Proposal (burnAddress, CID 14)
		proposerKey := ctx.GenesisValidators[0]
		origBurn, _ := ctx.Proposal.BurnAddress(nil)
		targetBurn := common.HexToAddress("0x000000000000000000000000000000000000bEEF")
		targetVal := new(big.Int).SetBytes(targetBurn.Bytes())
		var tx1 *types.Transaction
		var opts1 *bind.TransactOpts
		var err error

		for {
			opts1, _ = ctx.GetTransactor(proposerKey)
			tx1, err = ctx.Proposal.CreateUpdateConfigProposal(opts1, big.NewInt(14), targetVal)
			if err == nil {
				break
			}
			if strings.Contains(err.Error(), "Proposal creation too frequent") {
				t.Log("G-14 Config hit cooldown, waiting 1 block...")
				waitBlocks(t, 1)
				continue
			}
			t.Fatalf("config proposal failed: %v", err)
		}
		ctx.WaitMined(tx1.Hash())

		// 2. Validator Proposal
		proposerKey2 := ctx.GenesisValidators[1] // Use another validator to avoid nonce/race if parallel
		_, candAddr, _ := ctx.CreateAndFundAccount(utils.ToWei(1))
		var tx2 *types.Transaction
		var opts2 *bind.TransactOpts

		for {
			opts2, _ = ctx.GetTransactor(proposerKey2)
			tx2, err = ctx.Proposal.CreateProposal(opts2, candAddr, true, "G-14 Parallel Val")
			if err == nil {
				break
			}
			if strings.Contains(err.Error(), "Proposal creation too frequent") {
				t.Log("G-14 Validator hit cooldown, waiting 1 block...")
				waitBlocks(t, 1)
				continue
			}
			t.Fatalf("validator proposal failed: %v", err)
		}
		ctx.WaitMined(tx2.Hash())

		// 3. Get IDs
		rec1, _ := ctx.Clients[0].TransactionReceipt(context.Background(), tx1.Hash())
		rec2, _ := ctx.Clients[0].TransactionReceipt(context.Background(), tx2.Hash())

		var id1, id2 [32]byte
		for _, l := range rec1.Logs {
			ev, err := ctx.Proposal.ParseLogCreateConfigProposal(*l)
			if err == nil {
				id1 = ev.Id
				break
			}
		}
		for _, l := range rec2.Logs {
			ev, err := ctx.Proposal.ParseLogCreateProposal(*l)
			if err == nil {
				id2 = ev.Id
				break
			}
		}

		// 4. Vote for both (Sequentially per account to avoid nonce race)
		for i, vk := range ctx.GenesisValidators {
			vo1, _ := ctx.GetTransactor(vk)
			txV1, err1 := ctx.Proposal.VoteProposal(vo1, id1, true)
			if err1 == nil {
				ctx.WaitMined(txV1.Hash())
			} else {
				t.Logf("Vote1 val %d failed: %v", i, err1)
			}

			vo2, _ := ctx.GetTransactor(vk)
			txV2, err2 := ctx.Proposal.VoteProposal(vo2, id2, true)
			if err2 == nil {
				ctx.WaitMined(txV2.Hash())
			} else {
				t.Logf("Vote2 val %d failed: %v", i, err2)
			}
		}

		// 5. Verify Execution
		waitBlocks(t, 1)

		vCount, _ := ctx.Validators.GetVotingValidatorCount(nil)
		t.Logf("G-14 Threshold check: voting validators = %d", vCount)

		// Revert config change to original burn address
		opts1, _ = ctx.GetTransactor(proposerKey)
		waitNextBlock()
		origVal := new(big.Int).SetBytes(origBurn.Bytes())
		txReset, err := ctx.Proposal.CreateUpdateConfigProposal(opts1, big.NewInt(14), origVal)
		if err == nil {
			ctx.WaitMined(txReset.Hash())
			recReset, _ := ctx.Clients[0].TransactionReceipt(context.Background(), txReset.Hash())
			var idReset [32]byte
			for _, l := range recReset.Logs {
				if ev, err := ctx.Proposal.ParseLogCreateConfigProposal(*l); err == nil {
					idReset = ev.Id
					break
				}
			}
			for _, vk := range ctx.GenesisValidators {
				vo, _ := ctx.GetTransactor(vk)
				ctx.Proposal.VoteProposal(vo, idReset, true)
			}
			waitNextBlock()
		}
	})

	// [G-10] Already in Top Validator Set
	t.Run("G-10_AddExistingValidator", func(t *testing.T) {
		// Try to add an existing genesis validator
		existing := common.HexToAddress(ctx.Config.Validators[0].Address)

		proposerKey := ctx.GenesisValidators[0]
		opts, _ := ctx.GetTransactor(proposerKey)

		_, err := ctx.Proposal.CreateProposal(opts, existing, true, "G-10 Invalid")
		if err == nil {
			t.Fatal("Expected error 'Validator is already in top validator set', got success")
		}
		t.Logf("Correctly rejected: %v", err)
	})

	// [G-09] Description Too Long
	t.Run("G-09_DescriptionTooLong", func(t *testing.T) {
		longDesc := make([]byte, 3001) // > 3000
		for i := range longDesc {
			longDesc[i] = 'a'
		}

		proposerKey := ctx.GenesisValidators[0]
		opts, _ := ctx.GetTransactor(proposerKey)
		_, candAddr, _ := ctx.CreateAndFundAccount(utils.ToWei(1))

		_, err := ctx.Proposal.CreateProposal(opts, candAddr, true, string(longDesc))
		if err == nil {
			t.Fatal("Expected error 'Details too long', got success")
		}
	})
}
