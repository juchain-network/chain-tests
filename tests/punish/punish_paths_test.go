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
		runExecutePendingAutoByConsensus(t, "P-24")
	})

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
