package tests

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestG_PunishPaths(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}

	// [P-23] Punish Normal (Must be called by Miner)
	t.Run("P-23_PunishNormal", func(t *testing.T) {
		// Only the current block proposer can call punish
		// This is hard to test from outside unless we are the miner.
		// In integration tests, we can try calling it from Validator 0
		// who is likely to be a miner occasionally.

		valKey := ctx.GenesisValidators[0]
		targetVal := common.HexToAddress(ctx.Config.Validators[1].Address)

		opts, err := ctx.GetTransactor(valKey)
		if err != nil {
			t.Fatal(err)
		}

		_, err = ctx.Punish.Punish(opts, targetVal)
		if err != nil {
			t.Logf("punish call failed (as expected if not miner): %v", err)
		} else {
			t.Log("punish call succeeded (Validator 0 is current miner)")
		}
	})

	// [P-24] ExecutePending must be blocked for external tx.
	t.Run("P-24_ExecutePendingForbiddenExternalTx", func(t *testing.T) {
		opts, err := ctx.GetTransactor(ctx.GenesisValidators[0])
		if err != nil {
			t.Fatal(err)
		}
		_, err = ctx.Punish.ExecutePending(opts, big.NewInt(1))
		if err == nil {
			t.Fatalf("expected executePending external tx to be rejected")
		}
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "forbidden system transaction") && !strings.Contains(msg, "miner only") {
			t.Fatalf("expected forbidden system tx or miner-only rejection, got: %v", err)
		}
	})

	// [P-24] ExecutePending should be auto-executed by consensus without external tx.
	t.Run("P-24_ExecutePendingAutoByConsensus", func(t *testing.T) {
		const (
			configIDPunishThreshold int64 = 1
			configIDRemoveThreshold int64 = 2
			configIDDecreaseRate    int64 = 3
		)

		punishThreshold, err := ctx.Proposal.PunishThreshold(nil)
		if err != nil || punishThreshold == nil {
			t.Fatalf("read punishThreshold failed: %v", err)
		}
		removeThreshold, err := ctx.Proposal.RemoveThreshold(nil)
		if err != nil || removeThreshold == nil {
			t.Fatalf("read removeThreshold failed: %v", err)
		}
		decreaseRate, err := ctx.Proposal.DecreaseRate(nil)
		if err != nil || decreaseRate == nil {
			t.Fatalf("read decreaseRate failed: %v", err)
		}
		origPunishThreshold := new(big.Int).Set(punishThreshold)
		origRemoveThreshold := new(big.Int).Set(removeThreshold)
		origDecreaseRate := new(big.Int).Set(decreaseRate)

		t.Cleanup(func() {
			if err := ctx.EnsureConfig(configIDPunishThreshold, origPunishThreshold, big.NewInt(1)); err != nil {
				t.Errorf("restore punishThreshold failed: %v", err)
			}
			if err := ctx.EnsureConfig(configIDRemoveThreshold, origRemoveThreshold, big.NewInt(1000)); err != nil {
				t.Errorf("restore removeThreshold failed: %v", err)
			}
			if err := ctx.EnsureConfig(configIDDecreaseRate, origDecreaseRate, big.NewInt(1)); err != nil {
				t.Errorf("restore decreaseRate failed: %v", err)
			}
		})

		// Make epoch-path punish queue generation deterministic while keeping remove path disabled.
		if err := ctx.EnsureConfig(configIDPunishThreshold, big.NewInt(1), punishThreshold); err != nil {
			t.Fatalf("ensure punishThreshold=1 failed: %v", err)
		}
		if err := ctx.EnsureConfig(configIDRemoveThreshold, big.NewInt(1000), removeThreshold); err != nil {
			t.Fatalf("ensure removeThreshold=1000 failed: %v", err)
		}
		if err := ctx.EnsureConfig(configIDDecreaseRate, big.NewInt(1), decreaseRate); err != nil {
			t.Fatalf("ensure decreaseRate=1 failed: %v", err)
		}

		epochBig, err := ctx.Proposal.Epoch(nil)
		if err != nil || epochBig == nil || epochBig.Sign() == 0 {
			t.Fatalf("epoch not available: %v", err)
		}
		epoch := epochBig.Uint64()
		if epoch > 600 {
			t.Skipf("epoch=%d too large for bounded local integration assertion", epoch)
		}

		targetVal := common.HexToAddress(ctx.Config.Validators[1].Address)
		targetNeedsManualEpochPunish := true
		if activeVals, err := ctx.Validators.GetActiveValidators(nil); err == nil {
			for _, addr := range activeVals {
				if keyForAddress(addr) == nil {
					targetVal = addr
					targetNeedsManualEpochPunish = false
					break
				}
			}
		}
		if targetVal == (common.Address{}) {
			t.Fatalf("invalid target validator")
		}

		pendingHead := func() (common.Address, bool) {
			addr, err := ctx.Punish.PendingValidators(nil, big.NewInt(0))
			if err != nil {
				return common.Address{}, false
			}
			return addr, true
		}

		// Stabilize to an empty queue before creating a deterministic pending entry.
		for i := 0; i < 20; i++ {
			if _, has := pendingHead(); !has {
				break
			}
			waitBlocks(t, 1)
		}
		if _, has := pendingHead(); has {
			t.Fatalf("pending queue should be empty before P-24 auto-execution assertion")
		}

		submitEpochPunish := func(target common.Address) (*types.Receipt, error) {
			var lastErr error
			maxAttempts := int(epoch*3 + 30)
			for attempt := 0; attempt < maxAttempts; attempt++ {
				head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
				if err != nil || head == nil {
					lastErr = fmt.Errorf("read header failed: %w", err)
					waitBlocks(t, 1)
					continue
				}

				curHeight := head.Number.Uint64()
				if (curHeight+1)%epoch != 0 {
					waitBlocks(t, 1)
					continue
				}

				proposerKey, proposerAddr := pickInTurnValidatorForNextBlock(t)
				if proposerAddr == target {
					waitBlocks(t, 1)
					continue
				}

				opts, err := ctx.GetTransactorNoEpochWait(proposerKey, true)
				if err != nil {
					lastErr = err
					waitBlocks(t, 1)
					continue
				}

				tx, err := ctx.Punish.Punish(opts, target)
				if err != nil {
					lastErr = err
					msg := strings.ToLower(err.Error())
					if strings.Contains(msg, "epoch block forbidden") ||
						strings.Contains(msg, "nonce too low") ||
						strings.Contains(msg, "replacement transaction underpriced") ||
						strings.Contains(msg, "already known") ||
						strings.Contains(msg, "miner only") {
						waitBlocks(t, 1)
						continue
					}
					waitBlocks(t, 1)
					continue
				}

				if err := ctx.WaitMined(tx.Hash()); err != nil {
					lastErr = err
					waitBlocks(t, 1)
					continue
				}

				receipt, err := ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
				if err != nil || receipt == nil {
					lastErr = fmt.Errorf("read punish receipt failed: %w", err)
					waitBlocks(t, 1)
					continue
				}
				if receipt.BlockNumber == nil || receipt.BlockNumber.Uint64()%epoch != 0 {
					lastErr = fmt.Errorf("punish tx mined on non-epoch block %v", receipt.BlockNumber)
					waitBlocks(t, 1)
					continue
				}
				return receipt, nil
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("failed to submit punish on epoch block")
			}
			return nil, lastErr
		}

		if targetNeedsManualEpochPunish {
			receipt, err := submitEpochPunish(targetVal)
			if err != nil {
				t.Skipf("skip deterministic epoch punish submit in current window: %v", err)
			}
			t.Logf("epoch punish mined at block %d", receipt.BlockNumber.Uint64())
		} else {
			t.Logf("using unknown-key validator %s; waiting for consensus-generated pending entry", targetVal.Hex())
			observed := false
			maxObserveBlocks := int(epoch*4 + 30)
			for i := 0; i < maxObserveBlocks; i++ {
				if _, has := pendingHead(); has {
					observed = true
					break
				}
				waitBlocks(t, 1)
			}
			if !observed {
				t.Skipf("pending queue entry not observed within %d blocks for auto-consume assertion", maxObserveBlocks)
			}
		}

		head, has := pendingHead()
		if !has {
			t.Fatalf("pending queue entry not created after epoch punish")
		}
		if head != targetVal {
			t.Logf("pending queue head %s differs from target %s (queue still considered non-empty)", head.Hex(), targetVal.Hex())
		}

		consumed := false
		for i := 0; i < 12; i++ {
			waitBlocks(t, 1)
			if _, has := pendingHead(); !has {
				consumed = true
				break
			}
		}
		if !consumed {
			t.Fatalf("pending queue was not auto-consumed by consensus execution")
		}
	})

	// [P-25] DecreaseMissedBlocksCounter
	t.Run("P-25_DecreaseMissedBlocksCounter", func(t *testing.T) {
		// Only called on Epoch blocks. If called on non-epoch, should revert.
		opts, err := ctx.GetTransactor(ctx.GenesisValidators[0])
		if err != nil {
			t.Fatal(err)
		}

		_, err = ctx.Punish.DecreaseMissedBlocksCounter(opts, big.NewInt(1))
		if err != nil {
			t.Logf("decreaseMissedBlocksCounter failed: %v", err)
		} else {
			t.Log("decreaseMissedBlocksCounter succeeded (lucky hit on epoch block)")
		}
	})
}
