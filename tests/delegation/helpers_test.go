package tests

import (
	"context"
	"crypto/ecdsa"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func keyForAddress(addr common.Address) *ecdsa.PrivateKey {
	if ctx == nil {
		return nil
	}
	return ctx.ValidatorKeyByAddress(addr)
}

func minerKeyOrSkip(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	if ctx == nil || len(ctx.Clients) == 0 {
		t.Fatalf("Context not initialized")
	}

	var lastCoinbase common.Address
	var lastValidator common.Address
	for attempt := 0; attempt < 30; attempt++ {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil {
			t.Fatalf("failed to read header: %v", err)
		}
		coinbase := header.Coinbase
		lastCoinbase = coinbase
		if coinbase != (common.Address{}) {
			validator, err := ctx.ValidatorAddressBySigner(coinbase)
			if err == nil {
				lastValidator = validator
				if key := keyForAddress(validator); key != nil {
					return key, validator
				}
			}
		}
		waitBlocks(t, 1)
	}

	known := make([]string, 0, len(ctx.GenesisValidators))
	for _, k := range ctx.GenesisValidators {
		known = append(known, crypto.PubkeyToAddress(k.PublicKey).Hex())
	}
	t.Fatalf("no validator key matches current coinbase signer after 30 blocks; lastSigner=%s lastValidator=%s known=%s", lastCoinbase.Hex(), lastValidator.Hex(), strings.Join(known, ","))
	return nil, common.Address{} // unreachable
}

// waitForNextEpochBlock mines or waits until an epoch block has passed and state has settled.
// It also handles triggering updateActiveValidatorSet as the miner if we are at an epoch block.
func waitForNextEpochBlock(t *testing.T) uint64 {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	height, err := ctx.WaitForNextEpochTransition()
	if err != nil {
		t.Fatalf("wait for next epoch transition failed: %v", err)
	}
	return height
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
	currentValidator, err := ctx.ValidatorAddressBySigner(header.Coinbase)
	if err != nil {
		t.Fatalf("failed to map coinbase signer %s to validator: %v", header.Coinbase.Hex(), err)
	}
	idx := 0
	for i, v := range validators {
		if v == currentValidator {
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
