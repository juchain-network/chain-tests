package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestB_Governance(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if len(ctx.GenesisValidators) == 0 {
		t.Fatalf("No genesis validators configured")
	}

	// Proposer rotation counter
	proposerIndex := 0

	// Helper to extract proposal ID
	getPropID := func(tx *types.Transaction) [32]byte {
		if tx == nil {
			return [32]byte{}
		}
		for _, client := range ctx.Clients {
			receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
			if err != nil || receipt == nil {
				continue
			}
			for _, log := range receipt.Logs {
				if ev, err := ctx.Proposal.ParseLogCreateProposal(*log); err == nil {
					return ev.Id
				}
				if ev, err := ctx.Proposal.ParseLogCreateConfigProposal(*log); err == nil {
					return ev.Id
				}
			}
		}
		return [32]byte{}
	}

	broadcastTx := func(tx *types.Transaction) {
		if tx == nil {
			return
		}
		for _, client := range ctx.Clients {
			_ = client.SendTransaction(context.Background(), tx)
		}
	}

	// Helper to find an active validator proposer
	getActiveProposer := func() *ecdsa.PrivateKey {
		for attempt := 0; attempt < 3; attempt++ {
			for i := 0; i < len(ctx.GenesisValidators)*2; i++ {
				k := ctx.GenesisValidators[proposerIndex%len(ctx.GenesisValidators)]
				proposerIndex++
				addr := crypto.PubkeyToAddress(k.PublicKey)
				active, _ := ctx.Validators.IsValidatorActive(nil, addr)
				if !active {
					continue
				}
				info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
				if !info.IsJailed {
					return k
				}
			}
			waitForNextEpochBlock(t)
			waitBlocks(t, 1)
		}
		return nil
	}

	voteProposalToPass := func(t *testing.T, propID [32]byte, name string) {
		if propID == ([32]byte{}) {
			t.Fatalf("%s failed: missing proposal ID", name)
		}

		votingCount, err := ctx.Validators.GetVotingValidatorCount(nil)
		if err != nil {
			t.Fatalf("%s failed: get voting validator count: %v", name, err)
		}
		threshold := votingCount.Uint64()/2 + 1
		var votes uint64

	voterLoop:
		for _, vk := range ctx.GenesisValidators {
			if votes >= threshold {
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

			existing, err := ctx.Proposal.Votes(nil, voterAddr, propID)
			if err == nil && existing.VoteTime != nil && existing.VoteTime.Sign() > 0 {
				if !existing.Auth {
					t.Fatalf("%s failed: existing reject vote from %s", name, voterAddr.Hex())
				}
				votes++
				continue
			}

			var lastErr error
			for vtry := 0; vtry < 5; vtry++ {
				vo, _ := ctx.GetTransactor(vk)
				txV, errV := ctx.Proposal.VoteProposal(vo, propID, true)
				if errV != nil {
					if strings.Contains(errV.Error(), "Epoch block forbidden") || strings.Contains(errV.Error(), "too frequent") || strings.Contains(errV.Error(), "nonce") {
						ctx.RefreshNonce(voterAddr)
						waitBlocks(t, 1)
						continue
					}
					if strings.Contains(errV.Error(), "You can't vote for a proposal twice") {
						ctx.RefreshNonce(voterAddr)
						existing, err := ctx.Proposal.Votes(nil, voterAddr, propID)
						if err == nil && existing.VoteTime != nil && existing.VoteTime.Sign() > 0 && existing.Auth {
							votes++
							continue voterLoop
						}
						t.Fatalf("%s failed: duplicate vote without record from %s", name, voterAddr.Hex())
					}
					t.Fatalf("%s failed: vote submit from %s: %v", name, voterAddr.Hex(), errV)
				}

				broadcastTx(txV)
				if errW := ctx.WaitMined(txV.Hash()); errW != nil {
					if strings.Contains(errW.Error(), "Epoch block forbidden") {
						waitBlocks(t, 1)
						lastErr = errW
						continue
					}
					if strings.Contains(errW.Error(), "revert") || strings.Contains(errW.Error(), "reverted") {
						existing, err := ctx.Proposal.Votes(nil, voterAddr, propID)
						if err == nil && existing.VoteTime != nil && existing.VoteTime.Sign() > 0 && existing.Auth {
							votes++
							continue voterLoop
						}
						activeNow, _ := ctx.Validators.IsValidatorActive(nil, voterAddr)
						infoNow, _ := ctx.Staking.GetValidatorInfo(nil, voterAddr)
						if !activeNow || infoNow.IsJailed {
							ctx.RefreshNonce(voterAddr)
							continue voterLoop
						}
						ctx.RefreshNonce(voterAddr)
						waitBlocks(t, 1)
						lastErr = errW
						continue
					}
					if strings.Contains(errW.Error(), "timeout waiting for tx") {
						existing, err := ctx.Proposal.Votes(nil, voterAddr, propID)
						if err == nil && existing.VoteTime != nil && existing.VoteTime.Sign() > 0 && existing.Auth {
							votes++
							continue voterLoop
						}
					}
					t.Fatalf("%s failed: vote tx from %s: %v", name, voterAddr.Hex(), errW)
				}
				// Wait 1 block for state to settle
				waitBlocks(t, 1)
				votes++
				continue voterLoop
			}

			if lastErr != nil {
				activeNow, _ := ctx.Validators.IsValidatorActive(nil, voterAddr)
				infoNow, _ := ctx.Staking.GetValidatorInfo(nil, voterAddr)
				if activeNow && !infoNow.IsJailed {
					t.Fatalf("%s failed: vote from %s did not succeed after retries: %v", name, voterAddr.Hex(), lastErr)
				}
			}
		}

		if votes < threshold {
			t.Fatalf("%s failed: only %d votes, need %d", name, votes, threshold)
		}
	}

	// Helper to update config and verify applied
	updateConfigAndWait := func(t *testing.T, cid uint256, val int64, name string) {
		var tx *types.Transaction
		var err error
		for retry := 0; retry < 5; retry++ {
			pk := getActiveProposer()
			if pk == nil {
				t.Fatalf("no active proposer available for %s", name)
			}
			opts, _ := ctx.GetTransactor(pk)
			tx, err = ctx.Proposal.CreateUpdateConfigProposal(opts, big.NewInt(int64(cid)), big.NewInt(val))
			if err == nil {
				broadcastTx(tx)
				if errW := ctx.WaitMined(tx.Hash()); errW != nil {
					if strings.Contains(errW.Error(), "timeout waiting for tx") {
						waitBlocks(t, 1)
						continue
					}
					t.Fatalf("update config %s tx failed: %v", name, errW)
				}
				break
			}
			if strings.Contains(err.Error(), "Proposal creation too frequent") {
				ctx.RefreshNonce(crypto.PubkeyToAddress(pk.PublicKey))
				waitBlocks(t, 1)
				continue
			}
			if strings.Contains(err.Error(), "Validator only") {
				ctx.RefreshNonce(crypto.PubkeyToAddress(pk.PublicKey))
				continue
			}
			t.Fatalf("setup config %s failed: %v", name, err)
		}
		if tx == nil {
			t.Fatalf("setup config %s failed: no tx", name)
		}

		propID := getPropID(tx)
		voteProposalToPass(t, propID, name)

		waitBlocks(t, 1)
		current, err := ctx.GetConfigValue(int64(cid))
		if err != nil {
			t.Fatalf("read config %s failed: %v", name, err)
		}
		if current.Cmp(big.NewInt(val)) != 0 {
			t.Fatalf("config %s not applied: expected %d, got %v", name, val, current)
		}
	}

	// Setup: Ensure stable config for this test group
	if !t.Run("Setup_Governance", func(t *testing.T) {
		updateConfigAndWait(t, 0, 1000, "ProposalLastingPeriod")
		updateConfigAndWait(t, 19, 1, "ProposalCooldown")
	}) {
		t.Fatal("Setup_Governance failed")
	}

	// Helper to create and pass a proposal
	createAndPassProposal := func(dst common.Address, flag bool, desc string) error {
		proposerKey := getActiveProposer()
		if proposerKey == nil {
			return fmt.Errorf("no active proposer available")
		}
		var tx *types.Transaction
		var err error

		for attempts := 0; attempts < len(ctx.GenesisValidators)*5; attempts++ {
			proposerOpts, _ := ctx.GetTransactor(proposerKey)
			proposerOpts.Value = nil
			tx, err = ctx.Proposal.CreateProposal(proposerOpts, dst, flag, desc)
			if err == nil {
				break
			}
			if strings.Contains(err.Error(), "Proposal creation too frequent") {
				ctx.RefreshNonce(crypto.PubkeyToAddress(proposerKey.PublicKey))
				waitBlocks(t, 1)
				continue
			}
			if strings.Contains(err.Error(), "Validator only") {
				ctx.RefreshNonce(crypto.PubkeyToAddress(proposerKey.PublicKey))
				proposerKey = getActiveProposer()
				if proposerKey == nil {
					return fmt.Errorf("no active proposer available")
				}
				continue
			}
			return fmt.Errorf("createProposal failed: %w", err)
		}
		if tx == nil {
			return fmt.Errorf("createProposal failed: %v", err)
		}
		broadcastTx(tx)
		if errW := ctx.WaitMined(tx.Hash()); errW != nil {
			return fmt.Errorf("createProposal tx failed: %w", errW)
		}
		// Wait 1 block for proposal to be indexed
		waitBlocks(t, 1)

		proposalID := getPropID(tx)
		if proposalID == ([32]byte{}) {
			return fmt.Errorf("createProposal missing proposal ID")
		}

		voteProposalToPass(t, proposalID, desc)

		waitBlocks(t, 1)
		pass, _ := ctx.Proposal.Pass(nil, dst)
		if flag && !pass {
			return fmt.Errorf("proposal should be passed")
		}
		if !flag && pass {
			return fmt.Errorf("proposal should be removed")
		}
		return nil
	}

	t.Run("G-01_AddValidator", func(t *testing.T) {
		_, addr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "create account failed")
		err = createAndPassProposal(addr, true, "G-01 Add")
		utils.AssertNoError(t, err, "add validator proposal failed")
	})

	t.Run("G-02_RemoveValidator", func(t *testing.T) {
		_, addr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "create account failed")
		createAndPassProposal(addr, true, "G-02 Prep") // Pass it first
		err = createAndPassProposal(addr, false, "G-02 Remove")
		utils.AssertNoError(t, err, "remove validator proposal failed")
	})

	t.Run("G-03_ReOnboard", func(t *testing.T) {
		_, addr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "create account failed")
		err = createAndPassProposal(addr, true, "G-03 Add")
		utils.AssertNoError(t, err, "revive proposal failed")
	})

	t.Run("G-13_FlipFlop", func(t *testing.T) {
		_, addr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "create account failed")
		err = createAndPassProposal(addr, true, "G-13 Add")
		utils.AssertNoError(t, err, "G-13 add failed")
		err = createAndPassProposal(addr, false, "G-13 Remove")
		utils.AssertNoError(t, err, "G-13 remove failed")
	})

	t.Run("G-11_GhostRemoval", func(t *testing.T) {
		randomAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		err := createAndPassProposal(randomAddr, false, "G-11 Ghost")
		utils.AssertNoError(t, err, "ghost removal failed")
	})

	t.Run("G-06_DuplicateProposal", func(t *testing.T) {
		_, addr, err := ctx.CreateAndFundAccount(utils.ToWei(1))
		utils.AssertNoError(t, err, "create account failed")
		createAndPassProposal(addr, true, "G-06 Prep")

		proposerKey := getActiveProposer()
		if proposerKey == nil {
			t.Fatal("no active proposer available for duplicate test")
		}
		opts, _ := ctx.GetTransactor(proposerKey)
		_, err = ctx.Proposal.CreateProposal(opts, addr, true, "G-06 Should Fail")
		if err == nil {
			t.Fatal("Expected failure for already passed dst, got success")
		}
		if !strings.Contains(err.Error(), "already passed") && !strings.Contains(err.Error(), "execution reverted") {
			t.Errorf("Unexpected error: %v", err)
		}
		ctx.RefreshNonce(crypto.PubkeyToAddress(proposerKey.PublicKey))
		t.Log("Duplicate add rejected correctly:", err)
	})

	t.Run("G-05_Cooldown", func(t *testing.T) {
		updateConfigAndWait(t, 19, 5, "ProposalCooldown=5")

		proposerKey := getActiveProposer()
		if proposerKey == nil {
			t.Fatal("no active proposer available for cooldown test")
		}

		var tx *types.Transaction
		var err error
		for retry := 0; retry < 6; retry++ {
			opts, _ := ctx.GetTransactor(proposerKey)
			tx, err = ctx.Proposal.CreateProposal(opts, common.HexToAddress("0x9999"), false, "G-05 1")
			if err == nil {
				broadcastTx(tx)
				if errW := ctx.WaitMined(tx.Hash()); errW != nil {
					t.Fatalf("first proposal tx failed: %v", errW)
				}
				break
			}
			if strings.Contains(err.Error(), "Proposal creation too frequent") {
				ctx.RefreshNonce(crypto.PubkeyToAddress(proposerKey.PublicKey))
				waitBlocks(t, 1)
				continue
			}
			t.Fatalf("first proposal failed: %v", err)
		}
		if tx == nil {
			t.Fatal("first proposal failed: no tx")
		}

		opts, _ := ctx.GetTransactor(proposerKey)
		tx2, err2 := ctx.Proposal.CreateProposal(opts, common.HexToAddress("0x8888"), false, "G-05 2")
		if err2 == nil {
			broadcastTx(tx2)
			if errW := ctx.WaitMined(tx2.Hash()); errW == nil || !strings.Contains(errW.Error(), "Proposal creation too frequent") {
				t.Fatalf("expected cooldown revert, got: %v", errW)
			}
		} else {
			ctx.RefreshNonce(crypto.PubkeyToAddress(proposerKey.PublicKey))
			if !strings.Contains(err2.Error(), "Proposal creation too frequent") {
				t.Fatalf("expected cooldown error, got: %v", err2)
			}
		}
		t.Log("Cooldown triggered correctly")

		t.Log("Waiting for cooldown to expire...")
		waitBlocks(t, 2)

		updateConfigAndWait(t, 19, 1, "ProposalCooldown=1")
	})

	t.Run("G-07_FrontRunning", func(t *testing.T) {
		fakeValKey, _, _ := ctx.CreateAndFundAccount(utils.ToWei(100005))
		regOpts, _ := ctx.GetTransactor(fakeValKey)
		regOpts.Value = utils.ToWei(100000)
		_, err := ctx.Staking.RegisterValidator(regOpts, big.NewInt(1000))
		if err == nil {
			t.Fatal("Expected register failure (no proposal)")
		}
		ctx.RefreshNonce(crypto.PubkeyToAddress(fakeValKey.PublicKey))
		t.Log("Registration without proposal correctly blocked:", err)
	})

	t.Run("V-02_DescriptionBoundary", func(t *testing.T) {
		proposerKey := getActiveProposer()
		if proposerKey == nil {
			t.Fatal("no active proposer available for description test")
		}
		opts, _ := ctx.GetTransactor(proposerKey)
		valAddr := crypto.PubkeyToAddress(proposerKey.PublicKey)
		longMoniker := strings.Repeat("a", 71)
		_, err := ctx.Validators.CreateOrEditValidator(opts, valAddr, longMoniker, "", "", "", "")
		if err == nil {
			t.Fatal("Should fail with moniker > 70 bytes")
		}
		ctx.RefreshNonce(crypto.PubkeyToAddress(proposerKey.PublicKey))
		t.Logf("Caught expected error: %v", err)
	})

	t.Run("G-16_SmoothExpansion", func(t *testing.T) {
		currentSet, _ := ctx.Validators.GetActiveValidators(nil)
		initialCount := len(currentSet)
		epochBI, err := ctx.Proposal.Epoch(nil)
		if err != nil || epochBI.Sign() == 0 {
			t.Fatalf("epoch not available")
		}
		epoch := epochBI.Uint64()
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || header == nil {
			t.Fatalf("failed to read header: %v", err)
		}
		cur := header.Number.Uint64()
		blocksInto := cur % epoch
		remaining := epoch - blocksInto
		minRemaining := uint64(50)
		if epoch < minRemaining {
			minRemaining = epoch / 2
			if minRemaining < 10 && epoch > 1 {
				minRemaining = epoch - 1
			}
		}
		if remaining < minRemaining {
			t.Logf("Not enough blocks in current epoch (%d remaining, need >=%d), waiting for next epoch...", remaining, minRemaining)
			waitForNextEpochBlock(t)
			waitBlocks(t, 1)
		} else if blocksInto == 0 {
			// Avoid onlyNotEpoch reverts on epoch blocks.
			waitBlocks(t, 1)
		}

		v1Key, v1Addr, err := ctx.CreateAndFundAccount(utils.ToWei(100005))
		utils.AssertNoError(t, err, "create v1 failed")
		createAndPassProposal(v1Addr, true, "G-16 V1")

		v1Opts, _ := ctx.GetTransactor(v1Key)
		v1Opts.Value = utils.ToWei(100000)
		ctx.WaitIfEpochBlock()
		tx1, err := ctx.Staking.RegisterValidator(v1Opts, big.NewInt(1000))
		utils.AssertNoError(t, err, "v1 register failed")
		if errW := ctx.WaitMined(tx1.Hash()); errW != nil {
			t.Fatalf("v1 register tx failed: %v", errW)
		}
		r1, err := ctx.Clients[0].TransactionReceipt(context.Background(), tx1.Hash())
		if err != nil || r1 == nil {
			t.Fatalf("failed to read v1 receipt: %v", err)
		}
		epochIdV1 := r1.BlockNumber.Uint64() / epoch

		v2Key, v2Addr, err := ctx.CreateAndFundAccount(utils.ToWei(100005))
		utils.AssertNoError(t, err, "create v2 failed")
		createAndPassProposal(v2Addr, true, "G-16 V2")

		v2Opts, _ := ctx.GetTransactor(v2Key)
		v2Opts.Value = utils.ToWei(100000)
		tx2, err := ctx.Staking.RegisterValidator(v2Opts, big.NewInt(1000))
		if err != nil {
			ctx.RefreshNonce(crypto.PubkeyToAddress(v2Key.PublicKey))
			header, herr := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
			if herr == nil && header != nil {
				epochIdNow := header.Number.Uint64() / epoch
				if epochIdNow != epochIdV1 {
					t.Fatalf("epoch advanced before v2 tx accepted (v1=%d now=%d); same-epoch assertion invalid", epochIdV1, epochIdNow)
				}
			}
			t.Log("V2 registration correctly blocked in same epoch:", err)
		} else {
			errW := ctx.WaitMined(tx2.Hash())
			r2, rerr := ctx.Clients[0].TransactionReceipt(context.Background(), tx2.Hash())
			if rerr != nil || r2 == nil {
				if errW != nil {
					t.Fatalf("v2 register failed but receipt missing: %v", errW)
				}
				t.Fatalf("failed to read v2 receipt: %v", rerr)
			}
			epochIdV2 := r2.BlockNumber.Uint64() / epoch
			if epochIdV2 != epochIdV1 {
				t.Fatalf("epoch advanced (v1=%d v2=%d); same-epoch assertion invalid", epochIdV1, epochIdV2)
			}
			if errW != nil {
				t.Logf("V2 registration reverted as expected: %v", errW)
			} else {
				t.Fatalf("V2 register succeeded in the same epoch; expected block")
			}
		}

		topValidators, _ := ctx.Validators.GetTopValidators(nil)
		t.Logf("Smooth expansion check: initial=%d current=%d", initialCount, len(topValidators))
	})
}

func waitNextBlock() {
	waitBlocks(nil, 1)
}
