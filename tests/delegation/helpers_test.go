package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func keyForAddress(addr common.Address) *ecdsa.PrivateKey {
	if ctx == nil {
		return nil
	}
	for _, k := range ctx.GenesisValidators {
		if crypto.PubkeyToAddress(k.PublicKey) == addr {
			return k
		}
	}
	return nil
}

func minerKeyOrSkip(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	if ctx == nil || len(ctx.Clients) == 0 {
		t.Fatalf("Context not initialized")
	}

	var lastCoinbase common.Address
	for attempt := 0; attempt < 30; attempt++ {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil {
			t.Fatalf("failed to read header: %v", err)
		}
		coinbase := header.Coinbase
		lastCoinbase = coinbase
		if coinbase != (common.Address{}) {
			if key := keyForAddress(coinbase); key != nil {
				return key, coinbase
			}
		}
		waitBlocks(t, 1)
	}

	known := make([]string, 0, len(ctx.GenesisValidators))
	for _, k := range ctx.GenesisValidators {
		known = append(known, crypto.PubkeyToAddress(k.PublicKey).Hex())
	}
	t.Fatalf("no validator key matches current coinbase after 30 blocks; last=%s known=%s", lastCoinbase.Hex(), strings.Join(known, ","))
	return nil, common.Address{} // unreachable
}

// waitForNextEpochBlock mines or waits until an epoch block has passed and state has settled.
// It also handles triggering updateActiveValidatorSet as the miner if we are at an epoch block.
func waitForNextEpochBlock(t *testing.T) uint64 {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	epochBig, err := ctx.Proposal.Epoch(nil)
	if err != nil || epochBig.Sign() == 0 {
		t.Fatalf("epoch not available")
	}
	epoch := epochBig.Uint64()
	header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to read header: %v", err)
	}
	cur := header.Number.Uint64()

	// Target is the next multiple of epoch
	nextEpochBlock := ((cur / epoch) + 1) * epoch
	blocksToWait := nextEpochBlock - cur

	t.Logf("Current block %d, next epoch block %d, waiting %d blocks...", cur, nextEpochBlock, blocksToWait)

	// Wait until we are at nextEpochBlock - 1
	if blocksToWait > 1 {
		waitBlocks(t, int(blocksToWait-1))
	}

	// Now wait block by block until we hit exactly nextEpochBlock
	for {
		header, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if header.Number.Uint64() >= nextEpochBlock {
			break
		}
		time.Sleep(blockPollInterval())
	}

	// Trigger the validator set update.
	highest, _ := ctx.Validators.GetHighestValidators(nil)
	top, _ := ctx.Staking.GetTopValidators(nil, highest)

	success := false
	// Try aggressively to trigger the update
	for retry := 0; retry < 10; retry++ {
		curHeader, _ := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		curHeight := curHeader.Number.Uint64()

		// If we passed the window, we might have missed it or it happened
		if curHeight > nextEpochBlock+5 {
			break
		}

		for _, vk := range ctx.GenesisValidators {
			addr := crypto.PubkeyToAddress(vk.PublicKey)

			// Refresh nonce to be safe; allow epoch-block tx for validator-set update
			ctx.RefreshNonce(addr)
			opts, _ := ctx.GetTransactorNoEpochWait(vk, true)

			// Try to update for the current block OR the next one
			// We try both because we might be at the end of the block
			targets := []uint64{curHeight, curHeight + 1}

			for _, target := range targets {
				if target%epoch != 0 {
					continue
				}
				tx, err := ctx.Validators.UpdateActiveValidatorSet(opts, top, big.NewInt(int64(target)))
				if err == nil {
					t.Logf("Triggered UpdateActiveValidatorSet successfully at block %d from %s", target, addr.Hex())
					ctx.WaitMined(tx.Hash())
					success = true
					goto Done
				}
			}
		}
		time.Sleep(blockPollInterval())
	}
Done:

	if !success {
		t.Log("UpdateActiveValidatorSet was not triggered (might be already done or missed)")
	}

	newHeight, _ := ctx.Clients[0].BlockNumber(context.Background())
	fmt.Printf("Epoch wait complete. New height: %d\n", newHeight)
	ctx.SyncNonces()
	return newHeight
}

func waitForValidatorActive(t *testing.T, addr common.Address, maxEpochs int) bool {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}
	for i := 0; i < maxEpochs; i++ {
		active, _ := ctx.Validators.IsValidatorActive(nil, addr)
		if active {
			return true
		}
		waitForNextEpochBlock(t)
		waitBlocks(t, 1)
	}
	return false
}

func ensureMinActiveValidators(t *testing.T, min int, maxEpochs int) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if min < 1 {
		min = 1
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}
	for attempt := 0; attempt < maxEpochs; attempt++ {
		set, err := ctx.Validators.GetActiveValidators(nil)
		if err == nil && len(set) >= min {
			return
		}
		waitForNextEpochBlock(t)
		waitBlocks(t, 1)
	}
	set, _ := ctx.Validators.GetActiveValidators(nil)
	t.Fatalf("active validators < %d (got %d)", min, len(set))
}

func getActiveProposerOrSkip(t *testing.T, maxEpochs int) *ecdsa.PrivateKey {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}
	for attempt := 0; attempt < maxEpochs; attempt++ {
		for _, k := range ctx.GenesisValidators {
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
	t.Fatalf("no active proposer available")
	return nil
}

func pickInTurnValidatorForNextBlock(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	validators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil || len(validators) == 0 {
		t.Fatalf("no active validators available")
	}
	header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || header == nil {
		t.Fatalf("failed to read header: %v", err)
	}
	coinbase := header.Coinbase
	idx := 0
	for i, v := range validators {
		if v == coinbase {
			idx = i + 1
			break
		}
	}
	if idx >= len(validators) {
		idx = 0
	}
	addr := validators[idx]
	key := keyForAddress(addr)
	if key == nil {
		t.Fatalf("no key for in-turn validator %s", addr.Hex())
	}
	return key, addr
}

func waitNextBlock() {
	waitBlocks(nil, 1)
}
