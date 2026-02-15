package tests

import (
	"context"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestY_UpdateActiveValidatorSet(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	epoch, _ := ctx.Proposal.Epoch(nil)
	if epoch.Sign() == 0 {
		t.Fatalf("epoch not set")
	}

	// Non-epoch call should fail
	t.Run("V-07_UpdateSetNonEpoch", func(t *testing.T) {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || header == nil {
			t.Fatalf("failed to read header: %v", err)
		}
		current, _ := ctx.Validators.GetActiveValidators(nil)
		abi, err := contracts.ValidatorsMetaData.GetAbi()
		if err != nil {
			t.Fatalf("failed to load validators ABI: %v", err)
		}
		data, err := abi.Pack("updateActiveValidatorSet", current, epoch)
		if err != nil {
			t.Fatalf("failed to pack updateActiveValidatorSet: %v", err)
		}
		msg := ethereum.CallMsg{
			From: header.Coinbase,
			To:   &testctx.ValidatorsAddr,
			Data: data,
		}
		_, err = ctx.Clients[0].CallContract(context.Background(), msg, nil)
		if err == nil {
			t.Fatalf("expected Block epoch only revert for non-epoch call, got success")
		}
		if strings.Contains(err.Error(), "Miner only") || strings.Contains(err.Error(), "forbidden system transaction") {
			t.Fatalf("caller is not current miner or system blocked: %v", err)
		}
		if !strings.Contains(err.Error(), "Block epoch only") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Normal epoch update path
	t.Run("V-08_UpdateSetEpoch", func(t *testing.T) {
		waitForNextEpochBlock(t)

		highest, err := ctx.Validators.GetHighestValidators(nil)
		utils.AssertNoError(t, err, "getHighestValidators failed")
		expected, err := ctx.Staking.GetTopValidators(nil, highest)
		utils.AssertNoError(t, err, "getTopValidators failed")
		if len(expected) == 0 {
			t.Fatalf("expected validator set empty")
		}
		// At epoch boundary, active set should match top validators.
		waitBlocks(t, 1)
		newSet, _ := ctx.Validators.GetActiveValidators(nil)
		if len(newSet) != len(expected) {
			t.Fatalf("validator set length mismatch: expected %d, got %d", len(expected), len(newSet))
		}
	})
}
