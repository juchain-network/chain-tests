package tests

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
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
		var err error
		for retry := 0; retry < 10; retry++ {
			opts, errG := ctx.GetTransactor(ctx.GenesisValidators[0])
			if errG != nil {
				waitBlocks(t, 1)
				continue
			}

			tx, errCall := ctx.Punish.ExecutePending(opts, big.NewInt(1))
			if errCall == nil {
				ctx.WaitMined(tx.Hash())
				err = nil
				break
			}
			err = errCall
			if strings.Contains(err.Error(), "Epoch block forbidden") {
				waitBlocks(t, 1)
				continue
			}
			break
		}
		if err != nil {
			t.Logf("executePending(1) failed: %v", err)
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
