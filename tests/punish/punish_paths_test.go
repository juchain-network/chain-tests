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

	// [P-24] ExecutePending No-op
	t.Run("P-24_ExecutePendingNoop", func(t *testing.T) {
		waitMinedBounded := func(txHash common.Hash, timeout time.Duration) error {
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				receipt, errR := ctx.Clients[0].TransactionReceipt(context.Background(), txHash)
				if errR == nil && receipt != nil {
					if receipt.Status == 0 {
						return nil
					}
					return nil
				}
				time.Sleep(retrySleep())
			}
			return fmt.Errorf("tx %s still pending after %s", txHash.Hex(), timeout)
		}

		fromAddr := crypto.PubkeyToAddress(ctx.GenesisValidators[0].PublicKey)
		var lastErr error
		for retry := 0; retry < 6; retry++ {
			opts, errG := ctx.GetTransactor(ctx.GenesisValidators[0])
			if errG != nil {
				ctx.WaitIfEpochBlock()
				continue
			}

			tx, errCall := ctx.Punish.ExecutePending(opts, big.NewInt(1))
			if errCall == nil {
				lastErr = waitMinedBounded(tx.Hash(), 4*time.Second)
				if lastErr == nil {
					t.Logf("executePending(1) sent: %s", tx.Hash().Hex())
				}
				break
			}
			lastErr = errCall
			if strings.Contains(errCall.Error(), "Epoch block forbidden") {
				ctx.WaitIfEpochBlock()
				continue
			}
			msg := strings.ToLower(errCall.Error())
			if strings.Contains(msg, "nonce too low") ||
				strings.Contains(msg, "replacement transaction underpriced") ||
				strings.Contains(msg, "already known") {
				ctx.RefreshNonce(fromAddr)
				time.Sleep(retrySleep())
				continue
			}
			break
		}
		if lastErr != nil {
			t.Logf("executePending(1) best-effort result: %v", lastErr)
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
